## status: completed

<task_context>
<domain>testdata</domain>
<type>testing</type>
<scope>golden_files</scope>
<complexity>low</complexity>
<dependencies>cli|config</dependencies>
</task_context>

# Task 13.0: Update Golden Test Files

## Overview

Regenerate golden test files to reflect new mode names (memory/persistent/distributed) and updated configuration defaults.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md)
- **YOU SHOULD ALWAYS** have in mind that this is a greenfield approach - no backwards compatibility required
- **MUST** complete Phase 1 (Core Config) and Phase 2 (Infrastructure) before starting
</critical>

<research>
When you need information about golden file testing:
- Use perplexity to find Go golden file testing patterns
- Use context7 to check testify or similar framework documentation
</research>

<requirements>
- Identify all golden files containing "standalone" mode references
- Update golden files to use "memory" mode
- Regenerate golden files using `UPDATE_GOLDEN=1` flag
- Verify all CLI config commands pass with new golden files
- Ensure golden files reflect correct default mode (memory)
</requirements>

## Subtasks

- [x] 13.1 Identify all golden files in `testdata/` directory
- [x] 13.2 Find golden files containing "standalone" references
- [x] 13.3 Update `config-diagnostics-standalone.golden` → `config-diagnostics-memory.golden`
- [x] 13.4 Update `config-show-mixed.golden` mode references
- [x] 13.5 Update `config-show-standalone.golden` → `config-show-memory.golden`
- [x] 13.6 Regenerate golden files using `UPDATE_GOLDEN=1`
- [x] 13.7 Run CLI config tests to verify golden file accuracy
- [x] 13.8 Update any test code referencing old golden file names

## Implementation Details

### Objective
Update golden test files to reflect new mode system and ensure CLI configuration commands produce correct output.

### Golden Files to Update

**Location:** `testdata/` directory (typically at project root or under `cli/`)

**Files identified from techspec:**
1. `testdata/config-diagnostics-standalone.golden` → Rename and update
2. `testdata/config-show-mixed.golden` → Update mode references
3. `testdata/config-show-standalone.golden` → Rename and update

### Update Process

**Step 1: Find all golden files**
```bash
find . -name "*.golden" -type f | grep -E "(config|mode|standalone)"
```

**Step 2: Manual updates**

Before regenerating, update mode references:
```yaml
# BEFORE:
mode: standalone

# AFTER:
mode: memory
```

**Step 3: Regenerate golden files**
```bash
# Set environment variable to update golden files
UPDATE_GOLDEN=1 go test ./cli/cmd/config/... -v

# Verify changes
git diff testdata/
```

**Step 4: Validate**
```bash
# Run tests without UPDATE_GOLDEN to verify matches
go test ./cli/cmd/config/... -v
```

### File Rename Operations

```bash
# Rename golden files to reflect new mode
mv testdata/config-diagnostics-standalone.golden \
   testdata/config-diagnostics-memory.golden

mv testdata/config-show-standalone.golden \
   testdata/config-show-memory.golden
```

### Test Code Updates

Update test files that reference old golden file names:

```go
// BEFORE:
goldenFile := "testdata/config-show-standalone.golden"

// AFTER:
goldenFile := "testdata/config-show-memory.golden"
```

### Relevant Files

- `testdata/config-diagnostics-standalone.golden` → rename to `config-diagnostics-memory.golden`
- `testdata/config-show-mixed.golden` → update mode references
- `testdata/config-show-standalone.golden` → rename to `config-show-memory.golden`
- Test files in `cli/cmd/config/` that reference golden files

### Dependent Files

- Phase 1: `pkg/config/resolver.go` (mode defaults)
- Phase 2: Infrastructure changes affecting configuration output

## Deliverables

- Renamed golden files with correct mode terminology
- Updated golden file content reflecting new defaults
- Passing CLI config tests with regenerated golden files
- Documentation of golden file regeneration process

## Tests

Validation through CLI tests:

- [x] Run `go test ./cli/cmd/config/... -v` and verify all pass
- [x] Check `config diagnostics` command output matches new golden file
- [x] Check `config show` command output matches updated golden files
- [x] Verify default mode is "memory" in generated output
- [x] Confirm mode validation accepts memory/persistent/distributed
- [x] Check git diff shows expected changes only

## Success Criteria

- All CLI config tests pass with new golden files
- Golden files accurately reflect new mode system
- No references to "standalone" mode remain in golden files
- Default mode is "memory" in all generated configuration output
- Regeneration process is documented for future updates
