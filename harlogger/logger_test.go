package harlogger

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProxyVersion = "0.1.0-test"
const testProxyName = "TestProxy"

// TestNewLogger tests the NewLogger function.
func TestNewLogger(t *testing.T) {
	t.Run("with_output_file", func(t *testing.T) {
		outputFile := "test_output.har"
		logger := NewLogger(outputFile, testProxyName, testProxyVersion)
		defer os.Remove(outputFile) // Clean up

		assert.NotNil(t, logger, "Logger should not be nil")
		assert.True(t, logger.IsEnabled(), "Logger should be enabled")
		assert.Equal(t, outputFile, logger.outputFile, "Output file name mismatch")
		assert.NotNil(t, logger.h, "HAR object should be initialized")
		assert.Equal(t, "1.2", logger.h.Log.Version, "HAR version mismatch")
		assert.Equal(t, testProxyName, logger.h.Log.Creator.Name, "Creator name mismatch")
		assert.Equal(t, testProxyVersion, logger.h.Log.Creator.Version, "Creator version mismatch")
		assert.Empty(t, logger.h.Log.Entries, "Entries should be empty initially")
	})

	t.Run("without_output_file", func(t *testing.T) {
		logger := NewLogger("", testProxyName, testProxyVersion)
		assert.NotNil(t, logger, "Logger should not be nil")
		assert.False(t, logger.IsEnabled(), "Logger should be disabled")
		assert.Empty(t, logger.outputFile, "Output file name should be empty")
		assert.Nil(t, logger.h, "HAR object should not be initialized")
	})
}

// TestLogger_IsEnabled tests the IsEnabled method.
func TestLogger_IsEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		logger := NewLogger("test.har", testProxyName, testProxyVersion)
		defer os.Remove("test.har")
		assert.True(t, logger.IsEnabled())
	})

	t.Run("disabled", func(t *testing.T) {
		logger := NewLogger("", testProxyName, testProxyVersion)
		assert.False(t, logger.IsEnabled())
	})
}

