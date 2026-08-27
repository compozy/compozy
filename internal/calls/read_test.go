package calls

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/config"
)

// Suite: call result authorization
// Invariant: a result is readable only inside its owner scope and by its parent, bound child, or operator.
// Boundary IN: Service.Result.
// Boundary OUT: native tool dispatch is owned by the daemon native-tool suite.
func TestServiceResultAuthorization(t *testing.T) {
	t.Parallel()

	service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
	record := createContractedCall(t, service)
	settled, err := service.Return(t.Context(), ReturnInput{
		Scope: record.OwnerScope(), CallID: record.CallID, Result: json.RawMessage(`{"answer":42}`),
		Actor: SettlementActor{Kind: actorKindAgentSession, ID: record.ChildSessionID},
	})
	if err != nil {
		t.Fatalf("Return() error = %v", err)
	}
	base := CallReadQuery{
		ReadScope: ReadScope{ProfileID: settled.Call.ProfileID},
		Scope:     settled.Call.Scope, WorkspaceID: settled.Call.WorkspaceID,
	}

	for _, actor := range []Actor{
		{Kind: actorKindAgentSession, ID: settled.Call.ParentSessionID},
		{Kind: actorKindAgentSession, ID: settled.Call.ChildSessionID},
		{Kind: "human", ID: "operator:test"},
	} {
		query := base
		query.Actor = actor
		result, resultErr := service.Result(t.Context(), query, settled.Call.CallID)
		if resultErr != nil || string(result.Bytes) != `{"answer":42}` {
			t.Fatalf("Result(%#v) = %s, %v", actor, result.Bytes, resultErr)
		}
	}

	denied := base
	denied.Actor = Actor{Kind: actorKindAgentSession, ID: "ses-sibling"}
	if _, err := service.Result(t.Context(), denied, settled.Call.CallID); !IsCode(err, CodeTargetDenied) {
		t.Fatalf("Result(sibling) error = %v, want %s", err, CodeTargetDenied)
	}

	otherWorkspace := base
	otherWorkspace.WorkspaceID = "ws-other"
	otherWorkspace.Actor = Actor{Kind: actorKindAgentSession, ID: settled.Call.ParentSessionID}
	if _, err := service.Result(t.Context(), otherWorkspace, settled.Call.CallID); !IsCode(err, CodeNotFound) {
		t.Fatalf("Result(other workspace) error = %v, want %s", err, CodeNotFound)
	}
	if database.calls[settled.Call.CallID].State != StateCompleted {
		t.Fatalf("stored call changed during denied reads: %#v", database.calls[settled.Call.CallID])
	}
}
