package config

import (
	"strings"
	"testing"
)

func TestParseAgentDefCategoryPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantPath []string
		wantErr  string
	}{
		{
			name: "Should parse category path",
			content: `---
name: coder
provider: claude
category_path: ["Marketing", "Sales"]
---

Prompt.
`,
			wantPath: []string{"Marketing", "Sales"},
		},
		{
			name: "Should return nil when category path is missing",
			content: `---
name: coder
provider: claude
---

Prompt.
`,
		},
		{
			name: "Should return nil when category path is an empty array",
			content: `---
name: coder
provider: claude
category_path: []
---

Prompt.
`,
		},
		{
			name: "Should trim whitespace segments",
			content: `---
name: coder
provider: claude
category_path: ["  Marketing  ", " Sales "]
---

Prompt.
`,
			wantPath: []string{"Marketing", "Sales"},
		},
		{
			name: "Should reject blank segment",
			content: `---
name: coder
provider: claude
category_path: ["Marketing", ""]
---

Prompt.
`,
			wantErr: "agent.category_path[1]",
		},
		{
			name: "Should reject whitespace only segment",
			content: `---
name: coder
provider: claude
category_path: ["   "]
---

Prompt.
`,
			wantErr: "blank segment",
		},
		{
			name: "Should reject dot segment",
			content: `---
name: coder
provider: claude
category_path: ["."]
---

Prompt.
`,
			wantErr: "agent.category_path[0]",
		},
		{
			name: "Should reject dot dot segment",
			content: `---
name: coder
provider: claude
category_path: [".."]
---

Prompt.
`,
			wantErr: "agent.category_path[0]",
		},
		{
			name: "Should reject forward slash in segment",
			content: `---
name: coder
provider: claude
category_path: ["Marketing/Sales"]
---

Prompt.
`,
			wantErr: "must not contain",
		},
		{
			name: "Should reject backslash in segment",
			content: `---
name: coder
provider: claude
category_path: ["Marketing\\Sales"]
---

Prompt.
`,
			wantErr: "must not contain",
		},
		{
			name: "Should reject non array value",
			content: `---
name: coder
provider: claude
category_path: "Marketing"
---

Prompt.
`,
			wantErr: "category_path",
		},
		{
			name: "Should reject categories alias key",
			content: `---
name: coder
provider: claude
categories: ["Marketing"]
---

Prompt.
`,
			wantErr: "categories",
		},
		{
			name: "Should parse category path from TOML frontmatter",
			content: `---
name = "coder"
provider = "claude"
category_path = ["Marketing", "Sales"]
---

Prompt.
`,
			wantPath: []string{"Marketing", "Sales"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseAgentDef([]byte(tt.content))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("ParseAgentDef() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseAgentDef() error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAgentDef() error = %v", err)
			}
			if !equalStringSlicesForTest(agent.CategoryPath, tt.wantPath) {
				t.Fatalf("ParseAgentDef() CategoryPath = %#v, want %#v", agent.CategoryPath, tt.wantPath)
			}
		})
	}
}

func equalStringSlicesForTest(got []string, want []string) bool {
	if (got == nil) != (want == nil) {
		return false
	}
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}
