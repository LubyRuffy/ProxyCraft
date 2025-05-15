package harlogger

import "time"

// HAR is the root object of a HAR file.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#har
type HAR struct {
	Log Log `json:"log"`
}

// Log is the main log object.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#log
type Log struct {
	Version string   `json:"version"`
	Creator Creator  `json:"creator"`
	Browser *Browser `json:"browser,omitempty"` // Optional
	Pages   []Page   `json:"pages,omitempty"`   // Optional
	Entries []Entry  `json:"entries"`
	Comment string   `json:"comment,omitempty"` // Optional
}

// Creator is information about the HAR creator application.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#creator
type Creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Comment string `json:"comment,omitempty"` // Optional
}

// Browser is information about the browser that created the HAR.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#browser
type Browser struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Comment string `json:"comment,omitempty"` // Optional
}

// Page contains information about a single page.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#pages
type Page struct {
	StartedDateTime time.Time   `json:"startedDateTime"`
	ID              string      `json:"id"`
	Title           string      `json:"title"`
	PageTimings     PageTimings `json:"pageTimings"`
	Comment         string      `json:"comment,omitempty"` // Optional
}

// PageTimings describes page loading timings.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#pageTimings
type PageTimings struct {
	OnContentLoad float64 `json:"onContentLoad,omitempty"` // Optional, in ms
	OnLoad        float64 `json:"onLoad,omitempty"`        // Optional, in ms
	Comment       string  `json:"comment,omitempty"`       // Optional
}

// Entry represents an HTTP request/response pair.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#entries
type Entry struct {
	Pageref         string    `json:"pageref,omitempty"` // Optional
	StartedDateTime time.Time `json:"startedDateTime"`
	Time            float64   `json:"time"` // Total time in ms
	Request         Request   `json:"request"`
	Response        Response  `json:"response"`
	Cache           Cache     `json:"cache"`
	Timings         Timings   `json:"timings"`
	ServerIPAddress string    `json:"serverIPAddress,omitempty"` // Optional
	Connection      string    `json:"connection,omitempty"`      // Optional
	Comment         string    `json:"comment,omitempty"`         // Optional
}

// Request contains detailed information about the HTTP request.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#request
type Request struct {
	Method      string          `json:"method"`
	URL         string          `json:"url"`
	HTTPVersion string          `json:"httpVersion"`
	Cookies     []Cookie        `json:"cookies"`
	Headers     []NameValuePair `json:"headers"`
	QueryString []NameValuePair `json:"queryString"`
	PostData    *PostData       `json:"postData,omitempty"` // Optional
	HeadersSize int64           `json:"headersSize"`        // -1 if unknown
	BodySize    int64           `json:"bodySize"`           // -1 if unknown
	Comment     string          `json:"comment,omitempty"`  // Optional
}

// Response contains detailed information about the HTTP response.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#response
type Response struct {
	Status      int             `json:"status"`
	StatusText  string          `json:"statusText"`
	HTTPVersion string          `json:"httpVersion"`
	Cookies     []Cookie        `json:"cookies"`
	Headers     []NameValuePair `json:"headers"`
	Content     Content         `json:"content"`
	RedirectURL string          `json:"redirectURL"`
	HeadersSize int64           `json:"headersSize"`       // -1 if unknown
	BodySize    int64           `json:"bodySize"`          // -1 if unknown
	Comment     string          `json:"comment,omitempty"` // Optional
}

// Cookie contains information about a single cookie.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#cookies
type Cookie struct {
	Name     string     `json:"name"`
	Value    string     `json:"value"`
	Path     string     `json:"path,omitempty"`     // Optional
	Domain   string     `json:"domain,omitempty"`   // Optional
	Expires  *time.Time `json:"expires,omitempty"`  // Optional
	HTTPOnly bool       `json:"httpOnly,omitempty"` // Optional
	Secure   bool       `json:"secure,omitempty"`   // Optional
	Comment  string     `json:"comment,omitempty"`  // Optional
}

// NameValuePair is a generic name/value pair structure used for headers, query strings etc.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#nameValuePair
type NameValuePair struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Comment string `json:"comment,omitempty"` // Optional
}

// PostData describes posted data.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#postData
type PostData struct {
	MimeType string      `json:"mimeType"`
	Params   []PostParam `json:"params,omitempty"`
	Text     string      `json:"text,omitempty"`
	Encoding string      `json:"encoding,omitempty"` // Added for base64 encoded content
	// Comment string `json:"comment,omitempty"` // Optional according to spec, not commonly used by browsers
}

// PostParam describes a posted parameter.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#params
type PostParam struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`       // Optional
	FileName    string `json:"fileName,omitempty"`    // Optional
	ContentType string `json:"contentType,omitempty"` // Optional
	Comment     string `json:"comment,omitempty"`     // Optional
}

// Content describes the response content.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#content
type Content struct {
	Size        int64  `json:"size"`
	Compression int64  `json:"compression,omitempty"` // Optional
	MimeType    string `json:"mimeType"`
	Text        string `json:"text,omitempty"`     // Optional, decoded if possible
	Encoding    string `json:"encoding,omitempty"` // Optional (e.g., "base64")
	Comment     string `json:"comment,omitempty"`  // Optional
}

// Cache contains information about the cache entry.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#cache
type Cache struct {
	BeforeRequest *CacheEntry `json:"beforeRequest,omitempty"` // Optional
	AfterRequest  *CacheEntry `json:"afterRequest,omitempty"`  // Optional
	Comment       string      `json:"comment,omitempty"`       // Optional
}

// CacheEntry describes a cache entry.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#cacheEntry
type CacheEntry struct {
	Expires    *time.Time `json:"expires,omitempty"` // Optional
	LastAccess time.Time  `json:"lastAccess"`
	ETag       string     `json:"eTag"`
	HitCount   int        `json:"hitCount"`
	Comment    string     `json:"comment,omitempty"` // Optional
}

// Timings describes various timings for the request-response cycle.
// Spec: http://www.softwareishard.com/blog/har-12-spec/#timings
type Timings struct {
	Blocked float64 `json:"blocked,omitempty"` // Optional, in ms, -1 if not applicable
	DNS     float64 `json:"dns,omitempty"`     // Optional, in ms, -1 if not applicable
	Connect float64 `json:"connect,omitempty"` // Optional, in ms, -1 if not applicable
	Send    float64 `json:"send"`              // in ms
	Wait    float64 `json:"wait"`              // in ms
	Receive float64 `json:"receive"`           // in ms
	SSL     float64 `json:"ssl,omitempty"`     // Optional, in ms, -1 if not applicable
	Comment string  `json:"comment,omitempty"` // Optional
}