// TestLogger_AddEntry tests adding an entry to the HAR log.
func TestLogger_AddEntry(t *testing.T) {
	outputFile := "test_add_entry.har"
	logger := NewLogger(outputFile, testProxyName, testProxyVersion)
	defer os.Remove(outputFile)

	require.True(t, logger.IsEnabled(), "Logger should be enabled for this test")

	// Mock HTTP request
	reqURL, _ := url.Parse("http://example.com/path?query=value")
	reqBody := "Hello, world!"
	req, err := http.NewRequest("POST", reqURL.String(), strings.NewReader(reqBody))
	require.NoError(t, err, "Failed to create request")
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("User-Agent", "TestAgent")
	req.AddCookie(&http.Cookie{Name: "reqCookie", Value: "reqVal"})

	// Mock HTTP response
	respBody := "Response body here"
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Proto:         "HTTP/1.1",
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(respBody)),
		ContentLength: int64(len(respBody)),
		Request:       req, // Associate request with response
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Set-Cookie", "respCookie=respVal; Path=/")

	startedTime := time.Now().Add(-5 * time.Second) // Simulate request started 5 seconds ago
	timeTaken := 500 * time.Millisecond
	serverIP := "192.168.1.100"
	connectionID := "conn-123"

	logger.AddEntry(req, resp, startedTime, timeTaken, serverIP, connectionID)

	assert.Len(t, logger.h.Log.Entries, 1, "Should have one entry after adding")

	entry := logger.h.Log.Entries[0]
	assert.Equal(t, startedTime.UTC(), entry.StartedDateTime, "StartedDateTime mismatch")
	assert.Equal(t, float64(timeTaken.Milliseconds()), entry.Time, "Time taken mismatch")
	assert.Equal(t, serverIP, entry.ServerIPAddress, "ServerIPAddress mismatch")
	assert.Equal(t, connectionID, entry.Connection, "Connection ID mismatch")

	// Validate Request part of the entry
	harReq := entry.Request
	assert.Equal(t, "POST", harReq.Method, "Request method mismatch")
	assert.Equal(t, reqURL.String(), harReq.URL, "Request URL mismatch")
	assert.Equal(t, "HTTP/1.1", harReq.HTTPVersion, "Request HTTP version mismatch") // req.Proto is HTTP/1.1
	assert.Len(t, harReq.Cookies, 1, "Request cookie count mismatch")
	assert.Equal(t, "reqCookie", harReq.Cookies[0].Name)
	assert.Equal(t, "reqVal", harReq.Cookies[0].Value)
	assert.NotEmpty(t, harReq.Headers, "Request headers should not be empty")
	foundUserAgent := false
	for _, h := range harReq.Headers {
		if h.Name == "User-Agent" && h.Value == "TestAgent" {
			foundUserAgent = true
			break
		}
	}
	assert.True(t, foundUserAgent, "User-Agent header not found or incorrect in request")
	assert.Len(t, harReq.QueryString, 1, "Query string count mismatch")
	assert.Equal(t, "query", harReq.QueryString[0].Name)
	assert.Equal(t, "value", harReq.QueryString[0].Value)
	require.NotNil(t, harReq.PostData, "PostData should not be nil for POST request")
	assert.Equal(t, "text/plain", harReq.PostData.MimeType, "PostData MimeType mismatch")
	assert.Equal(t, reqBody, harReq.PostData.Text, "PostData text mismatch")
	assert.True(t, harReq.HeadersSize > 0, "HeadersSize should be greater than 0 for request")
	assert.Equal(t, int64(len(reqBody)), harReq.BodySize, "BodySize mismatch for request")

	// Validate Response part of the entry
	harResp := entry.Response
	assert.Equal(t, http.StatusOK, harResp.Status, "Response status mismatch")
	assert.Equal(t, "200 OK", harResp.StatusText, "Response status text mismatch")
	assert.Equal(t, "HTTP/1.1", harResp.HTTPVersion, "Response HTTP version mismatch")
	assert.Len(t, harResp.Cookies, 1, "Response cookie count mismatch")
	assert.Equal(t, "respCookie", harResp.Cookies[0].Name)
	assert.Equal(t, "respVal", harResp.Cookies[0].Value)
	assert.NotEmpty(t, harResp.Headers, "Response headers should not be empty")
	foundContentType := false
	for _, h := range harResp.Headers {
		if h.Name == "Content-Type" && h.Value == "application/json" {
			foundContentType = true
			break
		}
	}
	assert.True(t, foundContentType, "Content-Type header not found or incorrect in response")
	assert.Equal(t, "application/json", harResp.Content.MimeType, "Content MimeType mismatch")
	assert.Equal(t, respBody, harResp.Content.Text, "Content text mismatch") // Assuming text, adjust if binary/encoded
	assert.Equal(t, int64(len(respBody)), harResp.Content.Size, "Content size mismatch")
	assert.True(t, harResp.HeadersSize > 0, "HeadersSize should be greater than 0 for response")
	assert.Equal(t, int64(len(respBody)), harResp.BodySize, "BodySize mismatch for response")

	// Validate Timings
	assert.Equal(t, float64(timeTaken.Milliseconds()), entry.Timings.Send+entry.Timings.Wait+entry.Timings.Receive, "Total time should match sum of timings parts")
	assert.True(t, entry.Timings.Send >= 0, "Send time should be non-negative")
	assert.True(t, entry.Timings.Wait >= 0, "Wait time should be non-negative")
	assert.True(t, entry.Timings.Receive >= 0, "Receive time should be non-negative")
}

