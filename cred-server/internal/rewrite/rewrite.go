package rewrite

import (
	"net/url"
	"regexp"
	"strings"
)

// MultiHostConfig holds configuration for multi-host URL rewriting
type MultiHostConfig struct {
	PrimaryHost   string            // Primary target host (uses /proxy/{session}/ prefix)
	TargetHosts   map[string]string // hostname -> base URL for all hosts including primary
	ProxyBasePath string            // e.g., /proxy/{sessionID}
}

// RedirectURL rewrites Location header values to go through the proxy
func RedirectURL(location, targetURL, proxyBasePath string) string {
	// Parse the target URL to get scheme and host
	targetParsed, err := url.Parse(targetURL)
	if err != nil {
		return location
	}

	// Parse the location
	locParsed, err := url.Parse(location)
	if err != nil {
		return location
	}

	// If it's a relative path, prepend proxy base
	if locParsed.Host == "" {
		path := locParsed.Path
		// Handle relative paths like ./web/ or ../foo
		if strings.HasPrefix(path, "./") {
			path = path[1:] // ./web/ -> /web/
		} else if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		// Don't double-prefix if already proxied
		if strings.HasPrefix(path, proxyBasePath) {
			return path
		}
		return proxyBasePath + path
	}

	// If it's pointing to the target host, rewrite it
	if locParsed.Host == targetParsed.Host {
		// Don't double-prefix if already proxied
		if strings.HasPrefix(locParsed.Path, proxyBasePath) {
			return locParsed.Path
		}
		return proxyBasePath + locParsed.Path
	}

	// External redirect, leave as-is
	return location
}

// ResponseBody rewrites absolute URLs in HTML/JS to go through the proxy
func ResponseBody(body []byte, targetURL, proxyBasePath string) []byte {
	targetParsed, err := url.Parse(targetURL)
	if err != nil {
		return body
	}

	content := string(body)

	// Rewrite absolute URLs with the target host
	// e.g., https://argo.fklab.local/api/ -> /proxy/{sessionID}/api/
	fullTarget := targetParsed.Scheme + "://" + targetParsed.Host
	content = strings.ReplaceAll(content, fullTarget, proxyBasePath)

	// Also try with quotes (common in JSON/JS)
	content = strings.ReplaceAll(content, `"`+fullTarget, `"`+proxyBasePath)
	content = strings.ReplaceAll(content, `'`+fullTarget, `'`+proxyBasePath)

	// Rewrite protocol-relative URLs
	// e.g., //argo.fklab.local/api/ -> /proxy/{sessionID}/api/
	protoRelative := "//" + targetParsed.Host
	content = strings.ReplaceAll(content, protoRelative, proxyBasePath)
	content = strings.ReplaceAll(content, `"`+protoRelative, `"`+proxyBasePath)
	content = strings.ReplaceAll(content, `'`+protoRelative, `'`+proxyBasePath)

	// Rewrite <base href="..."> tags for SPAs
	content = BaseHref(content, proxyBasePath)

	// Rewrite common SPA absolute paths (quoted strings in JS/JSON)
	// These are paths that SPAs use for client-side routing
	content = RewriteSPAPaths(content, proxyBasePath)

	// Rewrite common API base path patterns
	// e.g., window.API_BASE_PATH = '/api' -> window.API_BASE_PATH = '/proxy/{sessionID}/api'
	content = APIBasePaths(content, proxyBasePath)

	return []byte(content)
}

// BaseHref finds and rewrites <base href="..."> tags
func BaseHref(content, proxyBasePath string) string {
	// Match <base href="/something/" /> or <base href="/something/">
	// We want to prepend the proxy base path
	idx := strings.Index(content, "<base href=\"")
	if idx == -1 {
		return content
	}

	startQuote := idx + len("<base href=\"")
	endQuote := strings.Index(content[startQuote:], "\"")
	if endQuote == -1 {
		return content
	}

	originalHref := content[startQuote : startQuote+endQuote]

	// Only rewrite if it's an absolute path (starts with /) AND not already proxied
	if strings.HasPrefix(originalHref, "/") && !strings.HasPrefix(originalHref, proxyBasePath) {
		newHref := proxyBasePath + originalHref
		content = content[:startQuote] + newHref + content[startQuote+endQuote:]
	}

	return content
}

// APIBasePaths rewrites common JavaScript API base path patterns
// NOTE: We intentionally DO NOT rewrite fetch/axios paths because modern SPAs
// typically use dynamic baseURL (e.g., window.location.origin + pathname).
// When the app runs through the proxy, the dynamic baseURL already includes
// the proxy path, so rewriting individual API paths would cause doubling.
func APIBasePaths(content, proxyBasePath string) string {
	// Only rewrite static API base path configurations, NOT individual API calls.
	// Apps with dynamic baseURL (from window.location) don't need path rewriting.

	// Check if the app uses dynamic baseURL from window.location
	// If so, skip all API path rewriting to avoid doubling
	if strings.Contains(content, "window.location.origin") ||
		strings.Contains(content, "location.origin") ||
		strings.Contains(content, "window.location.pathname") {
		// App uses dynamic baseURL - don't rewrite API paths
		return content
	}

	// Only for apps with static baseURL configuration, rewrite the base path
	// Pattern: window.API_BASE_PATH = '/api' -> '/proxy/{sessionID}/api'
	content = regexp.MustCompile(`(API_BASE_PATH|BASE_PATH)\s*[=:]\s*['"](/[^'"]*?)['"]`).
		ReplaceAllStringFunc(content, func(match string) string {
			re := regexp.MustCompile(`['"](/[^'"]*?)['"]`)
			return re.ReplaceAllStringFunc(match, func(path string) string {
				cleanPath := strings.Trim(path, `'"`)
				if strings.HasPrefix(cleanPath, "/") && !strings.HasPrefix(cleanPath, proxyBasePath) {
					return `"` + proxyBasePath + cleanPath + `"`
				}
				return path
			})
		})

	return content
}

