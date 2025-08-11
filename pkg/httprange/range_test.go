package httprange

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		name        string
		rangeHeader string
		fileSize    int64
		want        *Range
		wantErr     error
	}{
		{
			name:        "empty range header",
			rangeHeader: "",
			fileSize:    1000,
			want:        nil,
			wantErr:     nil,
		},
		{
			name:        "bounded range",
			rangeHeader: "bytes=0-499",
			fileSize:    1000,
			want: &Range{
				Start:  0,
				End:    499,
				Length: 500,
			},
			wantErr: nil,
		},
		{
			name:        "open-ended range",
			rangeHeader: "bytes=500-",
			fileSize:    1000,
			want: &Range{
				Start:  500,
				End:    999,
				Length: 500,
			},
			wantErr: nil,
		},
		{
			name:        "suffix range",
			rangeHeader: "bytes=-500",
			fileSize:    1000,
			want: &Range{
				Start:  500,
				End:    999,
				Length: 500,
			},
			wantErr: nil,
		},
		{
			name:        "suffix range larger than file",
			rangeHeader: "bytes=-2000",
			fileSize:    1000,
			want: &Range{
				Start:  0,
				End:    999,
				Length: 1000,
			},
			wantErr: nil,
		},
		{
			name:        "end clamped to file size",
			rangeHeader: "bytes=500-2000",
			fileSize:    1000,
			want: &Range{
				Start:  500,
				End:    999,
				Length: 500,
			},
			wantErr: nil,
		},
		{
			name:        "invalid prefix",
			rangeHeader: "chunks=0-499",
			fileSize:    1000,
			want:        nil,
			wantErr:     ErrInvalidRange,
		},
		{
			name:        "multiple ranges not supported",
			rangeHeader: "bytes=0-499,500-999",
			fileSize:    1000,
			want:        nil,
			wantErr:     ErrMultipleRanges,
		},
		{
			name:        "invalid range format",
			rangeHeader: "bytes=invalid",
			fileSize:    1000,
			want:        nil,
			wantErr:     ErrInvalidRange,
		},
		{
			name:        "start after end",
			rangeHeader: "bytes=500-100",
			fileSize:    1000,
			want:        nil,
			wantErr:     ErrInvalidRange,
		},
		{
			name:        "start beyond file size",
			rangeHeader: "bytes=1500-2000",
			fileSize:    1000,
			want:        nil,
			wantErr:     ErrUnsatisfiableRange,
		},
		{
			name:        "negative start",
			rangeHeader: "bytes=-500-999",
			fileSize:    1000,
			want:        nil,
			wantErr:     ErrInvalidRange,
		},
		{
			name:        "single byte range",
			rangeHeader: "bytes=0-0",
			fileSize:    1000,
			want: &Range{
				Start:  0,
				End:    0,
				Length: 1,
			},
			wantErr: nil,
		},
		{
			name:        "last byte",
			rangeHeader: "bytes=999-999",
			fileSize:    1000,
			want: &Range{
				Start:  999,
				End:    999,
				Length: 1,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRange(tt.rangeHeader, tt.fileSize)

			if err != tt.wantErr {
				t.Errorf("ParseRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want == nil {
				if got != nil {
					t.Errorf("ParseRange() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("ParseRange() = nil, want %v", tt.want)
				return
			}

			if got.Start != tt.want.Start || got.End != tt.want.End || got.Length != tt.want.Length {
				t.Errorf("ParseRange() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestContentRange(t *testing.T) {
	tests := []struct {
		name     string
		rng      *Range
		fileSize int64
		want     string
	}{
		{
			name: "first 500 bytes",
			rng: &Range{
				Start:  0,
				End:    499,
				Length: 500,
			},
			fileSize: 1000,
			want:     "bytes 0-499/1000",
		},
		{
			name: "last 500 bytes",
			rng: &Range{
				Start:  500,
				End:    999,
				Length: 500,
			},
			fileSize: 1000,
			want:     "bytes 500-999/1000",
		},
		{
			name: "single byte",
			rng: &Range{
				Start:  42,
				End:    42,
				Length: 1,
			},
			fileSize: 1000,
			want:     "bytes 42-42/1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rng.ContentRange(tt.fileSize)
			if got != tt.want {
				t.Errorf("ContentRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServeContent(t *testing.T) {
	content := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	fileSize := int64(len(content))

	tests := []struct {
		name           string
		rng            *Range
		expectedStatus int
		expectedBody   string
		expectedRange  string
	}{
		{
			name: "first 10 bytes",
			rng: &Range{
				Start:  0,
				End:    9,
				Length: 10,
			},
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "0123456789",
			expectedRange:  "bytes 0-9/36",
		},
		{
			name: "middle 10 bytes",
			rng: &Range{
				Start:  10,
				End:    19,
				Length: 10,
			},
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "abcdefghij",
			expectedRange:  "bytes 10-19/36",
		},
		{
			name: "last 10 bytes",
			rng: &Range{
				Start:  26,
				End:    35,
				Length: 10,
			},
			expectedStatus: http.StatusPartialContent,
			expectedBody:   "qrstuvwxyz",
			expectedRange:  "bytes 26-35/36",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(content)
			recorder := httptest.NewRecorder()

			err := ServeContent(recorder, reader, tt.rng, fileSize, "text/plain")
			if err != nil {
				t.Fatalf("ServeContent() error = %v", err)
			}

			resp := recorder.Result()
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", resp.StatusCode, tt.expectedStatus)
			}

			// Check Content-Range header
			contentRange := resp.Header.Get("Content-Range")
			if contentRange != tt.expectedRange {
				t.Errorf("Content-Range = %s, want %s", contentRange, tt.expectedRange)
			}

			// Check Accept-Ranges header
			acceptRanges := resp.Header.Get("Accept-Ranges")
			if acceptRanges != "bytes" {
				t.Errorf("Accept-Ranges = %s, want bytes", acceptRanges)
			}

			// Check body content
			body, _ := io.ReadAll(resp.Body)
			if string(body) != tt.expectedBody {
				t.Errorf("Body = %s, want %s", string(body), tt.expectedBody)
			}
		})
	}
}

func TestServeFullContent(t *testing.T) {
	content := []byte("Full content of the file")
	fileSize := int64(len(content))

	reader := bytes.NewReader(content)
	recorder := httptest.NewRecorder()

	err := ServeFullContent(recorder, reader, fileSize, "text/plain")
	if err != nil {
		t.Fatalf("ServeFullContent() error = %v", err)
	}

	resp := recorder.Result()
	defer resp.Body.Close()

	// Should return 200 OK (not 206)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Should have Accept-Ranges header
	acceptRanges := resp.Header.Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		t.Errorf("Accept-Ranges = %s, want bytes", acceptRanges)
	}

	// Should have Content-Length
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "24" {
		t.Errorf("Content-Length = %s, want 24", contentLength)
	}

	// Check body
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(content) {
		t.Errorf("Body = %s, want %s", string(body), string(content))
	}
}

func TestWriteRangeNotSatisfiable(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteRangeNotSatisfiable(recorder, 1000)

	resp := recorder.Result()
	defer resp.Body.Close()

	// Should return 416
	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusRequestedRangeNotSatisfiable)
	}

	// Should have Content-Range header with */fileSize
	contentRange := resp.Header.Get("Content-Range")
	if contentRange != "bytes */1000" {
		t.Errorf("Content-Range = %s, want bytes */1000", contentRange)
	}
}
