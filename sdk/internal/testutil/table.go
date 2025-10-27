package testutil

import (
	"context"
	"strings"
	"testing"
)

// TableTest describes a table-driven builder test case executed via RunTableTests.
type TableTest struct {
	Name        string
	BuildFunc   func(context.Context) (any, error)
	WantErr     bool
	ErrContains string
	Validate    func(*testing.T, any)
}

// RunTableTests executes each table-driven test with a consistent test context.
func RunTableTests(t *testing.T, tests []TableTest) {
	t.Helper()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			ctx := NewTestContext(t)
			result, err := tc.BuildFunc(ctx)
			if tc.WantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if tc.ErrContains != "" && !strings.Contains(err.Error(), tc.ErrContains) {
					t.Fatalf("expected error containing %q, got %v", tc.ErrContains, err)
				}
				return
			}
			RequireNoError(t, err)
			if tc.Validate != nil {
				tc.Validate(t, result)
			}
		})
	}
}
