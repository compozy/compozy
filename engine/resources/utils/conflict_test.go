package resourceutil

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConflictErrorDefaultMessage(t *testing.T) {
	t.Run("Should return default message when details empty", func(t *testing.T) {
		err := ConflictError{Details: []ReferenceDetail{}}
		assert.Equal(t, "resource has conflicting references", err.Error())
	})
}

func TestConflictErrorReferenceCounting(t *testing.T) {
	cases := []struct {
		name    string
		details []ReferenceDetail
		expect  string
	}{
		{
			name:    "Should use singular when only one reference",
			details: []ReferenceDetail{{Resource: "agents", IDs: []string{"ag1"}}},
			expect:  "resource referenced by 1 collection: agents",
		},
		{
			name: "Should count total references across resources",
			details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
				{Resource: "agents", IDs: []string{"ag1"}},
			},
			expect: "resource referenced by 3 collections: agents, workflows",
		},
		{
			name:    "Should handle blank resources using total references",
			details: []ReferenceDetail{{Resource: " ", IDs: []string{"id1", "id2"}}},
			expect:  "resource referenced by 2 collections",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ConflictError{Details: tc.details}.Error()
			assert.Equal(t, tc.expect, msg)
		})
	}
}

func TestConflictErrorFormatsResourceTypes(t *testing.T) {
	t.Run("Should deduplicate and sort resource types", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1"}},
				{Resource: "agents", IDs: []string{"ag1"}},
				{Resource: "workflows", IDs: []string{"wf2"}},
			},
		}
		msg := err.Error()
		assert.Equal(t, "resource referenced by 3 collections: agents, workflows", msg)
	})
}

func TestBuildConflictProblemIncludesReferences(t *testing.T) {
	t.Run("Should embed reference metadata", func(t *testing.T) {
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
			{Resource: "agents", IDs: []string{"ag1"}},
		}
		conflictErr := ConflictError{Details: details}
		problem := BuildConflictProblem(conflictErr, details)
		require.Equal(t, http.StatusConflict, problem.Status)
		require.NotNil(t, problem.Extras)
		refs, ok := problem.Extras["references"].([]map[string]any)
		if !ok {
			// The extras are serialized as []any when marshaled; in Go they remain []map[string]any.
			slice, okAny := problem.Extras["references"].([]any)
			require.True(t, okAny)
			require.Len(t, slice, len(details))
			return
		}
		require.Len(t, refs, len(details))
	})
}

func TestBuildConflictProblemDefaultDetail(t *testing.T) {
	t.Run("Should fall back to default message when error empty", func(t *testing.T) {
		problem := BuildConflictProblem(errors.New("   "), nil)
		assert.Equal(t, http.StatusConflict, problem.Status)
		assert.Equal(t, "resource has active references", problem.Detail)
		assert.Nil(t, problem.Extras)
	})
}

func TestBuildConflictProblemPreservesErrorMessage(t *testing.T) {
	t.Run("Should propagate provided error detail", func(t *testing.T) {
		err := errors.New("resource in use by workflows")
		problem := BuildConflictProblem(err, nil)
		assert.Equal(t, "resource in use by workflows", problem.Detail)
	})
}