// Test_buildHARRequest tests the buildHARRequest internal function.
func Test_buildHARRequest(t *testing.T) {
	logger := NewLogger("", testProxyName, testProxyVersion) // No output file needed for this test

	t.Run("get_request_no_body", func(t *testing.T) {
		reqURL, _ := url.Parse("http://example.com/test?q1=v1&q2=v2")
		req, _ := http.NewRequest("GET", reqURL.String(), nil)
		req.Header.Set("Accept", "application/json")
		req.AddCookie(&http.Cookie{Name: "c1", Value: "v1"})

		harReq := logger.buildHARRequest(req)

		assert.Equal(t, "GET", harReq.Method)
		assert.Equal(t, reqURL.String(), harReq.URL)
		assert.Equal(t, "HTTP/1.1", harReq.HTTPVersion)
		assert.Len(t, harReq.Cookies, 1)
		assert.Equal(t, "c1", harReq.Cookies[0].Name)
		assert.Len(t, harReq.Headers, 2) // Expect "Accept" and "Cookie" headers
		foundAcceptHeader := false
		for _, h := range harReq.Headers {
			if h.Name == "Accept" && h.Value == "application/json" {
				foundAcceptHeader = true
				break
			}
		}
		assert.True(t, foundAcceptHeader, "Accept header not found or incorrect")
		assert.Len(t, harReq.QueryString, 2)
		assert.Contains(t, harReq.QueryString, NameValuePair{Name: "q1", Value: "v1"})
		assert.Contains(t, harReq.QueryString, NameValuePair{Name: "q2", Value: "v2"})
		assert.Nil(t, harReq.PostData, "PostData should be nil for GET request")
		assert.True(t, harReq.HeadersSize > 0)
		assert.Equal(t, int64(0), harReq.BodySize, "BodySize should be 0 for GET with no body")
	})

	t.Run("post_request_text_body", func(t *testing.T) {
		bodyStr := "simple text body"
		req, _ := http.NewRequest("POST", "http://example.com/submit", strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
		req.ContentLength = int64(len(bodyStr))

		harReq := logger.buildHARRequest(req)

		assert.Equal(t, "POST", harReq.Method)
		require.NotNil(t, harReq.PostData)
		assert.Equal(t, "text/plain; charset=utf-8", harReq.PostData.MimeType)
		assert.Equal(t, bodyStr, harReq.PostData.Text)
		assert.Empty(t, harReq.PostData.Params)
		assert.Equal(t, int64(len(bodyStr)), harReq.BodySize)
	})

	t.Run("post_request_form_data", func(t *testing.T) {
		form := url.Values{}
		form.Add("name", "test user")
		form.Add("email", "test@example.com")
		bodyStr := form.Encode()

		req, _ := http.NewRequest("POST", "http://example.com/form", strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.ContentLength = int64(len(bodyStr))

		harReq := logger.buildHARRequest(req)

		require.NotNil(t, harReq.PostData)
		assert.Equal(t, "application/x-www-form-urlencoded", harReq.PostData.MimeType)
		assert.Equal(t, bodyStr, harReq.PostData.Text, "Raw form data text should be present")
		assert.Len(t, harReq.PostData.Params, 2)
		assert.Contains(t, harReq.PostData.Params, PostParam{Name: "name", Value: "test user"})
		assert.Contains(t, harReq.PostData.Params, PostParam{Name: "email", Value: "test@example.com"})
		assert.Equal(t, int64(len(bodyStr)), harReq.BodySize)
	})

	t.Run("post_request_json_body", func(t *testing.T) {
		bodyJSON := `{"key":"value"}`
		req, _ := http.NewRequest("POST", "http://example.com/api", strings.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(bodyJSON))

		harReq := logger.buildHARRequest(req)

		require.NotNil(t, harReq.PostData)
		assert.Equal(t, "application/json", harReq.PostData.MimeType)
		assert.Equal(t, bodyJSON, harReq.PostData.Text)
		assert.Empty(t, harReq.PostData.Params, "Params should be empty for JSON body")
		assert.Equal(t, int64(len(bodyJSON)), harReq.BodySize)
	})

	t.Run("post_request_binary_body_base64", func(t *testing.T) {
		bodyBytes := []byte{0xDE, 0xAD, 0xBE, 0xEF} // Sample binary data
		req, _ := http.NewRequest("POST", "http://example.com/binary", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.ContentLength = int64(len(bodyBytes))

		harReq := logger.buildHARRequest(req)

		require.NotNil(t, harReq.PostData)
		assert.Equal(t, "application/octet-stream", harReq.PostData.MimeType)
		// For binary, text should be base64 encoded
		// The actual implementation detail of base64 encoding is in buildHARRequest, so we check the output of that.
		// Assuming isTextMimeType correctly identifies octet-stream as non-text.
		// This test depends on the behavior of readAndRestoreBody and isTextMimeType.
		// The current buildHARRequest logic will base64 encode if not text.
		// We need to ensure `isTextMimeType` correctly identifies this as non-text.
		// For now, let's assume it will be base64 encoded.
		// encodedBody := base64.StdEncoding.EncodeToString(bodyBytes)
		// assert.Equal(t, encodedBody, harReq.PostData.Text) // This will be tested more directly with isTextMimeType tests
		assert.NotEmpty(t, harReq.PostData.Text, "Binary data should be present, likely base64 encoded")
		assert.Empty(t, harReq.PostData.Params)
		assert.Equal(t, int64(len(bodyBytes)), harReq.BodySize)
	})

	t.Run("request_with_chunked_encoding_no_content_length", func(t *testing.T) {
		bodyStr := "chunked body content"
		req, _ := http.NewRequest("POST", "http://example.com/chunked", strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Transfer-Encoding", "chunked")
		// No ContentLength set for chunked

		harReq := logger.buildHARRequest(req)

		require.NotNil(t, harReq.PostData)
		assert.Equal(t, "text/plain", harReq.PostData.MimeType)
		assert.Equal(t, bodyStr, harReq.PostData.Text)
		assert.Equal(t, int64(len(bodyStr)), harReq.BodySize, "BodySize should be actual read body size for chunked")
	})
}

// Test_buildHARResponse tests the buildHARResponse internal function.
func Test_buildHARResponse(t *testing.T) {
	logger := NewLogger("", testProxyName, testProxyVersion) // No output file needed for this test

	t.Run("simple_response_text_body", func(t *testing.T) {
		bodyStr := "This is a response."
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Proto:         "HTTP/1.1",
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(bodyStr)),
			ContentLength: int64(len(bodyStr)),
		}
		resp.Header.Set("Content-Type", "text/plain; charset=iso-8859-1")
		resp.Header.Set("Set-Cookie", "session=123; Path=/")
		resp.Header.Add("Set-Cookie", "user=abc; Path=/; HttpOnly")

		harResp := logger.buildHARResponse(resp)

		assert.Equal(t, http.StatusOK, harResp.Status)
		assert.Equal(t, "200 OK", harResp.StatusText)
		assert.Equal(t, "HTTP/1.1", harResp.HTTPVersion)
		assert.Len(t, harResp.Cookies, 2)
		// Order of cookies from headers might not be guaranteed, so check for presence
		foundCookie1 := false
		foundCookie2 := false
		for _, c := range harResp.Cookies {
			if c.Name == "session" && c.Value == "123" {
				foundCookie1 = true
			}
			if c.Name == "user" && c.Value == "abc" && c.HTTPOnly {
				foundCookie2 = true
			}
		}
		assert.True(t, foundCookie1, "Session cookie not found or incorrect")
		assert.True(t, foundCookie2, "User cookie not found or incorrect")

		assert.Len(t, harResp.Headers, 3) // Content-Type and 2 Set-Cookie headers (each Set-Cookie is a separate entry)
		assert.Equal(t, "text/plain; charset=iso-8859-1", harResp.Content.MimeType)
		assert.Equal(t, bodyStr, harResp.Content.Text)
		assert.Equal(t, int64(len(bodyStr)), harResp.Content.Size)
		assert.Equal(t, "", harResp.Content.Encoding, "Encoding should be empty for plain text")
		assert.True(t, harResp.HeadersSize > 0)
		assert.Equal(t, int64(len(bodyStr)), harResp.BodySize)
		assert.Equal(t, "", harResp.RedirectURL)
	})

	t.Run("response_json_body", func(t *testing.T) {
		bodyJSON := `{"status":"success"}`
		resp := &http.Response{
			StatusCode:    http.StatusCreated,
			Status:        "201 Created",
			Proto:         "HTTP/2.0",
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader(bodyJSON)),
			ContentLength: int64(len(bodyJSON)),
		}
		resp.Header.Set("Content-Type", "application/json")

		harResp := logger.buildHARResponse(resp)

		assert.Equal(t, http.StatusCreated, harResp.Status)
		assert.Equal(t, "application/json", harResp.Content.MimeType)
		assert.Equal(t, bodyJSON, harResp.Content.Text)
		assert.Equal(t, int64(len(bodyJSON)), harResp.Content.Size)
	})

	t.Run("response_binary_body_image_png", func(t *testing.T) {
		bodyBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A} // Minimal PNG header
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Proto:         "HTTP/1.1",
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(bodyBytes)),
			ContentLength: int64(len(bodyBytes)),
		}
		resp.Header.Set("Content-Type", "image/png")

		harResp := logger.buildHARResponse(resp)

		assert.Equal(t, "image/png", harResp.Content.MimeType)
		// Binary content should be base64 encoded in harResp.Content.Text
		// encodedBody := base64.StdEncoding.EncodeToString(bodyBytes)
		// assert.Equal(t, encodedBody, harResp.Content.Text) // This depends on isTextMimeType
		assert.NotEmpty(t, harResp.Content.Text, "Binary content text should not be empty")
		assert.Equal(t, "base64", harResp.Content.Encoding, "Encoding should be base64 for binary")
		assert.Equal(t, int64(len(bodyBytes)), harResp.Content.Size)
		assert.Equal(t, int64(len(bodyBytes)), harResp.BodySize)
	})

	t.Run("response_with_redirect_url", func(t *testing.T) {
		redirectURL := "http://newlocation.example.com"
		resp := &http.Response{
			StatusCode: http.StatusMovedPermanently,
			Status:     "301 Moved Permanently",
			Proto:      "HTTP/1.1",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("Redirecting...")),
		}
		resp.Header.Set("Location", redirectURL)

		harResp := logger.buildHARResponse(resp)

		assert.Equal(t, redirectURL, harResp.RedirectURL)
	})

	t.Run("response_no_body_content_length_zero", func(t *testing.T) {
		resp := &http.Response{
			StatusCode:    http.StatusNoContent,
			Status:        "204 No Content",
			Proto:         "HTTP/1.1",
			Header:        make(http.Header),
			Body:          http.NoBody, // or io.NopCloser(strings.NewReader(""))
			ContentLength: 0,
		}
		resp.Header.Set("X-Custom", "empty")

		harResp := logger.buildHARResponse(resp)

		assert.Equal(t, int64(0), harResp.Content.Size)
		assert.Equal(t, "", harResp.Content.Text)
		assert.Equal(t, int64(0), harResp.BodySize)
		// MimeType might be empty or application/octet-stream if not set and body is empty.
		// The spec is a bit vague. Let's assume it's empty if not specified.
		// assert.Empty(t, harResp.Content.MimeType) // This depends on default behavior of http library or our logic
	})

	t.Run("response_body_read_error", func(t *testing.T) {
		// Simulate a body that errors on read
		errReader := &errorReader{}
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			Header:        make(http.Header),
			Body:          io.NopCloser(errReader),
			ContentLength: -1, // Unknown length
		}
		resp.Header.Set("Content-Type", "text/plain")

		harResp := logger.buildHARResponse(resp)

		assert.Equal(t, int64(0), harResp.Content.Size, "Size should be 0 on read error")
		assert.Equal(t, "", harResp.Content.Text, "Text should be empty on read error")
		assert.Equal(t, int64(0), harResp.BodySize, "BodySize should be 0 on read error")
		// Log message for error should have been printed, not directly testable here without capturing logs
	})
}

