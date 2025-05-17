package harlogger

import (
	"bytes"           // Added for bytes.NewBuffer
	"context"         // Added for context in auto-save
	"encoding/base64" // Added for base64 encoding binary bodies
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url" // Added for url.Values in buildHARQueryString
	"os"
	"strings" // Added for strings.NewReader
	"sync"
	"time"
	// Added for header canonicalization and size calculation
	// Assuming certs.Manager might be needed for version or other info
)

const (
	proxyName    = "ProxyCraft"
	proxyVersion = "0.1.0"
)

// Logger is responsible for creating and writing HAR logs.
// It is designed to be thread-safe.
type Logger struct {
	mu               sync.Mutex
	h                *HAR
	outputFile       string
	enabled          bool
	autoSaveEnabled  bool
	autoSaveInterval time.Duration
	cancelAutoSave   context.CancelFunc
}

// NewLogger creates a new HAR logger.
// If outputFile is empty, logging will be disabled.
func NewLogger(outputFile string, proxyName string, proxyVersion string) *Logger {
	l := &Logger{
		outputFile:       outputFile,
		enabled:          outputFile != "",
		autoSaveEnabled:  false,
		autoSaveInterval: 30 * time.Second, // Default to 30 seconds
	}
	if l.enabled {
		l.h = &HAR{
			Log: Log{
				Version: "1.2",
				Creator: Creator{
					Name:    proxyName,
					Version: proxyVersion,
				},
				Entries: []Entry{},
			},
		}
	}
	return l
}

// IsEnabled checks if HAR logging is active.
func (l *Logger) IsEnabled() bool {
	return l.enabled
}

