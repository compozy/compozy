package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed int
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (r *flushRecorder) Flush() {
	r.flushed++
}

func TestStartSSESetsHeaders(t *testing.T) {
	recorder := newFlushRecorder()
	stream := StartSSE(recorder)
	require.NotNil(t, stream)
	result := recorder.Result()
	require.Equal(t, sseContentType, result.Header.Get("Content-Type"))
	require.Equal(t, sseCacheControl, result.Header.Get("Cache-Control"))
	require.Equal(t, sseConnection, result.Header.Get("Connection"))
	require.Equal(t, sseAccelBuffering, result.Header.Get("X-Accel-Buffering"))
}

func TestLastEventID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stream", http.NoBody)
	req.Header.Set("Last-Event-ID", "42")
	id, ok, err := LastEventID(req)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(42), id)
	noHeaderID, present, err := LastEventID(httptest.NewRequest(http.MethodGet, "/stream", http.NoBody))
	require.NoError(t, err)
	require.False(t, present)
	require.Zero(t, noHeaderID)
	req.Header.Set("Last-Event-ID", "invalid")
	_, _, err = LastEventID(req)
	require.Error(t, err)
}

func TestWriteEventFormatsPayload(t *testing.T) {
	recorder := newFlushRecorder()
	stream := StartSSE(recorder)
	require.NotNil(t, stream)
	data := []byte("{\"status\":\"RUNNING\"}")
	require.NoError(t, stream.WriteEvent(7, "workflow_status", data))
	body := recorder.Body.String()
	require.Equal(t, "id: 7\nevent: workflow_status\ndata: {\"status\":\"RUNNING\"}\n\n", body)
	require.Positive(t, recorder.flushed)
}

func TestWriteEventHandlesMultilineData(t *testing.T) {
	recorder := newFlushRecorder()
	stream := StartSSE(recorder)
	data := []byte("line1\nline2")
	require.NoError(t, stream.WriteEvent(9, "multi", data))
	require.Equal(t, "id: 9\nevent: multi\ndata: line1\ndata: line2\n\n", recorder.Body.String())
}

func TestWriteHeartbeat(t *testing.T) {
	recorder := newFlushRecorder()
	stream := StartSSE(recorder)
	require.NoError(t, stream.WriteHeartbeat())
	require.Equal(t, heartbeatFrameBody, recorder.Body.String())
	require.Equal(t, 1, recorder.flushed)
}