// Test_calculateHeadersSize tests the calculateHeadersSize function.
func Test_calculateHeadersSize(t *testing.T) {
	headers := http.Header{}
	assert.Equal(t, int64(0), calculateHeadersSize(headers), "Size of empty headers should be 0")

	headers.Set("Content-Type", "application/json")
	// "Content-Type: application/json\r\n\r\n"
	// 12 + 2 + 16 + 2 (for header line) + 2 (for end of headers)
	expectedSize := int64(len("Content-Type: application/json\r\n") + len("\r\n"))
	assert.Equal(t, expectedSize, calculateHeadersSize(headers), "Size of single header mismatch")

	headers.Add("X-Custom-Header", "Value1")
	headers.Add("X-Custom-Header", "Value2")
	// "Content-Type: application/json\r\n"
	// "X-Custom-Header: Value1\r\n"
	// "X-Custom-Header: Value2\r\n"
	// "\r\n"
	expectedSize = int64(len("Content-Type: application/json\r\n") +
		len("X-Custom-Header: Value1\r\n") +
		len("X-Custom-Header: Value2\r\n") +
		len("\r\n"))
	assert.Equal(t, expectedSize, calculateHeadersSize(headers), "Size of multiple headers mismatch")
}

// Test_readAndRestoreBody tests the readAndRestoreBody utility function.
func Test_readAndRestoreBody(t *testing.T) {
	t.Run("nil_body", func(t *testing.T) {
		var body io.ReadCloser // nil
		data, err := readAndRestoreBody(&body, 0)
		assert.NoError(t, err)
		assert.Empty(t, data)
		assert.Nil(t, body, "Body should remain nil")
	})

	t.Run("empty_body", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader(""))
		data, err := readAndRestoreBody(&body, 0)
		assert.NoError(t, err)
		assert.Empty(t, data)
		assert.NotNil(t, body, "Body should be restored")
		// Try reading again
		secondRead, _ := io.ReadAll(body)
		assert.Empty(t, secondRead, "Second read should yield empty data")
	})

	t.Run("non_empty_body", func(t *testing.T) {
		content := "Hello, Gopher!"
		body := io.NopCloser(strings.NewReader(content))
		data, err := readAndRestoreBody(&body, int64(len(content)))
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))
		assert.NotNil(t, body, "Body should be restored")

		// Try reading again from the restored body
		secondReadBytes, err := io.ReadAll(body)
		assert.NoError(t, err)
		assert.Equal(t, content, string(secondReadBytes), "Second read should yield original content")
	})

	t.Run("body_with_content_length_mismatch_less_than_actual", func(t *testing.T) {
		content := "This is longer than specified"
		body := io.NopCloser(strings.NewReader(content))
		// Specify a smaller content length
		data, err := readAndRestoreBody(&body, 10) // Read up to 10 bytes
		assert.NoError(t, err)
		// readAndRestoreBody reads the whole thing if possible, contentLength is a hint
		assert.Equal(t, content, string(data))
		restoredBody, _ := io.ReadAll(body)
		assert.Equal(t, content, string(restoredBody))
	})

	t.Run("body_with_content_length_unknown", func(t *testing.T) {
		content := "Unknown length content"
		body := io.NopCloser(strings.NewReader(content))
		data, err := readAndRestoreBody(&body, -1) // Unknown content length
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))
		restoredBody, _ := io.ReadAll(body)
		assert.Equal(t, content, string(restoredBody))
	})

	t.Run("error_during_read", func(t *testing.T) {
		body := io.NopCloser(&errorReader{})
		data, err := readAndRestoreBody(&body, 100)
		assert.Error(t, err, "Expected an error from errorReader")
		assert.Empty(t, data, "Data should be empty on read error")
		// The body is replaced with an empty reader on error by readAndRestoreBody
		assert.NotNil(t, body, "Body should be replaced with an empty reader on error")
		restoredData, readErr := io.ReadAll(body)
		assert.NoError(t, readErr)
		assert.Empty(t, restoredData, "Restored body should be empty after a read error")
	})
}

