package bridgesdk

import (
	"testing"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestSessionInitializeRequestClonesHandshakeSlicesContract(t *testing.T) {
	t.Parallel()

	t.Run("Should clone supported protocol and granted resource slices", func(t *testing.T) {
		t.Parallel()

		session := &Session{
			request: subprocess.InitializeRequest{
				SupportedProtocolVersion: []string{"1"},
				Capabilities: subprocess.InitializeCapabilities{
					Provides: []string{"bridge.adapter"},
					GrantedActions: []extensionprotocol.HostAPIMethod{
						extensionprotocol.HostAPIMethodBridgesInstancesList,
					},
					GrantedSecurity:       []string{"bridge.read"},
					GrantedResourceKinds:  []string{"agent"},
					GrantedResourceScopes: []string{"workspace"},
				},
			},
		}

		request := session.InitializeRequest()
		request.SupportedProtocolVersion[0] = "mutated"
		request.Capabilities.GrantedResourceKinds[0] = "mutated"
		request.Capabilities.GrantedResourceScopes[0] = "mutated"

		again := session.InitializeRequest()
		if got, want := again.SupportedProtocolVersion[0], "1"; got != want {
			t.Fatalf("SupportedProtocolVersion[0] = %q, want %q", got, want)
		}
		if got, want := again.Capabilities.GrantedResourceKinds[0], "agent"; got != want {
			t.Fatalf("GrantedResourceKinds[0] = %q, want %q", got, want)
		}
		if got, want := again.Capabilities.GrantedResourceScopes[0], "workspace"; got != want {
			t.Fatalf("GrantedResourceScopes[0] = %q, want %q", got, want)
		}
	})
}
