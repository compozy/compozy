package spec

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestLoopOnResultSchemaShouldRequireHaltForTerminalStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		schema *openapi3.Schema
		value  any
		valid  bool
	}{
		{
			name:   "Should reject a terminal status on a non-halt node action",
			schema: loopNodeOnResultSchema(),
			value:  map[string]any{"action": "continue", "terminal_status": "blocked"},
		},
		{
			name:   "Should accept a terminal status on a node halt",
			schema: loopNodeOnResultSchema(),
			value:  map[string]any{"action": "halt", "terminal_status": "blocked"},
			valid:  true,
		},
		{
			name:   "Should accept a node halt without a terminal status",
			schema: loopNodeOnResultSchema(),
			value:  map[string]any{"action": "halt"},
			valid:  true,
		},
		{
			name:   "Should accept a bare node halt action",
			schema: loopNodeOnResultSchema(),
			value:  "halt",
			valid:  true,
		},
		{
			name:   "Should accept a node route to a forward target",
			schema: loopNodeOnResultSchema(),
			value:  map[string]any{"route": "next_node"},
			valid:  true,
		},
		{
			name:   "Should reject a terminal status on a non-halt contract action",
			schema: loopContractOnResultSchema(),
			value:  map[string]any{"action": "continue", "terminal_status": "blocked"},
		},
		{
			name:   "Should accept a terminal status on a contract halt",
			schema: loopContractOnResultSchema(),
			value:  map[string]any{"action": "halt", "terminal_status": "blocked"},
			valid:  true,
		},
		{
			name:   "Should accept a contract halt without a terminal status",
			schema: loopContractOnResultSchema(),
			value:  map[string]any{"action": "halt"},
			valid:  true,
		},
		{
			name:   "Should accept a bare contract halt action",
			schema: loopContractOnResultSchema(),
			value:  "halt",
			valid:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema := tc.schema
			if err := schema.VisitJSON(map[string]any{"fail": tc.value}); (err == nil) != tc.valid {
				t.Fatalf("VisitJSON(%v) error = %v, want valid=%v", tc.value, err, tc.valid)
			}
		})
	}
}
