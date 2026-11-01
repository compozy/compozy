package core

import (
	"context"
	"testing"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type memorySessionSearchStub struct {
	sessionManagerStub
	read []string
}

func (s *memorySessionSearchStub) TranscriptSearch(
	_ context.Context,
	id string,
	_ transcript.SearchQuery,
) (transcript.SearchResult, error) {
	s.read = append(s.read, id)
	return transcript.SearchResult{Matches: []transcript.SearchMatch{{Snippet: "preview"}}}, nil
}
func (s *memorySessionSearchStub) TranscriptOutline(context.Context, string) (transcript.OutlineResult, error) {
	return transcript.OutlineResult{}, nil
}

type memorySearchPolicy struct{}

func (memorySearchPolicy) Authorize(_ context.Context, req workspaceaccess.Request) (workspaceaccess.Decision, error) {
	return workspaceaccess.Decision{Allowed: req.Target == workspaceaccess.ScopeTarget("allowed", "profile")}, nil
}

var _ SessionNavigationReader = (*memorySessionSearchStub)(nil)
var _ workspaceaccess.Policy = memorySearchPolicy{}

func TestAgentMemorySessionSearch(t *testing.T) {
	t.Run("Should exclude unauthorized transcripts across catalog pages", func(t *testing.T) {
		t.Parallel()
		s := &memorySessionSearchStub{}
		s.listPage = func(_ context.Context, q session.ListQuery) (session.ListPage, error) {
			if q.ReadScope.ProfileID != "profile" || !q.AllWorkspaces {
				t.Fatalf("invalid scope: %+v", q)
			}
			if q.Cursor == "" {
				return session.ListPage{
					Sessions:   []*session.Info{{ID: "denied", WorkspaceID: "denied", ProfileID: "profile"}},
					HasMore:    true,
					NextCursor: "next",
				}, nil
			}
			return session.ListPage{
				Sessions: []*session.Info{{ID: "allowed", WorkspaceID: "allowed", ProfileID: "profile"}},
			}, nil
		}
		results, err := SearchAgentMemorySessions(
			t.Context(),
			s,
			memorySearchPolicy{},
			workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "caller"},
			"profile",
			"preview",
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.read) != 1 || s.read[0] != "allowed" {
			t.Fatalf("unauthorized reads: %v", s.read)
		}
		if len(results) != 1 || results[0].SessionID != "allowed" || results[0].Source != "session" {
			t.Fatalf("results: %+v", results)
		}
	})
}
