package daytona

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/sandbox"
)

func TestDaytonaProviderSyncFromRuntimeExtractionErrorClosesRemoteProducerContract(t *testing.T) {
	t.Parallel()

	t.Run("Should stop remote archive session when extraction rejects unsafe entry", func(t *testing.T) {
		t.Parallel()

		transport := &blockingArchiveTransport{}
		provider := newTestProviderWithTransport(transport)
		state := newProviderSessionState(t, t.TempDir(), nil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resultCh := make(chan syncFromRuntimeResult, 1)
		go func() {
			result, err := provider.SyncFromRuntime(ctx, state, sandbox.SyncOptions{
				Reason: sandbox.SyncReasonStop,
			})
			resultCh <- syncFromRuntimeResult{result: result, err: err}
		}()

		select {
		case got := <-resultCh:
			if got.err == nil {
				t.Fatal("SyncFromRuntime() error = nil, want unsafe tar path error")
			}
			if !errors.Is(got.err, errUnsafeTarPath) {
				t.Fatalf("SyncFromRuntime() error = %v, want unsafe tar path error", got.err)
			}
			if len(got.result.Errors) != 1 {
				t.Fatalf("SyncFromRuntime() result errors = %v, want one extraction error", got.result.Errors)
			}
		case <-time.After(500 * time.Millisecond):
			transport.stopSession(t)
			t.Fatal("SyncFromRuntime() did not return after unsafe tar extraction error")
		}

		session := transport.session
		if session == nil {
			t.Fatal("transport session = nil")
		}
		if !session.closed.Load() {
			t.Fatal("transport session was not closed after unsafe tar extraction error")
		}
		if err := session.writerError(); err != nil {
			t.Fatalf("archive writer error = %v", err)
		}
	})
}

type syncFromRuntimeResult struct {
	result sandbox.SyncResult
	err    error
}

type blockingArchiveTransport struct {
	session *blockingArchiveSession
}

func (t *blockingArchiveTransport) Dial(
	_ context.Context,
	_ sandboxInfo,
	_ string,
) (transportSession, error) {
	session := newBlockingArchiveSession()
	t.session = session
	return session, nil
}

func (t *blockingArchiveTransport) stopSession(tb testing.TB) {
	tb.Helper()
	if t.session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := t.session.Stop(ctx); err != nil {
		tb.Logf("Stop() after timeout error = %v", err)
	}
}

type blockingArchiveSession struct {
	reader   *io.PipeReader
	done     chan struct{}
	closed   atomic.Bool
	errMu    sync.Mutex
	writeErr error
}

func newBlockingArchiveSession() *blockingArchiveSession {
	reader, writer := io.Pipe()
	session := &blockingArchiveSession{
		reader: reader,
		done:   make(chan struct{}),
	}
	go session.writeUnsafeArchive(writer)
	return session
}

func (s *blockingArchiveSession) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *blockingArchiveSession) Write(_ []byte) (int, error) {
	return 0, errors.New("sandbox/daytona: unexpected write to runtime archive session")
}

func (s *blockingArchiveSession) Close() error {
	s.closed.Store(true)
	return s.reader.CloseWithError(io.ErrClosedPipe)
}

func (s *blockingArchiveSession) CloseWrite() error {
	return nil
}

func (s *blockingArchiveSession) Done() <-chan struct{} {
	return s.done
}

func (s *blockingArchiveSession) Wait() error {
	<-s.done
	return nil
}

func (s *blockingArchiveSession) Stop(ctx context.Context) error {
	if err := s.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return err
	}
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("sandbox/daytona: stop blocking archive session: %w", ctx.Err())
	}
}

func (s *blockingArchiveSession) Stderr() string {
	return ""
}

func (s *blockingArchiveSession) writeUnsafeArchive(writer *io.PipeWriter) {
	defer close(s.done)
	tarWriter := tar.NewWriter(writer)
	writeErr := tarWriter.WriteHeader(&tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "../outside",
	})
	if writeErr == nil {
		writeErr = tarWriter.WriteHeader(&tar.Header{
			Name: "large.bin",
			Mode: 0o600,
			Size: 1 << 20,
		})
	}
	if writeErr == nil {
		_, writeErr = io.CopyN(tarWriter, zeroReader{}, 1<<20)
	}
	if closeErr := tarWriter.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	s.finishWriter(writer, writeErr)
}

func (s *blockingArchiveSession) finishWriter(writer *io.PipeWriter, writeErr error) {
	var closeErr error
	if writeErr != nil {
		closeErr = writer.CloseWithError(writeErr)
	} else {
		closeErr = writer.Close()
	}
	s.recordWriterError(writeErr)
	s.recordWriterError(closeErr)
}

func (s *blockingArchiveSession) recordWriterError(err error) {
	if err == nil || errors.Is(err, io.ErrClosedPipe) {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	s.writeErr = errors.Join(s.writeErr, err)
}

func (s *blockingArchiveSession) writerError() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.writeErr
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
