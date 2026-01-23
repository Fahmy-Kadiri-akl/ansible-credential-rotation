package analysis

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/rs/zerolog/log"
)

// DiscoverAPISpec probes common API spec paths and parses the spec
func DiscoverAPISpec(targetURL string) (string, []DetectedEndpoint) {
	client := &http.Client{
		Timeout: constants.HTTP__TIMEOUT__DISCOVERY__SECONDS__DEFAULT,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for _, path := range constants.SPEC__DISCOVERY__PATHS {
		specURL := targetURL + path
		resp, err := client.Get(specURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		// Check if it looks like a spec (JSON or contains swagger/openapi)
		contentType := resp.Header.Get("Content-Type")
		bodyStr := string(body)

		// If it's HTML, look for swagger-ui and try to find the actual spec URL
		if strings.Contains(contentType, "text/html") || strings.HasPrefix(bodyStr, "<!") {
			// Try to extract spec URL from swagger-ui HTML
			if actualSpecURL := extractSpecURLFromHTML(bodyStr, targetURL); actualSpecURL != "" {
				// Fetch the actual spec
				specResp, err := client.Get(actualSpecURL)
				if err == nil {
					defer specResp.Body.Close()
					if specResp.StatusCode == 200 {
						specBody, _ := io.ReadAll(specResp.Body)
						if endpoints := ParseOpenAPISpec(specBody); len(endpoints) > 0 {
							log.Info().Str("url", actualSpecURL).Msg("Found API spec via swagger-ui")
							return actualSpecURL, endpoints
						}
					}
				}
			}
			continue
		}

		// Try to parse as OpenAPI/Swagger spec
		if endpoints := ParseOpenAPISpec(body); len(endpoints) > 0 {
			log.Info().Str("url", specURL).Msg("Found API spec")
			return specURL, endpoints
		}
	}

	return "", nil
}

// extractSpecURLFromHTML tries to find the spec URL from swagger-ui HTML
func extractSpecURLFromHTML(html, baseURL string) string {
	// Look for swagger.json or openapi.json in the HTML
	specFiles := []string{"swagger.json", "openapi.json"}

	for _, specFile := range specFiles {
		if strings.Contains(html, specFile) {
			// Find the full path
			idx := strings.Index(html, specFile)
			// Look backwards for the start of the URL
			start := idx
			for start > 0 && html[start-1] != '"' && html[start-1] != '\'' && html[start-1] != '=' {
				start--
			}
			path := html[start : idx+len(specFile)]
			if strings.HasPrefix(path, "/") {
				return baseURL + path
			} else if strings.HasPrefix(path, "http") {
				return path
			}
		}
	}

	// Try common spec paths relative to swagger UI
	if strings.Contains(html, "swagger") {
		return baseURL + "/swagger/v1/swagger.json"
	}

	return ""
}

// ParseOpenAPISpec parses OpenAPI/Swagger spec and extracts endpoints
func ParseOpenAPISpec(data []byte) []DetectedEndpoint {
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil
	}

	endpoints := []DetectedEndpoint{}

	// Check for OpenAPI 3.x paths
	if paths, ok := spec["paths"].(map[string]interface{}); ok {
		for path, pathItem := range paths {
			if pathMethods, ok := pathItem.(map[string]interface{}); ok {
				for method, methodDef := range pathMethods {
					// Skip non-HTTP method keys like "parameters"
					method = strings.ToUpper(method)
					if !constants.IsHTTPMethod(method) {
						continue
					}

					ep := DetectedEndpoint{
						Path:   path,
						Method: method,
						Source: constants.SOURCE__LABEL__SPEC,
					}

					if def, ok := methodDef.(map[string]interface{}); ok {
						if summary, ok := def["summary"].(string); ok {
							ep.Description = summary
						} else if desc, ok := def["description"].(string); ok {
							ep.Description = desc
						}
						if opId, ok := def["operationId"].(string); ok {
							ep.Operation = opId
						}
						// Try to extract tags for context
						if tags, ok := def["tags"].([]interface{}); ok && len(tags) > 0 {
							if tag, ok := tags[0].(string); ok {
								if ep.Description == "" {
									ep.Description = tag
								}
							}
						}
					}

					endpoints = append(endpoints, ep)
				}
			}
		}
	}

	return endpoints
}
