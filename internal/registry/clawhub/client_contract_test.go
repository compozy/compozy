package clawhub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/registry"
)

func TestClientBehaviorContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should classify missing skill responses as registry package not found", func(t *testing.T) {
		t.Parallel()

		server := newContractServer(t, func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/api/v1/skills/@agh/missing" {
				t.Fatalf("request.URL.Path = %q, want missing skill path", request.URL.Path)
			}
			if request.URL.RawPath != "/api/v1/skills/@agh%2Fmissing" {
				t.Fatalf("request.URL.RawPath = %q, want one escaped missing skill segment", request.URL.RawPath)
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			writeContractResponse(t, writer, "{\"error\":\"missing skill\"}")
		})

		_, err := NewClient(server.URL).Info(context.Background(), "@agh/missing")
		if !errors.Is(err, registry.ErrPackageNotFound) {
			t.Fatalf("Info(missing) error = %v, want registry.ErrPackageNotFound", err)
		}
	})

	t.Run("Should reject oversized search responses before decoding", func(t *testing.T) {
		t.Parallel()

		server := newContractServer(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writeContractResponse(t, writer, "["+strings.Repeat(" ", int(maxJSONResponseBytes))+"]")
		})

		_, err := NewClient(server.URL).Search(context.Background(), "agent", registry.SearchOpts{})
		if !errors.Is(err, errResponseTooLarge) {
			t.Fatalf("Search(oversized) error = %v, want errResponseTooLarge", err)
		}
	})

	t.Run("Should reject oversized info responses before decoding", func(t *testing.T) {
		t.Parallel()

		server := newContractServer(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			body := fmt.Sprintf(
				"{\"slug\":\"@agh/review\",\"name\":%q}",
				strings.Repeat("a", int(maxJSONResponseBytes)),
			)
			writeContractResponse(t, writer, body)
		})

		_, err := NewClient(server.URL).Info(context.Background(), "@agh/review")
		if !errors.Is(err, errResponseTooLarge) {
			t.Fatalf("Info(oversized) error = %v, want errResponseTooLarge", err)
		}
	})

	t.Run("Should reject oversized downloads before returning a spooled archive", func(t *testing.T) {
		t.Parallel()

		const limit int64 = 64
		body := strings.Repeat("a", int(limit)+1024)
		server := newContractServer(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/gzip")
			writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
			writeContractResponse(t, writer, body)
		})

		result, err := NewClient(server.URL).Download(
			context.Background(),
			"@agh/review",
			registry.DownloadOpts{MaxArchiveSize: limit},
		)
		if result != nil && result.Reader != nil {
			t.Cleanup(func() {
				if closeErr := result.Reader.Close(); closeErr != nil {
					t.Errorf("result.Reader.Close() error = %v", closeErr)
				}
			})
		}
		if !errors.Is(err, registry.ErrArchiveTooLargeCompressed) {
			t.Fatalf("Download(oversized) error = %v, want registry.ErrArchiveTooLargeCompressed", err)
		}
	})

	t.Run("Should stop spooling downloads after the compressed limit is crossed", func(t *testing.T) {
		t.Parallel()

		const limit int64 = 64
		reader := &countingContractReader{
			reader: strings.NewReader(strings.Repeat("a", int(limit)+1024)),
		}

		result, _, err := spoolDownloadResponse(reader, "@agh/review", limit)
		if result != nil {
			t.Cleanup(func() {
				if closeErr := result.Close(); closeErr != nil {
					t.Errorf("result.Close() error = %v", closeErr)
				}
			})
		}
		if !errors.Is(err, registry.ErrArchiveTooLargeCompressed) {
			t.Fatalf("spoolDownloadResponse(oversized) error = %v, want registry.ErrArchiveTooLargeCompressed", err)
		}
		if got, want := reader.readBytes, limit+1; got != want {
			t.Fatalf("spoolDownloadResponse read bytes = %d, want %d", got, want)
		}
	})
}

func newContractServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func writeContractResponse(t *testing.T, writer io.Writer, body string) {
	t.Helper()

	if _, err := io.WriteString(writer, body); err != nil {
		t.Fatalf("write response body error = %v", err)
	}
}

type countingContractReader struct {
	reader    *strings.Reader
	readBytes int64
}

func (r *countingContractReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.readBytes += int64(n)
	return n, err
}