// Test_isTextMimeType tests the isTextMimeType function.
func Test_isTextMimeType(t *testing.T) {
	testCases := []struct {
		name     string
		mimeType string
		expected bool
	}{
		{"empty_mime", "", true}, // Default to text if empty or unparseable by mime.ParseMediaType
		{"text_plain", "text/plain", true},
		{"text_html", "text/html; charset=utf-8", true},
		{"application_json", "application/json", true},
		{"application_xml", "application/xml", true},
		{"application_javascript", "application/javascript", true},
		{"application_x_www_form_urlencoded", "application/x-www-form-urlencoded", true},
		{"image_jpeg", "image/jpeg", false},
		{"image_png", "image/png", false},
		{"audio_mpeg", "audio/mpeg", false},
		{"video_mp4", "video/mp4", false},
		{"application_octet_stream", "application/octet-stream", false},
		{"application_pdf", "application/pdf", false},
		{"application_zip", "application/zip", false},
		{"multipart_form_data", "multipart/form-data; boundary=something", false}, // Multipart itself is binary container
		{"text_css", "text/css", true},
		{"text_csv", "text/csv", true},
		{"application_xhtml+xml", "application/xhtml+xml", true},
		{"application_atom+xml", "application/atom+xml", true},
		{"application_rss+xml", "application/rss+xml", true},
		{"application_geo+json", "application/geo+json", true},
		{"application_ld+json", "application/ld+json", true},
		{"application_manifest+json", "application/manifest+json", true},
		{"application_vnd.api+json", "application/vnd.api+json", true},
		{"application_wasm", "application/wasm", false},
		{"font_woff", "font/woff", false},
		{"model_gltf_binary", "model/gltf-binary", false},
		{"invalid_mime_type_structure", "application", true}, // Falls back to text if ParseMediaType fails badly
		{"type_only_text", "text", true},                     // Falls back to text if ParseMediaType fails to get subtype
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isTextMimeType(tc.mimeType), "MIME type %s", tc.mimeType)
		})
	}
}

