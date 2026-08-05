//go:build integration

package extensionpkg_test

import (
	"encoding/json"
	"errors"
	"hash"
	"io"
	"net/http"
	"testing"
)

func closeProviderIntegrationBody(t testing.TB, body io.Closer) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Errorf("close HTTP body error = %v", err)
	}
}

func readProviderIntegrationBody(t testing.TB, body io.Reader) []byte {
	t.Helper()
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Errorf("read HTTP body error = %v", err)
	}
	return payload
}

func decodeProviderIntegrationJSON(t testing.TB, body io.Reader, target any) bool {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		t.Errorf("decode HTTP JSON body error = %v", err)
		return false
	}
	return true
}

func writeProviderIntegrationJSON(t testing.TB, writer http.ResponseWriter, payload any) {
	t.Helper()
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		t.Errorf("encode HTTP JSON response error = %v", err)
	}
}

func writeProviderIntegrationResponse(t testing.TB, writer io.Writer, payload string) {
	t.Helper()
	if _, err := io.WriteString(writer, payload); err != nil {
		t.Errorf("write HTTP response error = %v", err)
	}
}

func mustWriteProviderIntegrationHash(t testing.TB, digest hash.Hash, payload []byte) {
	t.Helper()
	if _, err := digest.Write(payload); err != nil {
		t.Fatalf("hash.Write() error = %v", err)
	}
}

func mustMarshalProviderIntegrationJSON(t testing.TB, payload any) []byte {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
