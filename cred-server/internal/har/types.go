package har

// File represents a HAR (HTTP Archive) file structure
type File struct {
	Log Log `json:"log"`
}

// Log contains the HAR log entries
type Log struct {
	Entries []Entry `json:"entries"`
}

// Entry represents a single HAR entry (request/response pair)
type Entry struct {
	StartedDateTime string   `json:"startedDateTime"`
	Request         Request  `json:"request"`
	Response        Response `json:"response"`
}

// Request represents an HTTP request in HAR format
type Request struct {
	Method      string        `json:"method"`
	URL         string        `json:"url"`
	Headers     []Header      `json:"headers"`
	PostData    *PostData     `json:"postData,omitempty"`
	QueryString []QueryString `json:"queryString"`
}

// Response represents an HTTP response in HAR format
type Response struct {
	Status     int      `json:"status"`
	StatusText string   `json:"statusText"`
	Headers    []Header `json:"headers"`
	Content    Content  `json:"content"`
}

// Header represents an HTTP header
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PostData represents POST request body data
type PostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// Content represents response body content
type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}

// QueryString represents a URL query parameter
type QueryString struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