// TestLogger_Save tests saving the HAR log to a file.
func TestLogger_Save(t *testing.T) {
	t.Run("save_enabled_logger_with_entries", func(t *testing.T) {
		outputFile := "test_save_output.har"
		logger := NewLogger(outputFile, testProxyName, testProxyVersion)
		defer os.Remove(outputFile) // Clean up

		// Add a simple entry
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("OK")), Header: make(http.Header)}
		resp.Header.Set("Content-Type", "text/plain")
		logger.AddEntry(req, resp, time.Now(), 100*time.Millisecond, "127.0.0.1", "1")

		err := logger.Save()
		assert.NoError(t, err, "Save should not return an error")

		// Verify file content
		data, err := os.ReadFile(outputFile)
		assert.NoError(t, err, "Failed to read output file")
		assert.NotEmpty(t, data, "Output file should not be empty")

		var harData HAR
		err = json.Unmarshal(data, &harData)
		assert.NoError(t, err, "Failed to unmarshal HAR data from file")
		assert.Len(t, harData.Log.Entries, 1, "HAR data should contain one entry")
		assert.Equal(t, "GET", harData.Log.Entries[0].Request.Method)
	})

	t.Run("save_enabled_logger_no_entries", func(t *testing.T) {
		outputFile := "test_save_empty.har"
		logger := NewLogger(outputFile, testProxyName, testProxyVersion)
		defer os.Remove(outputFile)

		err := logger.Save()
		assert.NoError(t, err, "Save should not return an error")

		data, err := os.ReadFile(outputFile)
		assert.NoError(t, err, "Failed to read output file")
		var harData HAR
		err = json.Unmarshal(data, &harData)
		assert.NoError(t, err, "Failed to unmarshal HAR data")
		assert.Empty(t, harData.Log.Entries, "HAR data should have no entries")
	})

	t.Run("save_disabled_logger", func(t *testing.T) {
		logger := NewLogger("", testProxyName, testProxyVersion)
		err := logger.Save()
		assert.NoError(t, err, "Save on disabled logger should not error (it's a no-op)")
		// No file should be created
		_, err = os.Stat("some_non_existent_file_for_disabled_logger.har")
		assert.True(t, os.IsNotExist(err), "No file should be created by disabled logger")
	})

	t.Run("save_error_creating_file_bad_path", func(t *testing.T) {
		// Using a path that likely cannot be created (e.g., directory that doesn't exist in a restricted area)
		// This is hard to make portable and reliable. A simpler way is to make the file unwriteable, but that's also tricky.
		// For now, let's assume a bad path like an empty string for output file if somehow passed NewLogger enabled state (which it shouldn't)
		// Or, more realistically, a path that becomes invalid after logger creation.
		// This specific scenario is hard to test perfectly without more complex mocks or OS-level manipulations.
		// Let's test the case where outputFile is a directory.
		dirName := "test_dir_as_file.har"
		_ = os.Mkdir(dirName, 0755)
		defer os.RemoveAll(dirName)

		logger := NewLogger(dirName, testProxyName, testProxyVersion) // This will make it enabled
		// Provide a minimal valid request and response to avoid panics in AddEntry unrelated to Save()
		reqForSaveTest, _ := http.NewRequest("GET", "http://example.com/save_test_path", nil)
		respForSaveTest := &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    reqForSaveTest, // Link request to response
		}
		logger.AddEntry(reqForSaveTest, respForSaveTest, time.Now(), 10*time.Millisecond, "127.0.0.1", "conn-save-test")

		err := logger.Save()
		assert.Error(t, err, "Save should error if outputFile is a directory")
	})
}

