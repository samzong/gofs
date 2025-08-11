// Package httprange implements HTTP Range request parsing and response handling
// according to RFC 7233. It provides a production-ready implementation for
// supporting resumable downloads in HTTP file servers.
package httprange

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var (
	// ErrInvalidRange indicates that the Range header format is invalid
	ErrInvalidRange = errors.New("invalid range header")
	// ErrUnsatisfiableRange indicates that the requested range cannot be satisfied
	ErrUnsatisfiableRange = errors.New("range not satisfiable")
	// ErrMultipleRanges indicates multiple ranges were requested (not supported)
	ErrMultipleRanges = errors.New("multiple ranges not supported")
)

// Range represents a single byte range request
type Range struct {
	Start  int64 // Start byte position (inclusive)
	End    int64 // End byte position (inclusive)
	Length int64 // Length of the range
}

// ParseRange parses an HTTP Range header according to RFC 7233.
// It supports the following formats:
//   - bytes=0-499     (first 500 bytes)
//   - bytes=500-999   (second 500 bytes)
//   - bytes=-500      (last 500 bytes)
//   - bytes=500-      (from byte 500 to end)
//
// Currently only supports single range requests.
func ParseRange(rangeHeader string, fileSize int64) (*Range, error) {
	if rangeHeader == "" {
		return nil, nil
	}

	// Range header must start with "bytes="
	const prefix = "bytes="
	if !strings.HasPrefix(rangeHeader, prefix) {
		return nil, ErrInvalidRange
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, prefix)

	// Check for multiple ranges (not supported)
	if strings.Contains(rangeSpec, ",") {
		return nil, ErrMultipleRanges
	}

	// Parse the range specification
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil, ErrInvalidRange
	}

	var start, end int64
	var err error

	// Handle different range formats
	switch {
	case parts[0] == "" && parts[1] != "":
		// Suffix range: -N means last N bytes
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return nil, ErrInvalidRange
		}
		start = fileSize - n
		if start < 0 {
			start = 0
		}
		end = fileSize - 1

	case parts[0] != "" && parts[1] == "":
		// Open-ended range: N- means from byte N to end
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return nil, ErrInvalidRange
		}
		end = fileSize - 1

	case parts[0] != "" && parts[1] != "":
		// Bounded range: N-M means bytes N through M
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return nil, ErrInvalidRange
		}
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return nil, ErrInvalidRange
		}
		// Clamp end to file size
		if end >= fileSize {
			end = fileSize - 1
		}

	default:
		return nil, ErrInvalidRange
	}

	// Validate the range
	if start >= fileSize {
		return nil, ErrUnsatisfiableRange
	}

	return &Range{
		Start:  start,
		End:    end,
		Length: end - start + 1,
	}, nil
}

// ContentRange returns the Content-Range header value for this range
func (r *Range) ContentRange(fileSize int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, fileSize)
}

// ServeContent serves the specified range of content from the reader.
// It sets appropriate headers and returns the partial content.
func ServeContent(w http.ResponseWriter, r io.ReadSeeker, rng *Range, fileSize int64, mimeType string) error {
	// Seek to the start position
	if _, err := r.Seek(rng.Start, io.SeekStart); err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	// Set response headers for partial content
	w.Header().Set("Content-Range", rng.ContentRange(fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(rng.Length, 10))
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")

	// Set 206 Partial Content status
	w.WriteHeader(http.StatusPartialContent)

	// Copy the requested range
	_, err := io.CopyN(w, r, rng.Length)
	return err
}

// ServeFullContent serves the entire content when no range is requested.
// It sets the Accept-Ranges header to indicate range support.
func ServeFullContent(w http.ResponseWriter, r io.Reader, fileSize int64, mimeType string) error {
	// Set headers for full content
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	w.Header().Set("Content-Type", mimeType)

	// Copy the entire content
	_, err := io.Copy(w, r)
	return err
}

// WriteRangeNotSatisfiable writes a 416 Range Not Satisfiable response
func WriteRangeNotSatisfiable(w http.ResponseWriter, fileSize int64) {
	w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
	w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
}