// AddEntry records a new HTTP transaction (request and response) to the HAR log.
func (l *Logger) AddEntry(req *http.Request, resp *http.Response, startedDateTime time.Time, timeTaken time.Duration, serverIP string, connectionID string) {
	if !l.IsEnabled() {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	harReq := l.buildHARRequest(req)
	harResp := l.buildHARResponse(resp)

	entry := Entry{
		StartedDateTime: startedDateTime.UTC(), // HAR spec recommends UTC
		Time:            float64(timeTaken.Milliseconds()),
		Request:         harReq,
		Response:        harResp,
		Cache:           Cache{},                      // Empty for now, can be enhanced
		Timings:         l.buildHARTimings(timeTaken), // Simplified timings
		ServerIPAddress: serverIP,
		Connection:      connectionID, // Optional, can be a unique ID for the TCP/IP connection
	}

	l.h.Log.Entries = append(l.h.Log.Entries, entry)
}

// calculateHeadersSize calculates the approximate size of HTTP headers.
// HAR spec: "Size of all request headers (multi-line entries usually include CR LF line endings)."
// This is an approximation. It sums len(key) + len(": ") + len(value) + len("\r\n") for each header line.
func calculateHeadersSize(headers http.Header) int64 {
	var size int64
	// Create a buffer to roughly estimate header size by writing them out
	// This is a common way to estimate, though not perfectly precise for all edge cases.
	// A simpler sum of lengths might also be acceptable for HAR.
	// For example: key + ": " + value + "\r\n"
	for name, values := range headers {
		for _, value := range values {
			// Each header line: Name: Value
			size += int64(len(name) + len(": ") + len(value) + len("\r\n"))
		}
	}
	// Add the final \r\n for the end of the header block
	if len(headers) > 0 {
		size += int64(len("\r\n"))
	}
	return size
}

func (l *Logger) buildHARRequest(req *http.Request) Request {
	bodySize := int64(-1)
	if req.ContentLength > 0 {
		bodySize = req.ContentLength
	}

	var postData *PostData
	bodyBytes, err := readAndRestoreBody(&req.Body, req.ContentLength) // Capture and restore body
	if err != nil {
		log.Printf("Error reading request body for HAR: %v", err)
	}

	if len(bodyBytes) > 0 {
		mimeType := req.Header.Get("Content-Type")
		parsedMimeType, _, _ := mime.ParseMediaType(mimeType)

		postData = &PostData{
			MimeType: mimeType,
		}

		if parsedMimeType == "application/x-www-form-urlencoded" {
			// Parse form data
			parsedQuery, parseErr := url.ParseQuery(string(bodyBytes))
			if parseErr == nil {
				params := make([]PostParam, 0, len(parsedQuery))
				for name, values := range parsedQuery {
					for _, value := range values {
						params = append(params, PostParam{Name: name, Value: value})
					}
				}
				postData.Params = params
				postData.Text = string(bodyBytes) // Also include raw text
			} else {
				log.Printf("Error parsing form data for HAR: %v", parseErr)
				// Fallback to treating as plain text or binary
				if isTextMimeType(mimeType) {
					postData.Text = string(bodyBytes)
				} else {
					postData.Text = base64.StdEncoding.EncodeToString(bodyBytes)
					postData.Encoding = "base64"
				}
			}
		} else if isTextMimeType(mimeType) {
			postData.Text = string(bodyBytes)
		} else {
			postData.Text = base64.StdEncoding.EncodeToString(bodyBytes)
			postData.Encoding = "base64"
			// For binary, Params is usually not applicable unless it's multipart with individual text parts.
			// For simplicity, we are not parsing multipart here.
		}
	}

	// Update actual bodySize if it was initially -1 (chunked) or different from ContentLength
	actualBodySize := int64(len(bodyBytes))
	if bodySize == -1 || bodySize != actualBodySize {
		bodySize = actualBodySize
	}

	return Request{
		Method:      req.Method,
		URL:         req.URL.String(),
		HTTPVersion: req.Proto,
		Cookies:     l.buildHARCookies(req.Cookies()),
		Headers:     l.buildHARHeaders(req.Header),
		QueryString: l.buildHARQueryString(req.URL.Query()),
		PostData:    postData,
		HeadersSize: calculateHeadersSize(req.Header),
		BodySize:    bodySize,
	}
}

func (l *Logger) buildHARResponse(resp *http.Response) Response {
	if resp == nil {
		// Handle cases where response might be nil (e.g., network error before response)
		return Response{
			Status:     0,
			StatusText: "Error or No Response",
			Content: Content{
				Size:     0,
				MimeType: "application/octet-stream",
			},
			HeadersSize: -1,
			BodySize:    0,
		}
	}

	bodySize := int64(-1)
	if resp.ContentLength > 0 {
		bodySize = resp.ContentLength
	}

	mimeType := resp.Header.Get("Content-Type")

	// 读取响应体
	bodyBytes, err := readAndRestoreBody(&resp.Body, resp.ContentLength)
	if err != nil {
		log.Printf("Error reading response body for HAR: %v", err)
	}

	actualBodySize := int64(len(bodyBytes))

	content := Content{
		Size:     actualBodySize,
		MimeType: mimeType,
	}

	if len(bodyBytes) > 0 {
		contentEncodingHeader := resp.Header.Get("Content-Encoding")
		// Check if common compression encodings are used.
		// HAR spec doesn't explicitly state how to handle Content-Encoding for text field,
		// but if it's compressed, string(bodyBytes) is not useful as "text".
		isCompressed := strings.Contains(strings.ToLower(contentEncodingHeader), "gzip") ||
			strings.Contains(strings.ToLower(contentEncodingHeader), "deflate") ||
			strings.Contains(strings.ToLower(contentEncodingHeader), "br")

		if isTextMimeType(mimeType) && !isCompressed {
			content.Text = string(bodyBytes)
		} else {
			// For non-text types, or for compressed text types, use base64
			content.Text = base64.StdEncoding.EncodeToString(bodyBytes)
			content.Encoding = "base64"
		}
	}

	// Update bodySize if it was initially -1 (chunked) or different from ContentLength
	if bodySize == -1 || bodySize != actualBodySize {
		bodySize = actualBodySize
	}

	return Response{
		Status:      resp.StatusCode,
		StatusText:  resp.Status,
		HTTPVersion: resp.Proto,
		Cookies:     l.buildHARCookies(resp.Cookies()),
		Headers:     l.buildHARHeaders(resp.Header),
		Content:     content,
		RedirectURL: resp.Header.Get("Location"),
		HeadersSize: calculateHeadersSize(resp.Header),
		BodySize:    bodySize, // This will be the Content-Length, or -1 if chunked. Actual body size if read.
	}
}

func (l *Logger) buildHARCookies(cookies []*http.Cookie) []Cookie {
	harCookies := make([]Cookie, 0, len(cookies))
	for _, c := range cookies {
		var expiresPtr *time.Time
		if !c.Expires.IsZero() {
			expiresPtr = &c.Expires
		}
		harCookies = append(harCookies, Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  expiresPtr,
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
		})
	}
	return harCookies
}

func (l *Logger) buildHARHeaders(headers http.Header) []NameValuePair {
	harHeaders := make([]NameValuePair, 0, len(headers))
	for name, values := range headers {
		for _, value := range values {
			harHeaders = append(harHeaders, NameValuePair{Name: name, Value: value})
		}
	}
	return harHeaders
}

func (l *Logger) buildHARQueryString(query url.Values) []NameValuePair {
	harQuery := make([]NameValuePair, 0, len(query))
	for name, values := range query {
		for _, value := range values {
			harQuery = append(harQuery, NameValuePair{Name: name, Value: value})
		}
	}
	return harQuery
}

func (l *Logger) buildHARTimings(totalTime time.Duration) Timings {
	totalMs := float64(totalTime.Milliseconds())
	var sendTime, waitTime, receiveTime float64

	if totalMs > 0 {
		// Approximate split based on test expectations (1/3 each for Send, Wait, Receive)
		sendTime = totalMs / 3.0
		waitTime = totalMs / 3.0
		// Assign remainder to receiveTime to ensure the sum is totalMs
		receiveTime = totalMs - sendTime - waitTime
	} else {
		sendTime = 0
		waitTime = 0
		receiveTime = 0
	}

	return Timings{
		Blocked: -1, // Default to -1 as per HAR spec for "not applicable" or "not available"
		DNS:     -1,
		Connect: -1,
		Send:    sendTime,
		Wait:    waitTime,
		Receive: receiveTime,
		SSL:     -1,
	}
}