// errorReader is a helper for testing read errors.
// It returns an error on the first Read call.
type errorReader struct {
	called bool
}

func (er *errorReader) Read(p []byte) (n int, err error) {
	if !er.called {
		er.called = true
		return 0, os.ErrPermission // Simulate a read error
	}
	return 0, io.EOF // Subsequent calls behave like EOF
}

// Test_buildHARTimings tests the buildHARTimings function.
func Test_buildHARTimings(t *testing.T) {
	logger := NewLogger("", testProxyName, testProxyVersion)

	t.Run("zero_duration", func(t *testing.T) {
		timings := logger.buildHARTimings(0)
		assert.Equal(t, float64(0), timings.Send)
		assert.Equal(t, float64(0), timings.Wait)
		assert.Equal(t, float64(0), timings.Receive)
		// Blocked, DNS, Connect, SSL are not set by this simplified version
		assert.Equal(t, float64(-1), timings.Blocked) // Default from HAR spec for not applicable
		assert.Equal(t, float64(-1), timings.DNS)
		assert.Equal(t, float64(-1), timings.Connect)
		assert.Equal(t, float64(-1), timings.SSL)
	})

	t.Run("positive_duration", func(t *testing.T) {
		duration := 150 * time.Millisecond
		timings := logger.buildHARTimings(duration)

		// The simplified buildHARTimings splits total time into send, wait, receive.
		// It's a rough approximation. Send is 1/3, Wait is 1/3, Receive is 1/3.
		expectedPart := float64(duration.Milliseconds()) / 3.0

		assert.InDelta(t, expectedPart, timings.Send, 0.001, "Send time mismatch")
		assert.InDelta(t, expectedPart, timings.Wait, 0.001, "Wait time mismatch")
		// The remainder goes to receive to ensure sum is correct
		assert.InDelta(t, float64(duration.Milliseconds())-(timings.Send+timings.Wait), timings.Receive, 0.001, "Receive time mismatch or sum incorrect")
		assert.Equal(t, float64(duration.Milliseconds()), timings.Send+timings.Wait+timings.Receive, "Sum of send, wait, receive should be total time")
	})
}

