---
status: resolved
file: internal/core/agent/registry_overlay.go
line: 171
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVO-,comment:PRRC_kwDORy7nkc621Vay
---

# Issue 014: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>❓ Verification inconclusive</summary>

Script executed:

```shell
#!/bin/bash
set -euo pipefail

python - <<'PY'
import shlex

samples = [
    '"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" --stdio',
    '"/opt/My Tool/bin/tool" --flag "two words"',
]

for sample in samples:
    print("raw:           ", sample)
    print("fields-like:   ", sample.split())
    print("shell-aware:   ", shlex.split(sample))
    print()
PY

rg -n -C2 --glob '*.json' --glob '*.yaml' --glob '*.yml' \
  '"(command|fixed_args|probe_args)"\s*:\s*"[^"]*[[:space:]][^"]*"|(command|fixed_args|probe_args):\s*".*[[:space:]].*"'
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 1


stdout:
```
raw:            "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" --stdio
fields-like:    ['"/Applications/Visual', 'Studio', 'Code.app/Contents/Resources/app/bin/code"', '--stdio']
shell-aware:    ['/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code', '--stdio']

raw:            "/opt/My Tool/bin/tool" --flag "two words"
fields-like:    ['"/opt/My', 'Tool/bin/tool"', '--flag', '"two', 'words"']
shell-aware:    ['/opt/My Tool/bin/tool', '--flag', 'two words']

