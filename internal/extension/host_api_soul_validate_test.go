package extensionpkg

import (
	"context"
	"testing"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/soul"
)

func TestHostAPIHandlerSoulValidateBodyPresenceContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an omitted soul validate body as current-file validation", func(t *testing.T) {
		t.Parallel()

		env, authoring := newSoulValidateBodyPresenceEnvContract(t)
		_, err := env.handler.Handle(
			t.Context(),
			"ext-soul-read",
			string(extensioncontract.HostAPIMethodAgentsSoulValidate),
			mustHostAPIAuthoredJSON(t, map[string]any{
				"workspace_id": env.workspaceID,
				"agent_name":   "coder",
			}),
		)
		if err != nil {
			t.Fatalf("Handle(agents/soul/validate omitted body) error = %v", err)
		}
		if authoring.validateCalls != 1 {
			t.Fatalf("soul validate calls = %d, want 1", authoring.validateCalls)
		}
		if authoring.lastValidate.Body != nil {
			t.Fatalf("soul validate body = %q, want nil for omitted body", *authoring.lastValidate.Body)
		}
	})

	t.Run("Should preserve an explicit empty soul validate body as proposed content", func(t *testing.T) {
		t.Parallel()

		env, authoring := newSoulValidateBodyPresenceEnvContract(t)
		_, err := env.handler.Handle(
			t.Context(),
			"ext-soul-read",
			string(extensioncontract.HostAPIMethodAgentsSoulValidate),
			mustHostAPIAuthoredJSON(t, map[string]any{
				"workspace_id": env.workspaceID,
				"agent_name":   "coder",
				"body":         "",
			}),
		)
		if err != nil {
			t.Fatalf("Handle(agents/soul/validate empty body) error = %v", err)
		}
		if authoring.validateCalls != 1 {
			t.Fatalf("soul validate calls = %d, want 1", authoring.validateCalls)
		}
		if authoring.lastValidate.Body == nil {
			t.Fatal("soul validate body = nil, want explicit empty string pointer")
		}
		if *authoring.lastValidate.Body != "" {
			t.Fatalf("soul validate body = %q, want empty string", *authoring.lastValidate.Body)
		}
	})
}

func newSoulValidateBodyPresenceEnvContract(
	t *testing.T,
) (*hostAPITestEnv, *recordingSoulAuthoringContract) {
	t.Helper()

	env := newHostAPITestEnv(t)
	authoring := &recordingSoulAuthoringContract{
		result: hostAPITestSoulMutationResult(env.workspaceID, "coder", env.workspace.RootDir),
	}
	env.handler.soulAuthoring = authoring
	env.grant("ext-soul-read", []string{
		string(extensioncontract.HostAPIMethodAgentsSoulValidate),
	}, []string{"soul.read"})
	return env, authoring
}

type recordingSoulAuthoringContract struct {
	result        soul.MutationResult
	validateCalls int
	lastValidate  soul.ValidateRequest
}

func (s *recordingSoulAuthoringContract) Validate(
	_ context.Context,
	req soul.ValidateRequest,
) (soul.ValidateResult, error) {
	s.validateCalls++
	s.lastValidate = req
	return soul.ValidateResult{Soul: s.result.Soul}, nil
}

func (s *recordingSoulAuthoringContract) Put(context.Context, soul.PutRequest) (soul.MutationResult, error) {
	return s.result, nil
}

func (s *recordingSoulAuthoringContract) Delete(context.Context, soul.DeleteRequest) (soul.MutationResult, error) {
	return s.result, nil
}

func (s *recordingSoulAuthoringContract) History(context.Context, soul.HistoryRequest) (soul.HistoryResult, error) {
	return soul.HistoryResult{Revisions: []soul.Revision{s.result.Revision}}, nil
}

func (s *recordingSoulAuthoringContract) Rollback(
	context.Context,
	soul.RollbackRequest,
) (soul.MutationResult, error) {
	return s.result, nil
}

var _ hostAPISoulAuthoringService = (*recordingSoulAuthoringContract)(nil)