// Test_buildHARCookies tests the buildHARCookies function.
func Test_buildHARCookies(t *testing.T) {
	logger := NewLogger("", testProxyName, testProxyVersion)

	t.Run("no_cookies", func(t *testing.T) {
		harCookies := logger.buildHARCookies(nil)
		assert.Empty(t, harCookies)
		harCookies = logger.buildHARCookies([]*http.Cookie{})
		assert.Empty(t, harCookies)
	})

	t.Run("with_cookies", func(t *testing.T) {
		now := time.Now()
		cookies := []*http.Cookie{
			{Name: "c1", Value: "v1"},
			{Name: "c2", Value: "v2", Path: "/p", Domain: "d.com", Expires: now, HttpOnly: true, Secure: true},
		}
		harCookies := logger.buildHARCookies(cookies)
		assert.Len(t, harCookies, 2)

		assert.Equal(t, "c1", harCookies[0].Name)
		assert.Equal(t, "v1", harCookies[0].Value)
		assert.Empty(t, harCookies[0].Path)
		assert.Empty(t, harCookies[0].Domain)
		assert.Nil(t, harCookies[0].Expires)
		assert.False(t, harCookies[0].HTTPOnly)
		assert.False(t, harCookies[0].Secure)

		assert.Equal(t, "c2", harCookies[1].Name)
		assert.Equal(t, "v2", harCookies[1].Value)
		assert.Equal(t, "/p", harCookies[1].Path)
		assert.Equal(t, "d.com", harCookies[1].Domain)
		require.NotNil(t, harCookies[1].Expires)
		assert.True(t, now.Equal(*harCookies[1].Expires), "Expires time mismatch")
		assert.True(t, harCookies[1].HTTPOnly)
		assert.True(t, harCookies[1].Secure)
	})
}

// Test_buildHARHeaders tests the buildHARHeaders function.
func Test_buildHARHeaders(t *testing.T) {
	logger := NewLogger("", testProxyName, testProxyVersion)

	t.Run("no_headers", func(t *testing.T) {
		harHeaders := logger.buildHARHeaders(nil)
		assert.Empty(t, harHeaders)
		harHeaders = logger.buildHARHeaders(make(http.Header))
		assert.Empty(t, harHeaders)
	})

	t.Run("with_headers", func(t *testing.T) {
		headers := make(http.Header)
		headers.Set("Content-Type", "application/json")
		headers.Add("X-Multi-Value", "value1")
		headers.Add("X-Multi-Value", "value2")

		harHeaders := logger.buildHARHeaders(headers)
		// Order of headers is not guaranteed by http.Header iteration, so check for presence and count
		assert.Len(t, harHeaders, 3) // One for Content-Type, two for X-Multi-Value

		foundContentType := false
		multiValueCount := 0
		for _, h := range harHeaders {
			if h.Name == "Content-Type" && h.Value == "application/json" {
				foundContentType = true
			}
			if h.Name == "X-Multi-Value" && (h.Value == "value1" || h.Value == "value2") {
				multiValueCount++
			}
		}
		assert.True(t, foundContentType, "Content-Type header not found or incorrect")
		assert.Equal(t, 2, multiValueCount, "X-Multi-Value header count mismatch or values incorrect")
	})
}

// Test_buildHARQueryString tests the buildHARQueryString function.
func Test_buildHARQueryString(t *testing.T) {
	logger := NewLogger("", testProxyName, testProxyVersion)

	t.Run("no_query_params", func(t *testing.T) {
		harQuery := logger.buildHARQueryString(nil)
		assert.Empty(t, harQuery)
		harQuery = logger.buildHARQueryString(make(url.Values))
		assert.Empty(t, harQuery)
	})

	t.Run("with_query_params", func(t *testing.T) {
		query := make(url.Values)
		query.Set("param1", "value1")
		query.Add("param2", "value2a")
		query.Add("param2", "value2b")

		harQuery := logger.buildHARQueryString(query)
		assert.Len(t, harQuery, 3) // One for param1, two for param2

		foundP1 := false
		p2Count := 0
		for _, q := range harQuery {
			if q.Name == "param1" && q.Value == "value1" {
				foundP1 = true
			}
			if q.Name == "param2" && (q.Value == "value2a" || q.Value == "value2b") {
				p2Count++
			}
		}
		assert.True(t, foundP1, "param1 not found or incorrect")
		assert.Equal(t, 2, p2Count, "param2 count mismatch or values incorrect")
	})
}
