package api

import (
	"strings"
	"unicode/utf8"
)

func isTextContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	contentType = strings.ToLower(contentType)

	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)

	knownTextTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/ecmascript",
		"application/x-www-form-urlencoded",
		"application/xhtml+xml",
		"application/atom+xml",
		"application/rss+xml",
		"application/soap+xml",
		"application/x-yaml",
		"application/yaml",
		"application/graphql",
		"message/rfc822",
	}

	for _, textType := range knownTextTypes {
		if strings.HasPrefix(contentType, textType) {
			return true
		}
	}

	knownTextSuffixes := []string{
		"+json",
		"+xml",
		"+text",
	}

	for _, suffix := range knownTextSuffixes {
		if strings.HasSuffix(contentType, suffix) {
			return true
		}
	}

	otherTextTypes := map[string]bool{
		"application/json-patch+json":  true,
		"application/merge-patch+json": true,
		"application/schema+json":      true,
		"application/vnd.api+json":     true,
		"application/vnd.github+json":  true,
		"application/problem+json":     true,
		"application/x-httpd-php":      true,
		"application/x-sh":             true,
		"application/x-csh":            true,
		"application/typescript":       true,
		"application/sql":              true,
		"application/csv":              true,
		"application/x-csv":            true,
		"text/csv":                     true,
		"application/ld+json":          true,
	}

	return otherTextTypes[contentType]
}

func isBinaryContent(data []byte, contentType string) bool {
	if contentType != "" {
		contentTypeLower := strings.ToLower(contentType)

		if idx := strings.Index(contentTypeLower, ";"); idx >= 0 {
			contentTypeLower = contentTypeLower[:idx]
		}
		contentTypeLower = strings.TrimSpace(contentTypeLower)

		if isTextContentType(contentTypeLower) {
			if len(data) < 32 {
				// Continue to check short content below.
			} else {
				return false
			}
		}

		binaryPrefixes := []string{
			"image/",
			"audio/",
			"video/",
			"application/octet-stream",
			"application/pdf",
			"application/zip",
			"application/x-gzip",
			"application/x-tar",
			"application/x-7z-compressed",
			"application/x-rar-compressed",
			"application/x-msdownload",
			"application/vnd.ms-",
			"application/vnd.openxmlformats-",
			"font/",
			"model/",
		}

		for _, prefix := range binaryPrefixes {
			if strings.HasPrefix(contentTypeLower, prefix) {
				return true
			}
		}
	}

	if len(data) == 0 {
		return false
	}

	if utf8.Valid(data) {
		controlCount := 0
		totalCount := 0
		maxSamples := 1024

		bytesToCheck := len(data)
		if bytesToCheck > maxSamples {
			bytesToCheck = maxSamples
		}

		for i := 0; i < bytesToCheck; i++ {
			b := data[i]
			totalCount++
			if b < 32 && b != 9 && b != 10 && b != 13 {
				controlCount++
			}
		}

		if float64(controlCount)/float64(totalCount) > 0.15 {
			return true
		}

		return false
	}

	return true
}