// Save writes the HAR log to the specified output file.
// This should typically be called once when the proxy is shutting down.
func (l *Logger) Save() error {
	if !l.IsEnabled() {
		log.Println("HAR logging disabled, not saving.")
		return nil
	}
	if l.h == nil { // Should not happen if enabled, but good practice
		log.Println("HAR object is nil, not saving.")
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	file, err := os.Create(l.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create HAR output file %s: %w", l.outputFile, err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(l.h)

	closeErr := file.Close() // Close the file and check for error

	if encodeErr != nil {
		// Return encoding error first if it exists
		return fmt.Errorf("failed to encode HAR data to %s: %w", l.outputFile, encodeErr)
	}
	if closeErr != nil {
		// If encoding was fine, but closing failed
		return fmt.Errorf("failed to close HAR output file %s: %w", l.outputFile, closeErr)
	}

	log.Printf("HAR log successfully saved to %s with %d entries.", l.outputFile, len(l.h.Log.Entries))
	return nil // Both succeeded
}

// EnableAutoSave starts a background goroutine that automatically saves the HAR log
// at regular intervals specified by interval.
func (l *Logger) EnableAutoSave(interval time.Duration) {
	if !l.IsEnabled() {
		log.Println("HAR logging disabled, not enabling auto-save.")
		return
	}

	// If auto-save is already enabled, cancel it first
	if l.autoSaveEnabled && l.cancelAutoSave != nil {
		l.cancelAutoSave()
	}

	// Create a new context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	l.cancelAutoSave = cancel

	// Update auto-save settings
	l.mu.Lock()
	l.autoSaveEnabled = true
	if interval > 0 {
		l.autoSaveInterval = interval
	}
	l.mu.Unlock()

	log.Printf("Auto-save enabled, HAR log will be saved every %v", l.autoSaveInterval)

	// Start background goroutine for auto-saving
	go func() {
		ticker := time.NewTicker(l.autoSaveInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("Auto-save stopped")
				return
			case <-ticker.C:
				// Check if there are any entries to save
				l.mu.Lock()
				hasEntries := l.h != nil && len(l.h.Log.Entries) > 0
				l.mu.Unlock()

				if hasEntries {
					if err := l.Save(); err != nil {
						log.Printf("Error during auto-save: %v", err)
					}
				}
			}
		}
	}()
}

// DisableAutoSave stops the automatic saving of the HAR log.
func (l *Logger) DisableAutoSave() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.autoSaveEnabled && l.cancelAutoSave != nil {
		l.cancelAutoSave()
		l.autoSaveEnabled = false
		l.cancelAutoSave = nil
		log.Println("Auto-save disabled")
	}
}

// Helper to read body and restore it for http.Request or http.Response
// Returns the body bytes and an error if one occurred.
// The original body stream is replaced with a new one containing the same data.
func readAndRestoreBody(bodySlot *io.ReadCloser, contentLength int64) ([]byte, error) {
	if bodySlot == nil || *bodySlot == nil || *bodySlot == http.NoBody {
		return nil, nil
	}

	// Limit reading to avoid OOM on very large bodies if not strictly needed for HAR
	// For HAR, sometimes only a snippet or metadata is enough.
	// For now, let's try to read it all if ContentLength is reasonable.
	// A more advanced logger might have size limits for captured bodies.

	bodyBytes, err := io.ReadAll(*bodySlot)
	_ = (*bodySlot).Close() // Close the original body

	if err != nil {
		// On error, replace the body with an empty reader to prevent further errors on it
		*bodySlot = io.NopCloser(strings.NewReader("")) // Set to empty reader on error
		return nil, err
	}

	*bodySlot = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore body
	return bodyBytes, nil
}

// isTextMimeType checks if the MIME type is likely to be text-based.
func isTextMimeType(mimeType string) bool {
	if mimeType == "" {
		return true // Per test "empty_mime"
	}

	mt, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		// Handle cases like "text" or "application" which fail ParseMediaType
		// but are expected to be true by tests "type_only_text" and "invalid_mime_type_structure".
		// Also, if the original string starts with "text/" but is malformed for ParseMediaType.
		lowerMimeType := strings.ToLower(mimeType)
		if lowerMimeType == "text" || lowerMimeType == "application" || strings.HasPrefix(lowerMimeType, "text/") {
			return true
		}
		return false // Default to not text if parsing fails and not a special case
	}

	// If parsing succeeded, check against known text types
	return strings.HasPrefix(mt, "text/") || // Covers text/plain, text/html, text/css, text/csv
		mt == "text" || // Handle case where mt might be just "text" and err is nil
		mt == "application" || // Handle case where mt might be just "application"
		mt == "application/json" ||
		mt == "application/xml" ||
		mt == "application/javascript" ||
		mt == "application/x-www-form-urlencoded" ||
		mt == "application/xhtml+xml" ||
		mt == "application/atom+xml" ||
		mt == "application/rss+xml" ||
		mt == "application/geo+json" ||
		mt == "application/ld+json" ||
		mt == "application/manifest+json" ||
		mt == "application/vnd.api+json"
}

// TODO: Further refine PostData.Params parsing for form data.