// ResponseBodyMultiHost rewrites URLs in HTML/JS for multiple target hosts
func ResponseBodyMultiHost(body []byte, config *MultiHostConfig) []byte {
	content := string(body)

	// Rewrite URLs for each target host
	for hostname, baseURL := range config.TargetHosts {
		targetParsed, err := url.Parse(baseURL)
		if err != nil {
			continue
		}

		// Determine proxy path for this host
		var proxyPath string
		if hostname == config.PrimaryHost {
			proxyPath = config.ProxyBasePath
		} else {
			proxyPath = config.ProxyBasePath + "/_h/" + hostname
		}

		// Rewrite absolute URLs with this host
		fullTarget := targetParsed.Scheme + "://" + targetParsed.Host
		content = strings.ReplaceAll(content, fullTarget, proxyPath)
		content = strings.ReplaceAll(content, `"`+fullTarget, `"`+proxyPath)
		content = strings.ReplaceAll(content, `'`+fullTarget, `'`+proxyPath)

		// Rewrite protocol-relative URLs
		protoRelative := "//" + targetParsed.Host
		content = strings.ReplaceAll(content, protoRelative, proxyPath)
		content = strings.ReplaceAll(content, `"`+protoRelative, `"`+proxyPath)
		content = strings.ReplaceAll(content, `'`+protoRelative, `'`+proxyPath)
	}

	// Rewrite <base href="..."> tags for SPAs
	content = BaseHref(content, config.ProxyBasePath)

	// Rewrite common API base path patterns
	content = APIBasePaths(content, config.ProxyBasePath)

	return []byte(content)
}

// RedirectURLMultiHost rewrites Location header for multiple target hosts
func RedirectURLMultiHost(location string, config *MultiHostConfig) string {
	locParsed, err := url.Parse(location)
	if err != nil {
		return location
	}

	// If it's a relative path, prepend proxy base
	if locParsed.Host == "" {
		path := locParsed.Path
		if strings.HasPrefix(path, "./") {
			path = path[1:]
		} else if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if strings.HasPrefix(path, config.ProxyBasePath) {
			return path
		}
		return config.ProxyBasePath + path
	}

	// Check if this host is in our target hosts
	if baseURL, ok := config.TargetHosts[locParsed.Host]; ok {
		var proxyPath string
		if locParsed.Host == config.PrimaryHost {
			proxyPath = config.ProxyBasePath
		} else {
			proxyPath = config.ProxyBasePath + "/_h/" + locParsed.Host
		}

		// Don't double-prefix
		if strings.HasPrefix(locParsed.Path, proxyPath) {
			return locParsed.Path
		}

		_ = baseURL // Used for validation only
		return proxyPath + locParsed.Path
	}

	// External redirect, leave as-is
	return location
}

// RewriteSPAPaths rewrites common SPA absolute paths in JavaScript/JSON
// This handles cases where SPAs use absolute paths for client-side routing
// e.g., "/dashboard/sign-in" -> "/proxy/{sessionID}/dashboard/sign-in"
func RewriteSPAPaths(content, proxyBasePath string) string {
	// Common SPA route prefixes that should be rewritten
	// These are absolute paths that apps use for navigation
	spaPrefixes := []string{
		"/dashboard",
		"/account",
		"/settings",
		"/admin",
		"/app",
		"/auth",
		"/login",
		"/signin",
		"/sign-in",
		"/signup",
		"/sign-up",
		"/profile",
		"/org",
		"/project",
		"/projects",
		"/api/auth", // Next.js auth routes
	}

	for _, prefix := range spaPrefixes {
		// Skip if already contains proxy path to avoid double-rewriting
		if strings.Contains(content, proxyBasePath+prefix) {
			continue
		}

		// Rewrite quoted paths (in JS strings and JSON)
		// "href":"/dashboard" -> "href":"/proxy/{sessionID}/dashboard"
		content = strings.ReplaceAll(content, `"`+prefix+`"`, `"`+proxyBasePath+prefix+`"`)
		content = strings.ReplaceAll(content, `'`+prefix+`'`, `'`+proxyBasePath+prefix+`'`)

		// Rewrite paths followed by more path segments
		// "/dashboard/sign-in" -> "/proxy/{sessionID}/dashboard/sign-in"
		content = strings.ReplaceAll(content, `"`+prefix+`/`, `"`+proxyBasePath+prefix+`/`)
		content = strings.ReplaceAll(content, `'`+prefix+`/`, `'`+proxyBasePath+prefix+`/`)

		// Handle paths in href attributes
		// href="/dashboard" -> href="/proxy/{sessionID}/dashboard"
		content = strings.ReplaceAll(content, `href="`+prefix, `href="`+proxyBasePath+prefix)
		content = strings.ReplaceAll(content, `href='`+prefix, `href='`+proxyBasePath+prefix)

		// Handle paths in src attributes (for scripts/images)
		content = strings.ReplaceAll(content, `src="`+prefix, `src="`+proxyBasePath+prefix)
		content = strings.ReplaceAll(content, `src='`+prefix, `src='`+proxyBasePath+prefix)

		// Handle Next.js/React router patterns
		// to="/dashboard" -> to="/proxy/{sessionID}/dashboard"
		content = strings.ReplaceAll(content, `to="`+prefix, `to="`+proxyBasePath+prefix)
		content = strings.ReplaceAll(content, `to='`+prefix, `to='`+proxyBasePath+prefix)
	}

	return content
}