```

---

Script executed:

```shell
# Read the file to understand the full context
cat -n internal/core/agent/registry_overlay.go | head -180
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
     1	package agent
     2	
     3	import (
     4		"fmt"
     5		"slices"
     6		"strconv"
     7		"strings"
     8		"sync"
     9	
    10		"github.com/compozy/compozy/internal/core/model"
    11	)
    12	
    13	type catalogSnapshot struct {
    14		specs map[string]Spec
    15		order []string
    16	}
    17	
    18	var (
    19		activeCatalogMu sync.RWMutex
    20		activeCatalog   *catalogSnapshot
    21	)
    22	
    23	// OverlayEntry captures one declarative ACP runtime overlay entry assembled during command bootstrap.
    24	type OverlayEntry struct {
    25		Name     string
    26		Command  string
    27		Metadata map[string]string
    28	}
    29	
    30	// ActivateOverlay installs one command-scoped ACP runtime overlay built from
    31	// extension-declared IDE providers and returns a restore function.
    32	func ActivateOverlay(entries []OverlayEntry) (func(), error) {
    33		snapshot, err := buildOverlayCatalog(entries)
    34		if err != nil {
    35			return nil, err
    36		}
    37	
    38		activeCatalogMu.Lock()
    39		previous := activeCatalog
    40		activeCatalog = snapshot
    41		activeCatalogMu.Unlock()
    42	
    43		return func() {
    44			activeCatalogMu.Lock()
    45			activeCatalog = previous
    46			activeCatalogMu.Unlock()
    47		}, nil
    48	}
    49	
    50	func buildOverlayCatalog(entries []OverlayEntry) (*catalogSnapshot, error) {
    51		if len(entries) == 0 {
    52			return nil, nil
    53		}
    54	
    55		snapshot := baseCatalogSnapshot()
    56		added := make([]string, 0)
    57		for _, entry := range entries {
    58			spec, err := specFromDeclaredIDEProvider(entry)
    59			if err != nil {
    60				return nil, err
    61			}
    62			if _, ok := snapshot.specs[spec.ID]; !ok {
    63				added = append(added, spec.ID)
    64			}
    65			snapshot.specs[spec.ID] = spec
    66		}
    67	
    68		slices.Sort(added)
    69		snapshot.order = append(snapshot.order, added...)
    70		return &snapshot, nil
    71	}
    72	
    73	func currentCatalogSnapshot() catalogSnapshot {
    74		activeCatalogMu.RLock()
    75		if activeCatalog != nil {
    76			snapshot := cloneCatalogSnapshot(*activeCatalog)
    77			activeCatalogMu.RUnlock()
    78			return snapshot
    79		}
    80		activeCatalogMu.RUnlock()
    81	
    82		return baseCatalogSnapshot()
    83	}
    84	
    85	func baseCatalogSnapshot() catalogSnapshot {
    86		registryMu.RLock()
    87		defer registryMu.RUnlock()
    88	
    89		specs := make(map[string]Spec, len(registry))
    90		for ide := range registry {
    91			spec := registry[ide]
    92			specs[ide] = cloneAgentSpec(spec)
    93		}
    94		return catalogSnapshot{
    95			specs: specs,
    96			order: append([]string(nil), supportedRegistryIDEOrder...),
    97		}
    98	}
    99	
   100	func cloneCatalogSnapshot(snapshot catalogSnapshot) catalogSnapshot {
   101		specs := make(map[string]Spec, len(snapshot.specs))
   102		for ide := range snapshot.specs {
   103			spec := snapshot.specs[ide]
   104			specs[ide] = cloneAgentSpec(spec)
   105		}
   106		return catalogSnapshot{
   107			specs: specs,
   108			order: append([]string(nil), snapshot.order...),
   109		}
   110	}
   111	
   112	func specFromDeclaredIDEProvider(entry OverlayEntry) (Spec, error) {
   113		id := normalizeOverlayIdentifier(entry.Name)
   114		if id == "" {
   115			return Spec{}, fmt.Errorf("declare ACP runtime overlay: provider name is required")
   116		}
   117	
   118		command, fixedArgs, err := splitOverlayCommand(entry.Command)
   119		if err != nil {
   120			return Spec{}, fmt.Errorf("declare ACP runtime overlay %q: %w", entry.Name, err)
   121		}
   122	
   123		if metadataFixedArgs := parseOverlayArgs(entry.Metadata["fixed_args"]); len(metadataFixedArgs) > 0 {
   124			fixedArgs = metadataFixedArgs
   125		}
   126	
   127		spec := Spec{
   128			ID: id,
   129			DisplayName: overlayFirstNonEmpty(
   130				strings.TrimSpace(entry.Metadata["display_name"]),
   131				strings.TrimSpace(entry.Name),
   132			),
   133			SetupAgentName: strings.TrimSpace(entry.Metadata["agent_name"]),
   134			DefaultModel: overlayFirstNonEmpty(
   135				strings.TrimSpace(entry.Metadata["default_model"]),
   136				model.DefaultCodexModel,
   137			),
   138			Command:            command,
   139			FixedArgs:          fixedArgs,
   140			ProbeArgs:          parseOverlayArgs(entry.Metadata["probe_args"]),
   141			SupportsAddDirs:    parseOverlayBool(entry.Metadata["supports_add_dirs"]),
   142			UsesBootstrapModel: parseOverlayBool(entry.Metadata["uses_bootstrap_model"]),
   143			DocsURL:            strings.TrimSpace(entry.Metadata["docs_url"]),
   144			InstallHint:        strings.TrimSpace(entry.Metadata["install_hint"]),
   145			FullAccessModeID:   strings.TrimSpace(entry.Metadata["full_access_mode_id"]),
   146			EnvVars:            parseOverlayEnv(entry.Metadata),
   147		}
   148		if strings.TrimSpace(spec.DisplayName) == "" {
   149			spec.DisplayName = spec.ID
   150		}
   151		return spec, nil
   152	}
   153	
   154	func normalizeOverlayIdentifier(value string) string {
   155		return strings.TrimSpace(strings.ToLower(value))
   156	}
   157	
   158	func splitOverlayCommand(raw string) (string, []string, error) {
   159		parts := strings.Fields(strings.TrimSpace(raw))
   160		if len(parts) == 0 {
   161			return "", nil, fmt.Errorf("command is required")
   162		}
   163		return parts[0], parts[1:], nil
   164	}
   165	
   166	func parseOverlayArgs(raw string) []string {
   167		trimmed := strings.TrimSpace(raw)
   168		if trimmed == "" {
   169			return nil
   170		}
   171		return strings.Fields(trimmed)
   172	}
   173	
   174	func parseOverlayBool(raw string) bool {
   175		trimmed := strings.TrimSpace(raw)
   176		if trimmed == "" {
   177			return false
   178		}
   179		parsed, err := strconv.ParseBool(trimmed)
   180		return err == nil && parsed
```

---

Script executed:

```shell
# Search for calls to these functions
rg -n "splitOverlayCommand|parseOverlayArgs" --type go
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
internal/core/agent/registry_overlay.go:118:	command, fixedArgs, err := splitOverlayCommand(entry.Command)
internal/core/agent/registry_overlay.go:123:	if metadataFixedArgs := parseOverlayArgs(entry.Metadata["fixed_args"]); len(metadataFixedArgs) > 0 {
internal/core/agent/registry_overlay.go:140:		ProbeArgs:          parseOverlayArgs(entry.Metadata["probe_args"]),
internal/core/agent/registry_overlay.go:158:func splitOverlayCommand(raw string) (string, []string, error) {
internal/core/agent/registry_overlay.go:166:func parseOverlayArgs(raw string) []string {
```

---

Script executed:

```shell
# Look for overlay configs or test data
fd -e json -e yaml -e yml | xargs rg -l "command|fixed_args|probe_args" 2>/dev/null | head -10
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
docs/examples/agents/repo-copilot/mcp.json
```

---

Script executed:

```shell
# Check if there are any integration tests or examples
fd -e json -e yaml -e yml . | xargs grep -l "overlays\|overlay" 2>/dev/null | head -5
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


---

Script executed:

```shell
# Check the example config file to see if it contains commands with spaces
cat docs/examples/agents/repo-copilot/mcp.json | head -100
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "${PROJECT_ROOT}"
      ]
    },
    "github": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-github"
      ],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}
```

---

Script executed:

```shell
# Look for test files or other configurations
find . -name "*overlay*" -type f | head -20
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
./internal/core/agent/registry_overlay.go
./internal/core/agent/registry_overlay_test.go
./internal/core/provider/overlay.go
./internal/core/provider/overlay_test.go
```

---

Script executed:

```shell
# Check if there are any tests for these functions
rg -l "splitOverlayCommand|parseOverlayArgs|OverlayEntry" --type go
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
internal/core/provider/overlay_test.go
internal/core/provider/overlay.go
internal/cli/extensions_bootstrap.go
internal/core/agent/registry_overlay.go
internal/core/agent/registry_overlay_test.go
```

---

Script executed:

```shell
# Check the test file for registry_overlay
cat -n internal/core/agent/registry_overlay_test.go
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
     1	package agent
     2	
     3	import (
     4		"testing"
     5	
     6		"github.com/compozy/compozy/internal/core/model"
     7	)
     8	
     9	func TestActivateOverlayRegistersDeclarativeRuntimeSpec(t *testing.T) {
    10		restore, err := ActivateOverlay([]OverlayEntry{
    11			{
    12				Name:    "ext-adapter",
    13				Command: "mock-acp --serve",
    14				Metadata: map[string]string{
    15					"display_name":      "Mock ACP",
    16					"default_model":     "mock-model",
    17					"agent_name":        "codex",
    18					"supports_add_dirs": "true",
    19				},
    20			},
    21		})
    22		if err != nil {
    23			t.Fatalf("activate ACP overlay: %v", err)
    24		}
    25		defer restore()
    26	
    27		if err := ValidateRuntimeConfig(&model.RuntimeConfig{
    28			Mode:                   model.ExecutionModePRDTasks,
    29			IDE:                    "ext-adapter",
    30			OutputFormat:           model.OutputFormatText,
    31			BatchSize:              1,
    32			MaxRetries:             0,
    33			RetryBackoffMultiplier: 1.5,
    34		}); err != nil {
    35			t.Fatalf("validate runtime config with overlay IDE: %v", err)
    36		}
    37	
    38		spec, err := lookupAgentSpec("ext-adapter")
    39		if err != nil {
    40			t.Fatalf("lookup overlay spec: %v", err)
    41		}
    42		if spec.Command != "mock-acp" {
    43			t.Fatalf("unexpected overlay command: %q", spec.Command)
    44		}
    45		if len(spec.FixedArgs) != 1 || spec.FixedArgs[0] != "--serve" {
    46			t.Fatalf("unexpected overlay fixed args: %#v", spec.FixedArgs)
    47		}
    48		if spec.SetupAgentName != "codex" {
    49			t.Fatalf("unexpected setup agent name: %q", spec.SetupAgentName)
    50		}
    51		if got := DisplayName("ext-adapter"); got != "Mock ACP" {
    52			t.Fatalf("unexpected overlay display name: %q", got)
    53		}
    54		if got, err := SetupAgentName("ext-adapter"); err != nil || got != "codex" {
    55			t.Fatalf("unexpected overlay setup agent mapping: got %q err=%v", got, err)
    56		}
    57		if got, err := ResolveRuntimeModel("ext-adapter", ""); err != nil || got != "mock-model" {
    58			t.Fatalf("unexpected overlay runtime model: got %q err=%v", got, err)
    59		}
    60	}
```

---

Script executed:

```shell
# Check the provider overlay test
cat -n internal/core/provider/overlay_test.go
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
     1	package provider
     2	
     3	import (
     4		"context"
     5		"strings"
     6		"testing"
     7	)
     8	
     9	type overlayTestProvider struct {
    10		name string
    11	}
    12	
    13	func (p overlayTestProvider) Name() string { return p.name }
    14	
    15	func (overlayTestProvider) FetchReviews(context.Context, FetchRequest) ([]ReviewItem, error) {
    16		return nil, nil
    17	}
    18	
    19	func (overlayTestProvider) ResolveIssues(context.Context, string, []ResolvedIssue) error {
    20		return nil
    21	}
    22	
    23	func TestOverlayRegistryReturnsOverlayProviderBeforeBaseProvider(t *testing.T) {
    24		t.Parallel()
    25	
    26		base := NewRegistry()
    27		base.Register(overlayTestProvider{name: "base"})
    28	
    29		overlay := NewOverlayRegistry(base)
    30		overlay.Register(overlayTestProvider{name: "ext"})
    31	
    32		provider, err := overlay.Get("ext")
    33		if err != nil {
    34			t.Fatalf("overlay get ext: %v", err)
    35		}
    36		if got := provider.Name(); got != "ext" {
    37			t.Fatalf("unexpected overlay provider name: %q", got)
    38		}
    39	
    40		baseProvider, err := overlay.Get("base")
    41		if err != nil {
    42			t.Fatalf("overlay get base: %v", err)
    43		}
    44		if got := baseProvider.Name(); got != "base" {
    45			t.Fatalf("unexpected base provider name: %q", got)
    46		}
    47	}
    48	
    49	func TestOverlayRegistryDoesNotMutateBaseRegistry(t *testing.T) {
    50		t.Parallel()
    51	
    52		base := NewRegistry()
    53		base.Register(overlayTestProvider{name: "base"})
    54	
    55		overlay := NewOverlayRegistry(base)
    56		overlay.Register(overlayTestProvider{name: "ext"})
    57	
    58		if _, err := base.Get("ext"); err == nil {
    59			t.Fatal("expected base registry to remain unchanged")
    60		}
    61	}
    62	
    63	func TestActivateOverlayBuildsAliasedReviewProvider(t *testing.T) {
    64		restore, err := ActivateOverlay([]OverlayEntry{{Name: "ext-review", Command: "base"}})
    65		if err != nil {
    66			t.Fatalf("activate review overlay: %v", err)
    67		}
    68		defer restore()
    69	
    70		base := NewRegistry()
    71		base.Register(overlayTestProvider{name: "base"})
    72	
    73		registry := ResolveRegistry(base)
    74		provider, err := registry.Get("ext-review")
    75		if err != nil {
    76			t.Fatalf("resolve overlay provider: %v", err)
    77		}
    78		if got := provider.Name(); got != "ext-review" {
    79			t.Fatalf("unexpected overlay provider name: %q", got)
    80		}
    81	
    82		if _, err := provider.FetchReviews(context.Background(), FetchRequest{}); err != nil {
    83			t.Fatalf("delegate overlay fetch: %v", err)
    84		}
    85	}
    86	
    87	func TestResolveRegistryReturnsBaseWhenNoOverlayIsActive(t *testing.T) {
    88		t.Parallel()
    89	
    90		base := NewRegistry()
    91		base.Register(overlayTestProvider{name: "base"})
    92	
    93		resolved := ResolveRegistry(base)
    94		if resolved != base {
    95			t.Fatal("expected resolve registry to return the base registry when no overlay is active")
    96		}
    97	}
    98	
    99	func TestAliasedProviderResolveIssuesDelegatesToTarget(t *testing.T) {
   100		restore, err := ActivateOverlay([]OverlayEntry{{Name: "ext-review", Command: "base"}})
   101		if err != nil {
   102			t.Fatalf("activate review overlay: %v", err)
   103		}
   104		defer restore()
   105	
   106		base := NewRegistry()
   107		base.Register(&overlayTestProvider{name: "base"})
   108	
   109		registry := ResolveRegistry(base)
   110		resolved, err := registry.Get("ext-review")
   111		if err != nil {
   112			t.Fatalf("resolve overlay provider: %v", err)
   113		}
   114		if err := resolved.ResolveIssues(context.Background(), "123", nil); err != nil {
   115			t.Fatalf("delegate overlay resolve issues: %v", err)
   116		}
   117	}
   118	
   119	func TestAliasedProviderRejectsInvalidTargets(t *testing.T) {
   120		t.Parallel()
   121	
   122		tests := []struct {
   123			name    string
   124			prov    *aliasedProvider
   125			wantErr string
   126		}{
   127			{
   128				name:    "missing target",
   129				prov:    &aliasedProvider{name: "ext-review", registry: NewRegistry()},
   130				wantErr: `missing a target provider name`,
   131			},
   132			{
   133				name:    "self target",
   134				prov:    &aliasedProvider{name: "ext-review", targetName: "ext-review", registry: NewRegistry()},
   135				wantErr: `cannot target itself`,
   136			},
   137			{
   138				name:    "nil provider",
   139				prov:    nil,
   140				wantErr: `declared review provider is nil`,
   141			},
   142		}
   143	
   144		for _, tc := range tests {
   145			tc := tc
   146			t.Run(tc.name, func(t *testing.T) {
   147				t.Parallel()
   148	
   149				_, err := tc.prov.resolveTarget(nil)
   150				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
   151					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
   152				}
   153			})
   154		}
   155	}
   156	
   157	func TestAliasedProviderRejectsAliasCycle(t *testing.T) {
   158		base := NewRegistry()
   159		overlay := NewOverlayRegistry(base)
   160		first := &aliasedProvider{name: "first", targetName: "second", registry: overlay}
   161		second := &aliasedProvider{name: "second", targetName: "first", registry: overlay}
   162		overlay.Register(first)
   163		overlay.Register(second)
   164	
   165		_, err := first.resolveTarget(nil)
   166		if err == nil || !strings.Contains(err.Error(), `alias cycle`) {
   167			t.Fatalf("expected alias cycle error, got %v", err)
   168		}
   169	}
```

---

Script executed:

```shell
# Look at how OverlayEntry is created in extensions bootstrap
rg -A 10 -B 5 "OverlayEntry{" internal/cli/extensions_bootstrap.go
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 2


stderr:
```
regex parse error:
    OverlayEntry{
                ^
error: unclosed counted repetition
```

---

Script executed:

```shell
# Look for where OverlayEntry is actually created with commands
rg "OverlayEntry" -A 5 --type go | head -100
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
internal/core/agent/registry_overlay_test.go:	restore, err := ActivateOverlay([]OverlayEntry{
internal/core/agent/registry_overlay_test.go-		{
internal/core/agent/registry_overlay_test.go-			Name:    "ext-adapter",
internal/core/agent/registry_overlay_test.go-			Command: "mock-acp --serve",
internal/core/agent/registry_overlay_test.go-			Metadata: map[string]string{
internal/core/agent/registry_overlay_test.go-				"display_name":      "Mock ACP",
--
internal/core/agent/registry_overlay.go:// OverlayEntry captures one declarative ACP runtime overlay entry assembled during command bootstrap.
internal/core/agent/registry_overlay.go:type OverlayEntry struct {
internal/core/agent/registry_overlay.go-	Name     string
internal/core/agent/registry_overlay.go-	Command  string
internal/core/agent/registry_overlay.go-	Metadata map[string]string
internal/core/agent/registry_overlay.go-}
internal/core/agent/registry_overlay.go-
--
internal/core/agent/registry_overlay.go:func ActivateOverlay(entries []OverlayEntry) (func(), error) {
internal/core/agent/registry_overlay.go-	snapshot, err := buildOverlayCatalog(entries)
internal/core/agent/registry_overlay.go-	if err != nil {
internal/core/agent/registry_overlay.go-		return nil, err
internal/core/agent/registry_overlay.go-	}
internal/core/agent/registry_overlay.go-
--
internal/core/agent/registry_overlay.go:func buildOverlayCatalog(entries []OverlayEntry) (*catalogSnapshot, error) {
internal/core/agent/registry_overlay.go-	if len(entries) == 0 {
internal/core/agent/registry_overlay.go-		return nil, nil
internal/core/agent/registry_overlay.go-	}
internal/core/agent/registry_overlay.go-
internal/core/agent/registry_overlay.go-	snapshot := baseCatalogSnapshot()
--
internal/core/agent/registry_overlay.go:func specFromDeclaredIDEProvider(entry OverlayEntry) (Spec, error) {
internal/core/agent/registry_overlay.go-	id := normalizeOverlayIdentifier(entry.Name)
internal/core/agent/registry_overlay.go-	if id == "" {
internal/core/agent/registry_overlay.go-		return Spec{}, fmt.Errorf("declare ACP runtime overlay: provider name is required")
internal/core/agent/registry_overlay.go-	}
internal/core/agent/registry_overlay.go-
--
internal/core/provider/overlay.go:// OverlayEntry captures one declarative review-provider overlay entry assembled during command bootstrap.
internal/core/provider/overlay.go:type OverlayEntry struct {
internal/core/provider/overlay.go-	Name     string
internal/core/provider/overlay.go-	Command  string
internal/core/provider/overlay.go-	Metadata map[string]string
internal/core/provider/overlay.go-}
internal/core/provider/overlay.go-
--
internal/core/provider/overlay.go:func ActivateOverlay(entries []OverlayEntry) (func(), error) {
internal/core/provider/overlay.go-	if len(entries) == 0 {
internal/core/provider/overlay.go-		return func() {}, nil
internal/core/provider/overlay.go-	}
internal/core/provider/overlay.go-
internal/core/provider/overlay.go-	factory := func(base RegistryReader) RegistryReader {
--
internal/core/provider/overlay.go:func buildDeclaredReviewOverlay(base RegistryReader, entries []OverlayEntry) RegistryReader {
internal/core/provider/overlay.go-	overlay := NewOverlayRegistry(base)
internal/core/provider/overlay.go-	for _, entry := range entries {
internal/core/provider/overlay.go-		overlay.Register(&aliasedProvider{
internal/core/provider/overlay.go-			name:       strings.TrimSpace(entry.Name),
internal/core/provider/overlay.go-			targetName: strings.TrimSpace(entry.Command),
--
internal/core/provider/overlay_test.go:	restore, err := ActivateOverlay([]OverlayEntry{{Name: "ext-review", Command: "base"}})
internal/core/provider/overlay_test.go-	if err != nil {
internal/core/provider/overlay_test.go-		t.Fatalf("activate review overlay: %v", err)
internal/core/provider/overlay_test.go-	}
internal/core/provider/overlay_test.go-	defer restore()
internal/core/provider/overlay_test.go-
--
internal/core/provider/overlay_test.go:	restore, err := ActivateOverlay([]OverlayEntry{{Name: "ext-review", Command: "base"}})
internal/core/provider/overlay_test.go-	if err != nil {
internal/core/provider/overlay_test.go-		t.Fatalf("activate review overlay: %v", err)
internal/core/provider/overlay_test.go-	}
internal/core/provider/overlay_test.go-	defer restore()
internal/core/provider/overlay_test.go-
--
internal/cli/extensions_bootstrap.go:func agentOverlayEntries(entries []extensions.DeclaredProvider) []agent.OverlayEntry {
internal/cli/extensions_bootstrap.go-	if len(entries) == 0 {
internal/cli/extensions_bootstrap.go-		return nil
internal/cli/extensions_bootstrap.go-	}
internal/cli/extensions_bootstrap.go-
internal/cli/extensions_bootstrap.go:	overlays := make([]agent.OverlayEntry, 0, len(entries))
internal/cli/extensions_bootstrap.go-	for _, entry := range entries {
internal/cli/extensions_bootstrap.go:		overlays = append(overlays, agent.OverlayEntry{
internal/cli/extensions_bootstrap.go-			Name:     entry.Name,
internal/cli/extensions_bootstrap.go-			Command:  entry.Command,
internal/cli/extensions_bootstrap.go-			Metadata: mapsClone(entry.Metadata),
internal/cli/extensions_bootstrap.go-		})
internal/cli/extensions_bootstrap.go-	}
--
internal/cli/extensions_bootstrap.go:func providerOverlayEntries(entries []extensions.DeclaredProvider) []provider.OverlayEntry {
internal/cli/extensions_bootstrap.go-	if len(entries) == 0 {
internal/cli/extensions_bootstrap.go-		return nil
internal/cli/extensions_bootstrap.go-	}
internal/cli/extensions_bootstrap.go-
internal/cli/extensions_bootstrap.go:	overlays := make([]provider.OverlayEntry, 0, len(entries))
internal/cli/extensions_bootstrap.go-	for _, entry := range entries {
internal/cli/extensions_bootstrap.go:		overlays = append(overlays, provider.OverlayEntry{
internal/cli/extensions_bootstrap.go-			Name:     entry.Name,
internal/cli/extensions_bootstrap.go-			Command:  entry.Command,
internal/cli/extensions_bootstrap.go-			Metadata: mapsClone(entry.Metadata),
internal/cli/extensions_bootstrap.go-		})
internal/cli/extensions_bootstrap.go-	}
```

---

Script executed:

```shell
# Check how extensions pass commands
cat -n internal/cli/extensions_bootstrap.go | head -150
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
     1	package cli
     2	
     3	import (
     4		"context"
     5		"fmt"
     6	
     7		core "github.com/compozy/compozy/internal/core"
     8		"github.com/compozy/compozy/internal/core/agent"
     9		extensions "github.com/compozy/compozy/internal/core/extension"
    10		"github.com/compozy/compozy/internal/core/provider"
    11		"github.com/compozy/compozy/internal/setup"
    12	)
    13	
    14	type declarativeAssets struct {
    15		Discovery extensions.DiscoveryResult
    16	}
    17	
    18	func (s *commandState) bootstrapDeclarativeAssets(
    19		ctx context.Context,
    20		cfg core.Config,
    21	) (declarativeAssets, func(), error) {
    22		if !s.requiresDeclarativeAssetBootstrap() {
    23			return declarativeAssets{}, func() {}, nil
    24		}
    25	
    26		discovery, err := extensions.Discovery{WorkspaceRoot: cfg.WorkspaceRoot}.Discover(ctx)
    27		if err != nil {
    28			return declarativeAssets{}, nil, fmt.Errorf("discover declarative extension assets: %w", err)
    29		}
    30	
    31		restoreProviderOverlay, err := provider.ActivateOverlay(providerOverlayEntries(discovery.Providers.Review))
    32		if err != nil {
    33			return declarativeAssets{}, nil, fmt.Errorf("activate review provider overlay: %w", err)
    34		}
    35	
    36		restoreAgentOverlay, err := agent.ActivateOverlay(agentOverlayEntries(discovery.Providers.IDE))
    37		if err != nil {
    38			restoreProviderOverlay()
    39			return declarativeAssets{}, nil, fmt.Errorf("activate ACP runtime overlay: %w", err)
    40		}
    41	
    42		cleanup := func() {
    43			restoreAgentOverlay()
    44			restoreProviderOverlay()
    45		}
    46	
    47		return declarativeAssets{Discovery: discovery}, cleanup, nil
    48	}
    49	
    50	func (s *commandState) requiresDeclarativeAssetBootstrap() bool {
    51		if s == nil {
    52			return false
    53		}
    54	
    55		switch s.kind {
    56		case commandKindFetchReviews, commandKindFixReviews, commandKindExec, commandKindStart:
    57			return true
    58		default:
    59			return false
    60		}
    61	}
    62	
    63	func agentOverlayEntries(entries []extensions.DeclaredProvider) []agent.OverlayEntry {
    64		if len(entries) == 0 {
    65			return nil
    66		}
    67	
    68		overlays := make([]agent.OverlayEntry, 0, len(entries))
    69		for _, entry := range entries {
    70			overlays = append(overlays, agent.OverlayEntry{
    71				Name:     entry.Name,
    72				Command:  entry.Command,
    73				Metadata: mapsClone(entry.Metadata),
    74			})
    75		}
    76		return overlays
    77	}
    78	
    79	func providerOverlayEntries(entries []extensions.DeclaredProvider) []provider.OverlayEntry {
    80		if len(entries) == 0 {
    81			return nil
    82		}
    83	
    84		overlays := make([]provider.OverlayEntry, 0, len(entries))
    85		for _, entry := range entries {
    86			overlays = append(overlays, provider.OverlayEntry{
    87				Name:     entry.Name,
    88				Command:  entry.Command,
    89				Metadata: mapsClone(entry.Metadata),
    90			})
    91		}
    92		return overlays
    93	}
    94	
    95	func extensionSkillSources(packs []extensions.DeclaredSkillPack) []setup.SkillPackSource {
    96		if len(packs) == 0 {
    97			return nil
    98		}
    99	
   100		sources := make([]setup.SkillPackSource, 0, len(packs))
   101		for i := range packs {
   102			pack := &packs[i]
   103			sources = append(sources, setup.SkillPackSource{
   104				ExtensionName: pack.Extension.Name,
   105				ManifestPath:  pack.ManifestPath,
   106				Pattern:       pack.Pattern,
   107				ResolvedPath:  pack.ResolvedPath,
   108				SourceFS:      pack.SourceFS,
   109				SourceDir:     pack.SourceDir,
   110			})
   111		}
   112		return sources
   113	}
   114	
   115	func mapsClone(src map[string]string) map[string]string {
   116		if len(src) == 0 {
   117			return nil
   118		}
   119		dst := make(map[string]string, len(src))
   120		for key, value := range src {
   121			dst[key] = value
   122		}
   123		return dst
   124	}
```

---

Script executed:

```shell
# Check if there's any documentation about the extension format
fd -e md -e txt | xargs grep -l "DeclaredProvider\|extension.*command" 2>/dev/null | head -5
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


---

Script executed:

```shell
# Look at the extensions package to understand the input format
rg -n "type DeclaredProvider" --type go -A 5
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
internal/core/extension/assets.go:12:type DeclaredProvider struct {
internal/core/extension/assets.go-13-	Extension    Ref
internal/core/extension/assets.go-14-	ManifestPath string
internal/core/extension/assets.go-15-	Name         string
internal/core/extension/assets.go-16-	Command      string
internal/core/extension/assets.go-17-	Metadata     map[string]string
--
internal/core/extension/assets.go:21:type DeclaredProviders struct {
internal/core/extension/assets.go-22-	IDE    []DeclaredProvider
internal/core/extension/assets.go-23-	Review []DeclaredProvider
internal/core/extension/assets.go-24-	Model  []DeclaredProvider
internal/core/extension/assets.go-25-}
internal/core/extension/assets.go-26-
```

---

Script executed:

```shell
# Find where DeclaredProvider is populated from manifest files
rg -n "DeclaredProvider" --type go -B 5 -A 3 | grep -A 3 -B 3 "Command:"
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 1


---

Script executed:

```shell
# Check for manifest schema or examples
fd -e schema.json -o -e manifest.json -o -name "*.schema.*" | head -10
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stderr:
```
error: The argument '--owner <user:group>' requires a value but none was supplied

For more information try '--help'
```

---

Script executed:

```shell
# Look for extension parsing code
rg -n "manifest.*parse\|decode.*manifest\|json.*struct\|yaml.*struct" --type go internal/core/extension/ | head -20
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


---

Script executed:

```shell
# Find the extension discovery code
find . -path ./node_modules -prune -o -name "*.go" -exec grep -l "Discovery\|discover" {} \; | grep extension
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
./internal/cli/extension/display.go
./internal/cli/extension/display_test.go
./internal/cli/extension/doctor.go
./internal/cli/extension/enablement.go
./internal/cli/extension/install.go
./internal/cli/extension/root.go
./internal/cli/extensions_bootstrap.go
./internal/core/extension/assets.go
./internal/core/extension/builtin/doc.go
./internal/core/extension/discovery.go
./internal/core/extension/discovery_test.go
./internal/core/extension/doc.go
./internal/core/extension/hooks_integration_test.go
./internal/core/extension/manager_test.go
./internal/core/extension/manifest.go
./internal/core/extension/runtime.go
./internal/core/extension/runtime_test.go
./internal/setup/extensions.go
./sdk/extension/handlers.go
./sdk/extension/hooks.go
./sdk/extension/types.go
```

---

Script executed:

```shell
# Check manifest loading
rg -n "LoadManifest\|ParseManifest\|ReadManifest" --type go | head -20
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


---

Script executed:

```shell
# Look for JSON/YAML unmarshaling of manifest structures
rg -n "json.Unmarshal\|yaml.Unmarshal" --type go -A 2 | grep -B 2 -A 2 "Command"
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 1


---

Script executed:

```shell
# Look at manifest parsing
cat -n internal/core/extension/manifest.go | head -200
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
     1	package extensions
     2	
     3	import "time"
     4	
     5	const (
     6		// ManifestFileNameTOML is the preferred manifest filename.
     7		ManifestFileNameTOML = "extension.toml"
     8		// ManifestFileNameJSON is the JSON fallback manifest filename.
     9		ManifestFileNameJSON = "extension.json"
    10		// DefaultHookPriority is the default priority for hook declarations.
    11		DefaultHookPriority = 500
    12		// MinHookPriority is the lowest allowed hook priority.
    13		MinHookPriority = 0
    14		// MaxHookPriority is the highest allowed hook priority.
    15		MaxHookPriority = 1000
    16	)
    17	
    18	// Source identifies where an extension was discovered.
    19	type Source string
    20	
    21	const (
    22		// SourceBundled identifies bundled extensions embedded in the Compozy binary.
    23		SourceBundled Source = "bundled"
    24		// SourceUser identifies user-scoped extensions installed under the user's home directory.
    25		SourceUser Source = "user"
    26		// SourceWorkspace identifies workspace-scoped extensions stored in the repository.
    27		SourceWorkspace Source = "workspace"
    28	)
    29	
    30	// Capability declares a manifest capability grant.
    31	type Capability string
    32	
    33	// Capability values define the supported extension capability taxonomy.
    34	const (
    35		CapabilityEventsRead        Capability = "events.read"
    36		CapabilityEventsPublish     Capability = "events.publish"
    37		CapabilityPromptMutate      Capability = "prompt.mutate"
    38		CapabilityPlanMutate        Capability = "plan.mutate"
    39		CapabilityAgentMutate       Capability = "agent.mutate"
    40		CapabilityJobMutate         Capability = "job.mutate"
    41		CapabilityRunMutate         Capability = "run.mutate"
    42		CapabilityReviewMutate      Capability = "review.mutate"
    43		CapabilityArtifactsRead     Capability = "artifacts.read"
    44		CapabilityArtifactsWrite    Capability = "artifacts.write"
    45		CapabilityTasksRead         Capability = "tasks.read"
    46		CapabilityTasksCreate       Capability = "tasks.create"
    47		CapabilityRunsStart         Capability = "runs.start"
    48		CapabilityMemoryRead        Capability = "memory.read"
    49		CapabilityMemoryWrite       Capability = "memory.write"
    50		CapabilityProvidersRegister Capability = "providers.register"
    51		CapabilitySkillsShip        Capability = "skills.ship"
    52		CapabilitySubprocessSpawn   Capability = "subprocess.spawn"
    53		CapabilityNetworkEgress     Capability = "network.egress"
    54	)
    55	
    56	// HookName identifies a canonical extension hook event.
    57	type HookName string
    58	
    59	// Hook names define the supported extension hook taxonomy.
    60	const (
    61		HookPlanPreDiscover        HookName = "plan.pre_discover"
    62		HookPlanPostDiscover       HookName = "plan.post_discover"
    63		HookPlanPreGroup           HookName = "plan.pre_group"
    64		HookPlanPostGroup          HookName = "plan.post_group"
    65		HookPlanPrePrepareJobs     HookName = "plan.pre_prepare_jobs"
    66		HookPlanPostPrepareJobs    HookName = "plan.post_prepare_jobs"
    67		HookPromptPreBuild         HookName = "prompt.pre_build"
    68		HookPromptPostBuild        HookName = "prompt.post_build"
    69		HookPromptPreSystem        HookName = "prompt.pre_system"
    70		HookAgentPreSessionCreate  HookName = "agent.pre_session_create"
    71		HookAgentPostSessionCreate HookName = "agent.post_session_create"
    72		HookAgentPreSessionResume  HookName = "agent.pre_session_resume"
    73		HookAgentOnSessionUpdate   HookName = "agent.on_session_update"
    74		HookAgentPostSessionEnd    HookName = "agent.post_session_end"
    75		HookJobPreExecute          HookName = "job.pre_execute"
    76		HookJobPostExecute         HookName = "job.post_execute"
    77		HookJobPreRetry            HookName = "job.pre_retry"
    78		HookRunPreStart            HookName = "run.pre_start"
    79		HookRunPostStart           HookName = "run.post_start"
    80		HookRunPreShutdown         HookName = "run.pre_shutdown"
    81		HookRunPostShutdown        HookName = "run.post_shutdown"
    82		HookReviewPreFetch         HookName = "review.pre_fetch"
    83		HookReviewPostFetch        HookName = "review.post_fetch"
    84		HookReviewPreBatch         HookName = "review.pre_batch"
    85		HookReviewPostFix          HookName = "review.post_fix"
    86		HookReviewPreResolve       HookName = "review.pre_resolve"
    87		HookArtifactPreWrite       HookName = "artifact.pre_write"
    88		HookArtifactPostWrite      HookName = "artifact.post_write"
    89	)
    90	
    91	// Manifest is the parsed extension manifest shared by discovery and runtime tasks.
    92	type Manifest struct {
    93		Extension  ExtensionInfo     `toml:"extension"  json:"extension"`
    94		Subprocess *SubprocessConfig `toml:"subprocess" json:"subprocess,omitempty"`
    95		Security   SecurityConfig    `toml:"security"   json:"security"`
    96		Hooks      []HookDeclaration `toml:"hooks"      json:"hooks,omitempty"`
    97		Resources  ResourcesConfig   `toml:"resources"  json:"resources,omitempty"`
    98		Providers  ProvidersConfig   `toml:"providers"  json:"providers,omitempty"`
    99	}
   100	
   101	// ExtensionInfo contains the identifying metadata for one extension.
   102	type ExtensionInfo struct {
   103		Name              string `toml:"name"                json:"name"`
   104		Version           string `toml:"version"             json:"version"`
   105		Description       string `toml:"description"         json:"description"`
   106		MinCompozyVersion string `toml:"min_compozy_version" json:"min_compozy_version"`
   107	}
   108	
   109	// SubprocessConfig configures the extension subprocess entrypoint.
   110	type SubprocessConfig struct {
   111		Command           string            `toml:"command"             json:"command"`
   112		Args              []string          `toml:"args"                json:"args,omitempty"`
   113		Env               map[string]string `toml:"env"                 json:"env,omitempty"`
   114		ShutdownTimeout   time.Duration     `toml:"shutdown_timeout"    json:"shutdown_timeout,omitempty"`
   115		HealthCheckPeriod time.Duration     `toml:"health_check_period" json:"health_check_period,omitempty"`
   116	}
   117	
   118	// SecurityConfig declares the capabilities requested by an extension.
   119	type SecurityConfig struct {
   120		Capabilities []Capability `toml:"capabilities" json:"capabilities"`
   121	}
   122	
   123	// HookDeclaration declares one hook subscription exposed by an extension.
   124	type HookDeclaration struct {
   125		Event    HookName      `toml:"event"    json:"event"`
   126		Priority int           `toml:"priority" json:"priority,omitempty"`
   127		Required bool          `toml:"required" json:"required,omitempty"`
   128		Timeout  time.Duration `toml:"timeout"  json:"timeout,omitempty"`
   129	}
   130	
   131	// ResourcesConfig declares declarative assets shipped with an extension.
   132	type ResourcesConfig struct {
   133		Skills []string `toml:"skills" json:"skills,omitempty"`
   134	}
   135	
   136	// ProvidersConfig declares provider overlays exported by an extension.
   137	type ProvidersConfig struct {
   138		IDE    []ProviderEntry `toml:"ide"    json:"ide,omitempty"`
   139		Review []ProviderEntry `toml:"review" json:"review,omitempty"`
   140		Model  []ProviderEntry `toml:"model"  json:"model,omitempty"`
   141	}
   142	
   143	// ProviderEntry declares one provider overlay entry from a manifest.
   144	type ProviderEntry struct {
   145		Name     string            `toml:"name"     json:"name"`
   146		Command  string            `toml:"command"  json:"command"`
   147		Metadata map[string]string `toml:"metadata" json:"metadata,omitempty"`
   148	}
   149	
   150	type capabilitySet map[Capability]struct{}
   151	
   152	func newCapabilitySet(values ...Capability) capabilitySet {
   153		set := make(capabilitySet, len(values))
   154		for _, value := range values {
   155			set[value] = struct{}{}
   156		}
   157		return set
   158	}
   159	
   160	func (s capabilitySet) contains(value Capability) bool {
   161		_, ok := s[value]
   162		return ok
   163	}
   164	
   165	type hookNameSet map[HookName]struct{}
   166	
   167	func newHookNameSet(values ...HookName) hookNameSet {
   168		set := make(hookNameSet, len(values))
   169		for _, value := range values {
   170			set[value] = struct{}{}
   171		}
   172		return set
   173	}
   174	
   175	func (s hookNameSet) contains(value HookName) bool {
   176		_, ok := s[value]
   177		return ok
   178	}
   179	
   180	var supportedCapabilities = newCapabilitySet(
   181		CapabilityEventsRead,
   182		CapabilityEventsPublish,
   183		CapabilityPromptMutate,
   184		CapabilityPlanMutate,
   185		CapabilityAgentMutate,
   186		CapabilityJobMutate,
   187		CapabilityRunMutate,
   188		CapabilityReviewMutate,
   189		CapabilityArtifactsRead,
   190		CapabilityArtifactsWrite,
   191		CapabilityTasksRead,
   192		CapabilityTasksCreate,
   193		CapabilityRunsStart,
   194		CapabilityMemoryRead,
   195		CapabilityMemoryWrite,
   196		CapabilityProvidersRegister,
   197		CapabilitySkillsShip,
   198		CapabilitySubprocessSpawn,
   199		CapabilityNetworkEgress,
   200	)
```

---

Script executed:

```shell
# Look at discovery code
cat -n internal/core/extension/discovery.go
```

Repository: compozy/compozy

---

Repository: compozy/compozy
Exit code: 0


stdout:
```
     1	package extensions
     2	
     3	import (
     4		"context"
     5		"errors"
     6		"fmt"
     7		"io/fs"
     8		"log/slog"
     9		"os"
    10		"path"
    11		"path/filepath"
    12		"slices"
    13		"strings"
    14	)
    15	
    16	// Discovery scans bundled, user, and workspace extension roots.
    17	type Discovery struct {
    18		WorkspaceRoot   string
    19		HomeDir         string
    20		IncludeDisabled bool
    21		Enablement      *EnablementStore
    22		BundledFS       fs.FS
    23	}
    24	
    25	// DiscoveryResult captures raw discovered entries plus the effective set after
    26	// precedence resolution and enablement filtering.
    27	type DiscoveryResult struct {
    28		Discovered []DiscoveredExtension
    29		Extensions []DiscoveredExtension
    30		Overrides  []OverrideRecord
    31		Failures   []DiscoveryFailure
    32		Providers  DeclaredProviders
    33		SkillPacks DeclaredSkillPacks
    34	}
    35	
    36	// DiscoveredExtension describes one discovered manifest declaration.
    37	type DiscoveredExtension struct {
    38		Ref          Ref
    39		Manifest     *Manifest
    40		ExtensionDir string
    41		ManifestPath string
    42		Enabled      bool
    43	
    44		rootFS   fs.FS
    45		fsBase   string
    46		diskRoot string
    47	}
    48	
    49	// OverrideRecord describes which higher-precedence declaration won for one name.
    50	type OverrideRecord struct {
    51		Name   string
    52		Winner OverrideSubject
    53		Loser  OverrideSubject
    54		Reason string
    55	}
    56	
    57	// OverrideSubject captures one declaration participating in precedence.
    58	type OverrideSubject struct {
    59		Source       Source
    60		ManifestPath string
    61		Version      string
    62	}
    63	
    64	// DiscoveryFailure reports a manifest load failure encountered during scanning.
    65	type DiscoveryFailure struct {
    66		Source       Source
    67		ExtensionDir string
    68		ManifestPath string
    69		Err          error
    70	}
    71	
    72	func (f DiscoveryFailure) Error() string {
    73		if f.Err == nil {
    74			return "extension discovery failure"
    75		}
    76	
    77		return fmt.Sprintf(
    78			"discover %s extension at %q: %v",
    79			f.Source,
    80			f.ExtensionDir,
    81			f.Err,
    82		)
    83	}
    84	
    85	func (f DiscoveryFailure) Unwrap() error {
    86		return f.Err
    87	}
    88	
    89	// Discover scans the three extension levels, resolves precedence, and returns
    90	// the effective declarations for the configured enablement view.
    91	func (d Discovery) Discover(ctx context.Context) (DiscoveryResult, error) {
    92		if err := contextError(ctx, "discover extensions"); err != nil {
    93			return DiscoveryResult{}, err
    94		}
    95	
    96		store, homeDir, err := d.resolveEnablementStore(ctx)
    97		if err != nil {
    98			return DiscoveryResult{}, err
    99		}
   100	
   101		workspaceRoot, err := d.resolveWorkspaceRoot()
   102		if err != nil {
   103			return DiscoveryResult{}, err
   104		}
   105	
   106		discovered, failures, err := d.scanDiscovered(ctx, store, homeDir, workspaceRoot)
   107		if err != nil {
   108			return DiscoveryResult{}, err
   109		}
   110	
   111		effective, overrides := resolveEffectiveExtensions(discovered)
   112		filtered := filterEffectiveExtensions(effective, d.IncludeDisabled)
   113	
   114		result := DiscoveryResult{
   115			Discovered: discovered,
   116			Extensions: filtered,
   117			Overrides:  overrides,
   118			Failures:   failures,
   119		}
   120		result.Providers = ExtractDeclaredProviders(result.Extensions)
   121		result.SkillPacks = ExtractDeclaredSkillPacks(result.Extensions)
   122	
   123		return result, nil
   124	}
   125	
   126	func (d Discovery) resolveEnablementStore(ctx context.Context) (*EnablementStore, string, error) {
   127		if d.Enablement != nil {
   128			return d.Enablement, d.Enablement.homeDir, nil
   129		}
   130	
   131		store, err := NewEnablementStore(ctx, d.HomeDir)
   132		if err != nil {
   133			return nil, "", fmt.Errorf("create discovery enablement store: %w", err)
   134		}
   135		return store, store.homeDir, nil
   136	}
   137	
   138	func (d Discovery) resolveWorkspaceRoot() (string, error) {
   139		trimmed := strings.TrimSpace(d.WorkspaceRoot)
   140		if trimmed == "" {
   141			return "", nil
   142		}
   143	
   144		root, err := normalizeWorkspaceRoot(trimmed)
   145		if err != nil {
   146			return "", err
   147		}
   148		return root, nil
   149	}
   150	
   151	func (d Discovery) scanDiscovered(
   152		ctx context.Context,
   153		store *EnablementStore,
   154		homeDir string,
   155		workspaceRoot string,
   156	) ([]DiscoveredExtension, []DiscoveryFailure, error) {
   157		discovered := make([]DiscoveredExtension, 0)
   158		failures := make([]DiscoveryFailure, 0)
   159	
   160		discovered, failures, err := d.scanBundled(ctx, store, discovered, failures)
   161		if err != nil {
   162			return nil, nil, err
   163		}
   164	
   165		userRoot := filepath.Join(homeDir, ".compozy", "extensions")
   166		discovered, failures, err = d.scanFilesystemRoot(
   167			ctx,
   168			store,
   169			SourceUser,
   170			userRoot,
   171			"",
   172			discovered,
   173			failures,
   174		)
   175		if err != nil {
   176			return nil, nil, err
   177		}
   178	
   179		if workspaceRoot != "" {
   180			workspaceExtensionsRoot := filepath.Join(workspaceRoot, ".compozy", "extensions")
   181			discovered, failures, err = d.scanFilesystemRoot(
   182				ctx,
   183				store,
   184				SourceWorkspace,
   185				workspaceExtensionsRoot,
   186				workspaceRoot,
   187				discovered,
   188				failures,
   189			)
   190			if err != nil {
   191				return nil, nil, err
   192			}
   193		}
   194	
   195		slices.SortFunc(discovered, compareDiscoveredBySource)
   196		return discovered, failures, nil
   197	}
   198	
   199	func (d Discovery) scanBundled(
   200		ctx context.Context,
   201		store *EnablementStore,
   202		discovered []DiscoveredExtension,
   203		failures []DiscoveryFailure,
   204	) ([]DiscoveredExtension, []DiscoveryFailure, error) {
   205		bundledFS := d.BundledFS
   206		if bundledFS == nil {
   207			bundledFS = defaultBundledExtensionsFS()
   208		}
   209	
   210		entries, err := fs.ReadDir(bundledFS, ".")
   211		if err != nil {
   212			return nil, nil, fmt.Errorf("read bundled extensions root: %w", err)
   213		}
   214	
   215		for _, entry := range entries {
   216			if err := contextError(ctx, "scan bundled extensions"); err != nil {
   217				return nil, nil, err
   218			}
   219			if !entry.IsDir() {
   220				continue
   221			}
   222	
   223			dirName := entry.Name()
   224			virtualDir := path.Join(bundledExtensionsDir, dirName)
   225			manifest, manifestPath, loadErr := loadManifestFromFS(ctx, bundledFS, dirName)
   226			if loadErr != nil {
   227				var notFoundErr *ManifestNotFoundError
   228				if errors.As(loadErr, &notFoundErr) {
   229					slog.Warn(
   230						"ignore bundled extension directory without manifest",
   231						slog.String("source", string(SourceBundled)),
   232						slog.String("extension_dir", virtualDir),
   233					)
   234					continue
   235				}
   236	
   237				failures = append(
   238					failures,
   239					logDiscoveryFailure(
   240						SourceBundled,
   241						virtualDir,
   242						manifestPathForFSDirectory(bundledFS, dirName, virtualDir),
   243						loadErr,
   244					),
   245				)
   246				continue
   247			}
   248	
   249			enabled, err := store.Enabled(ctx, Ref{Name: manifest.Extension.Name, Source: SourceBundled})
   250			if err != nil {
   251				return nil, nil, fmt.Errorf("resolve bundled extension enablement: %w", err)
   252			}
   253	
   254			discovered = append(discovered, DiscoveredExtension{
   255				Ref: Ref{
   256					Name:   manifest.Extension.Name,
   257					Source: SourceBundled,
   258				},
   259				Manifest:     manifest,
   260				ExtensionDir: virtualDir,
   261				ManifestPath: path.Join(bundledExtensionsDir, manifestPath),
   262				Enabled:      enabled,
   263				rootFS:       bundledFS,
   264				fsBase:       dirName,
   265			})
   266		}
   267	
   268		return discovered, failures, nil
   269	}
   270	
   271	func (d Discovery) scanFilesystemRoot(
   272		ctx context.Context,
   273		store *EnablementStore,
   274		source Source,
   275		root string,
   276		workspaceRoot string,
   277		discovered []DiscoveredExtension,
   278		failures []DiscoveryFailure,
   279	) ([]DiscoveredExtension, []DiscoveryFailure, error) {
   280		entries, err := os.ReadDir(root)
   281		if err != nil {
   282			if errors.Is(err, os.ErrNotExist) {
   283				return discovered, failures, nil
   284			}
   285			return nil, nil, fmt.Errorf("read %s extensions root %q: %w", source, root, err)
   286		}
   287	
   288		for _, entry := range entries {
   289			if err := contextError(ctx, "scan filesystem extensions"); err != nil {
   290				return nil, nil, err
   291			}
   292			if !entry.IsDir() {
   293				continue
   294			}
   295	
   296			dirPath := filepath.Join(root, entry.Name())
   297			manifest, loadErr := LoadManifest(ctx, dirPath)
   298			if loadErr != nil {
   299				var notFoundErr *ManifestNotFoundError
   300				if errors.As(loadErr, &notFoundErr) {
   301					slog.Warn(
   302						"ignore extension directory without manifest",
   303						slog.String("source", string(source)),
   304						slog.String("extension_dir", dirPath),
   305					)
   306					continue
   307				}
   308	
   309				failures = append(
   310					failures,
   311					logDiscoveryFailure(source, dirPath, manifestPathForDirectory(dirPath), loadErr),
   312				)
   313				continue
   314			}
   315	
   316			ref := Ref{
   317				Name:          manifest.Extension.Name,
   318				Source:        source,
   319				WorkspaceRoot: workspaceRoot,
   320			}
   321			enabled, err := store.Enabled(ctx, ref)
   322			if err != nil {
   323				return nil, nil, fmt.Errorf("resolve %s extension enablement: %w", source, err)
   324			}
   325	
   326			discovered = append(discovered, DiscoveredExtension{
   327				Ref:          ref,
   328				Manifest:     manifest,
   329				ExtensionDir: dirPath,
   330				ManifestPath: manifestPathForDirectory(dirPath),
   331				Enabled:      enabled,
   332				diskRoot:     dirPath,
   333			})
   334		}
   335	
   336		return discovered, failures, nil
   337	}
   338	
   339	func compareDiscoveredBySource(left, right DiscoveredExtension) int {
   340		if diff := sourceRank(left.Ref.Source) - sourceRank(right.Ref.Source); diff != 0 {
   341			return diff
   342		}
   343		if diff := strings.Compare(left.Ref.Name, right.Ref.Name); diff != 0 {
   344			return diff
   345		}
   346		return strings.Compare(left.ManifestPath, right.ManifestPath)
   347	}
   348	
   349	func resolveEffectiveExtensions(discovered []DiscoveredExtension) ([]DiscoveredExtension, []OverrideRecord) {
   350		if len(discovered) == 0 {
   351			return nil, nil
   352		}
   353	
   354		grouped := make(map[string][]DiscoveredExtension)
   355		keys := make([]string, 0)
   356		for i := range discovered {
   357			entry := discovered[i]
   358			key := strings.ToLower(strings.TrimSpace(entry.Ref.Name))
   359			if _, ok := grouped[key]; !ok {
   360				keys = append(keys, key)
   361			}
   362			grouped[key] = append(grouped[key], entry)
   363		}
   364		slices.Sort(keys)
   365	
   366		effective := make([]DiscoveredExtension, 0, len(keys))
   367		overrides := make([]OverrideRecord, 0)
   368		for _, key := range keys {
   369			group := append([]DiscoveredExtension(nil), grouped[key]...)
   370			slices.SortFunc(group, compareByPrecedence)
   371	
   372			winner := group[0]
   373			effective = append(effective, winner)
   374			for i := 1; i < len(group); i++ {
   375				loser := group[i]
   376				overrides = append(overrides, OverrideRecord{
   377					Name: winner.Ref.Name,
   378					Winner: OverrideSubject{
   379						Source:       winner.Ref.Source,
   380						ManifestPath: winner.ManifestPath,
   381						Version:      winner.Manifest.Extension.Version,
   382					},
   383					Loser: OverrideSubject{
   384						Source:       loser.Ref.Source,
   385						ManifestPath: loser.ManifestPath,
   386						Version:      loser.Manifest.Extension.Version,
   387					},
   388					Reason: "higher_precedence_source",
   389				})
   390			}
   391		}
   392	
   393		return effective, overrides
   394	}
   395	
   396	func compareByPrecedence(left, right DiscoveredExtension) int {
   397		if diff := sourceRank(right.Ref.Source) - sourceRank(left.Ref.Source); diff != 0 {
   398			return diff
   399		}
   400		return strings.Compare(left.ManifestPath, right.ManifestPath)
   401	}
   402	
   403	func filterEffectiveExtensions(entries []DiscoveredExtension, includeDisabled bool) []DiscoveredExtension {
   404		if len(entries) == 0 {
   405			return nil
   406		}
   407		if includeDisabled {
   408			return append([]DiscoveredExtension(nil), entries...)
   409		}
   410	
   411		filtered := make([]DiscoveredExtension, 0, len(entries))
   412		for i := range entries {
   413			if entries[i].Enabled {
   414				filtered = append(filtered, entries[i])
   415			}
   416		}
   417		return filtered
   418	}
   419	
   420	func sourceRank(source Source) int {
   421		switch source {
   422		case SourceBundled:
   423			return 0
   424		case SourceUser:
   425			return 1
   426		case SourceWorkspace:
   427			return 2
   428		default:
   429			return -1
   430		}
   431	}
   432	
   433	func manifestPathForDirectory(dir string) string {
   434		tomlPath := filepath.Join(dir, ManifestFileNameTOML)
   435		if _, err := os.Stat(tomlPath); err == nil {
   436			return tomlPath
   437		}
   438		return filepath.Join(dir, ManifestFileNameJSON)
   439	}
   440	
   441	func manifestPathForFSDirectory(root fs.FS, dir string, virtualDir string) string {
   442		tomlPath := path.Join(dir, ManifestFileNameTOML)
   443		if _, err := fs.Stat(root, tomlPath); err == nil {
   444			return path.Join(virtualDir, ManifestFileNameTOML)
   445		}
   446		return path.Join(virtualDir, ManifestFileNameJSON)
   447	}
   448	
   449	func logDiscoveryFailure(source Source, extensionDir, manifestPath string, err error) DiscoveryFailure {
   450		failure := DiscoveryFailure{
   451			Source:       source,
   452			ExtensionDir: extensionDir,
   453			ManifestPath: manifestPath,
   454			Err:          err,
   455		}
   456	
   457		slog.Error(
   458			"extension discovery failed",
   459			slog.String("source", string(source)),
   460			slog.String("extension_dir", extensionDir),
   461			slog.String("manifest_path", manifestPath),
   462			slog.String("error", err.Error()),
   463		)
   464	
   465		return failure
   466	}
   467	
   468	func loadManifestFromFS(ctx context.Context, root fs.FS, dir string) (*Manifest, string, error) {
   469		if err := contextError(ctx, "load extension manifest"); err != nil {
   470			return nil, "", err
   471		}
   472		if root == nil {
   473			return nil, "", fmt.Errorf("load extension manifest: filesystem is nil")
   474		}
   475	
   476		resolvedDir := strings.Trim(strings.TrimSpace(dir), "/")
   477		if resolvedDir == "" {
   478			return nil, "", fmt.Errorf("load extension manifest: directory is empty")
   479		}
   480	
   481		tomlPath := path.Join(resolvedDir, ManifestFileNameTOML)
   482		jsonPath := path.Join(resolvedDir, ManifestFileNameJSON)
   483	
   484		if _, err := fs.Stat(root, tomlPath); err == nil {
   485			manifest, loadErr := loadManifestFileFromFS(ctx, root, tomlPath, manifestFormatTOML)
   486			if loadErr != nil {
   487				return nil, "", loadErr
   488			}
   489			if _, err := fs.Stat(root, jsonPath); err == nil {
   490				slog.Warn(
   491					"extension.toml takes precedence over extension.json",
   492					slog.String("dir", resolvedDir),
   493					slog.String("manifest_path", tomlPath),
   494					slog.String("ignored_manifest_path", jsonPath),
   495				)
   496			}
   497			return manifest, tomlPath, nil
   498		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
   499			return nil, "", fmt.Errorf("stat extension manifest %q: %w", tomlPath, err)
   500		}
   501	
   502		if _, err := fs.Stat(root, jsonPath); err == nil {
   503			manifest, loadErr := loadManifestFileFromFS(ctx, root, jsonPath, manifestFormatJSON)
   504			return manifest, jsonPath, loadErr
   505		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
   506			return nil, "", fmt.Errorf("stat extension manifest %q: %w", jsonPath, err)
   507		}
   508	
   509		return nil, "", &ManifestNotFoundError{
   510			Dir:            resolvedDir,
   511			CandidatePaths: []string{tomlPath, jsonPath},
   512		}
   513	}
   514	
   515	func loadManifestFileFromFS(
   516		ctx context.Context,
   517		root fs.FS,
   518		filePath string,
   519		format ManifestFormat,
   520	) (*Manifest, error) {
   521		if err := contextError(ctx, "load extension manifest file"); err != nil {
   522			return nil, err
   523		}
   524	
   525		data, err := fs.ReadFile(root, filePath)
   526		if err != nil {
   527			return nil, fmt.Errorf("read extension manifest %q: %w", filePath, err)
   528		}
   529	
   530		raw, err := decodeRawManifest(data, format)
   531		if err != nil {
   532			return nil, &ManifestDecodeError{Path: filePath, Format: format, Err: err}
   533		}
   534		if err := raw.validatePresence(); err != nil {
   535			return nil, &ManifestValidationError{Path: filePath, Err: err}
   536		}
   537	
   538		manifest := raw.toManifest()
   539		if err := ValidateManifest(ctx, manifest); err != nil {
   540			return nil, &ManifestValidationError{Path: filePath, Err: err}
   541		}
   542	
   543		return manifest, nil
   544	}
```

---



</details>

**Don't parse overlay commands with `strings.Fields`.**

This drops quoting and escapes, so commands like `"/Applications/Visual Studio Code.app/.../code" --stdio` are split into the wrong executable and args. The same issue applies to `fixed_args` and `probe_args` in the metadata, so overlays with spaced paths or multi-word arguments become unusable.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_overlay.go` around lines 158 - 171,
splitOverlayCommand and parseOverlayArgs currently use strings.Fields which
loses quoting/escape semantics and breaks paths/args with spaces; replace both
with a shell-aware splitter (e.g., shlex/shellwords) that preserves quoted
strings and escapes so an executable like "/Applications/Visual Studio
Code.app/.../code" and its flags remain intact. Update splitOverlayCommand to
call the new shell-aware split function (instead of strings.Fields) and return
the first element as the command and the rest as args, and change
parseOverlayArgs to return the shell-aware split result (or nil for empty
input). Also ensure the same parser is used when reading fixed_args and
probe_args from metadata so overlays with spaced paths or quoted multi-word args
work correctly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:388c7721-ae35-4ddb-a0e7-78fbd9aa7a58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - `splitOverlayCommand` and `parseOverlayArgs` use `strings.Fields`, which breaks quoted executable paths and quoted arguments such as `"My Tool/bin/tool"` or `"two words"`.
  - Root cause: overlay command parsing is whitespace-based rather than shell-aware, so it cannot preserve quoting semantics from declarative provider metadata.
  - Implemented fix: replaced the current field splitting with a shell-aware argument parser and added regression tests for quoted command and metadata-argument inputs.
