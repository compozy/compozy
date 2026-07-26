package httpapi

import (
	"reflect"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
)

func TestPromptRequestPayloadRemainsTransportLocal(t *testing.T) {
	t.Parallel()

	t.Run("Should keep prompt requests local while using the shared core encoder", func(t *testing.T) {
		t.Parallel()

		promptPkg := reflect.TypeFor[promptRequest]().PkgPath()
		encoderPkg := reflect.TypeFor[core.PromptStreamEncoder]().PkgPath()
		sharedPkg := reflect.TypeFor[contract.AgentEventPayload]().PkgPath()

		if promptPkg == encoderPkg {
			t.Fatalf("prompt request package unexpectedly matches shared encoder package %q", encoderPkg)
		}
		if encoderPkg == sharedPkg {
			t.Fatalf("shared encoder package unexpectedly matches shared contract package %q", sharedPkg)
		}

		sharedTimestamp, ok := reflect.TypeFor[contract.AgentEventPayload]().FieldByName("Timestamp")
		if !ok {
			t.Fatal("contract.AgentEventPayload.Timestamp field is missing")
		}
		if sharedTimestamp.Type != reflect.TypeFor[time.Time]() {
			t.Fatalf("shared timestamp type = %v, want time.Time", sharedTimestamp.Type)
		}
	})
}
