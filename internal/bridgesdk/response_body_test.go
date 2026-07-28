// Suite: provider HTTP response cleanup
// Invariant: every response body is drained and closed, preserving simultaneous cleanup failures.
// Boundary IN: provider-owned HTTP response bodies after transport completion.
// Boundary OUT: provider request construction, status classification, and retry policy.
package bridgesdk

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestDecodeSingleJSONValueRejectsTrailingContent(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "Should accept one JSON value followed by whitespace", body: `{"id":"msg-1"}  `},
		{name: "Should reject a second JSON value", body: `{"id":"msg-1"}{"id":"msg-2"}`, wantErr: true},
		{name: "Should reject a valid JSON prefix followed by invalid tokens", body: `{"id":"msg-1"}garbage`, wantErr: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var decoded map[string]string
			err := DecodeSingleJSONValue(strings.NewReader(testCase.body), &decoded)
			if testCase.wantErr && err == nil {
				t.Fatal("DecodeSingleJSONValue() error = nil, want non-nil")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("DecodeSingleJSONValue() error = %v, want nil", err)
			}
		})
	}
}

func TestReadHTTPResponseBodyPreservesPartialContentAndFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should return bytes read before the provider body fails", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("provider body read failed")
		reader := &partialHTTPResponseReader{
			content: []byte("partial provider error"),
			readErr: readErr,
		}
		body, err := ReadHTTPResponseBody(reader)
		if got, want := body, "partial provider error"; got != want {
			t.Fatalf("ReadHTTPResponseBody() body = %q, want %q", got, want)
		}
		if !errors.Is(err, readErr) {
			t.Fatalf("ReadHTTPResponseBody() error = %v, want read failure", err)
		}
	})
}

func TestDrainAndCloseHTTPResponseBodyPreservesCleanupFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should drain unread bytes before closing the response body", func(t *testing.T) {
		t.Parallel()

		body := &responseBodyProbe{reader: strings.NewReader("provider response")}
		if err := DrainAndCloseHTTPResponseBody(body); err != nil {
			t.Fatalf("DrainAndCloseHTTPResponseBody() error = %v", err)
		}
		if got, want := body.bytesRead, len("provider response"); got != want {
			t.Fatalf("bytes read = %d, want %d", got, want)
		}
		if !body.closed {
			t.Fatal("response body closed = false, want true")
		}
	})

	t.Run("Should join drain and close failures", func(t *testing.T) {
		t.Parallel()

		drainErr := errors.New("drain failed")
		closeErr := errors.New("close failed")
		body := &responseBodyProbe{
			reader:   iotest.ErrReader(drainErr),
			closeErr: closeErr,
		}
		err := DrainAndCloseHTTPResponseBody(body)
		if !errors.Is(err, drainErr) {
			t.Fatalf("DrainAndCloseHTTPResponseBody() error = %v, want drain failure", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("DrainAndCloseHTTPResponseBody() error = %v, want close failure", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false after drain failure, want true")
		}
	})

	t.Run("Should accept an absent response body", func(t *testing.T) {
		t.Parallel()

		if err := DrainAndCloseHTTPResponseBody(nil); err != nil {
			t.Fatalf("DrainAndCloseHTTPResponseBody(nil) error = %v", err)
		}
	})

	t.Run("Should preserve operation and cleanup failures before commit", func(t *testing.T) {
		t.Parallel()

		operationErr := errors.New("decode failed")
		closeErr := errors.New("close failed")
		reports := 0
		err := FinalizeHTTPResponseBody(
			&responseBodyProbe{reader: strings.NewReader(""), closeErr: closeErr},
			operationErr,
			HTTPResponseCommitUnconfirmed,
			func(error) { reports++ },
		)
		if !errors.Is(err, operationErr) || !errors.Is(err, closeErr) {
			t.Fatalf("FinalizeHTTPResponseBody() error = %v, want operation and cleanup failures", err)
		}
		if reports != 0 {
			t.Fatalf("cleanup reports = %d, want 0", reports)
		}
	})

	t.Run("Should preserve cleanup without commit evidence", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close failed")
		reports := 0
		err := FinalizeHTTPResponseBody(
			&responseBodyProbe{reader: strings.NewReader(""), closeErr: closeErr},
			nil,
			HTTPResponseCommitUnconfirmed,
			func(error) { reports++ },
		)
		if !errors.Is(err, closeErr) {
			t.Fatalf("FinalizeHTTPResponseBody() error = %v, want cleanup failure", err)
		}
		if reports != 0 {
			t.Fatalf("cleanup reports = %d, want 0", reports)
		}
	})

	t.Run("Should retain committed cleanup failure when no reporter is available", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close failed")
		err := FinalizeHTTPResponseBody(
			&responseBodyProbe{reader: strings.NewReader(""), closeErr: closeErr},
			nil,
			HTTPResponseCommittedBySuccessStatus,
			nil,
		)
		if !errors.Is(err, closeErr) {
			t.Fatalf("FinalizeHTTPResponseBody() error = %v, want cleanup failure", err)
		}
		var committed *CommittedMutationError
		if !errors.As(err, &committed) {
			t.Fatalf("FinalizeHTTPResponseBody() error = %T, want CommittedMutationError", err)
		}
	})

	t.Run("Should mark a post-commit decode failure as unsafe to retry", func(t *testing.T) {
		t.Parallel()

		decodeErr := errors.New("decode failed")
		closeErr := errors.New("close failed")
		err := FinalizeHTTPResponseBody(
			&responseBodyProbe{reader: strings.NewReader(""), closeErr: closeErr},
			decodeErr,
			HTTPResponseCommittedBySuccessStatus,
			func(error) { t.Fatal("cleanup reporter called for a failed operation") },
		)
		var committed *CommittedMutationError
		if !errors.As(err, &committed) {
			t.Fatalf("FinalizeHTTPResponseBody() error = %T, want CommittedMutationError", err)
		}
		if !errors.Is(err, decodeErr) || !errors.Is(err, closeErr) {
			t.Fatalf("FinalizeHTTPResponseBody() error = %v, want decode and cleanup failures", err)
		}
	})
}

type responseBodyProbe struct {
	reader    io.Reader
	closeErr  error
	bytesRead int
	closed    bool
}

type partialHTTPResponseReader struct {
	content []byte
	readErr error
	read    bool
}

func (r *partialHTTPResponseReader) Read(buffer []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	return copy(buffer, r.content), r.readErr
}

func (b *responseBodyProbe) Read(buffer []byte) (int, error) {
	read, err := b.reader.Read(buffer)
	b.bytesRead += read
	return read, err
}

func (b *responseBodyProbe) Close() error {
	b.closed = true
	return b.closeErr
}
