---
name: architectural-analysis
description: Architectural audit that hunts dead code, duplicated functionality, anti-patterns, type confusion, and code smells across a whole codebase. Use when the user asks for architectural analysis, to find dead or unused code, identify duplication, or assess codebase health. Don't use for style/formatting, performance profiling, security audits, or feature-level code review.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Architectural Analysis

Read-only audit of an entire codebase. Report findings only — make no edits. Classification depth for every dimension lives in `references/detection-catalog.md`; each step below names its section — read that section in full before classifying findings in that dimension.

## Steps

### 1. Map the codebase
List directories and count source files, then Glob every source file and build a todo with one item per file. Note the entry points that anchor usage tracing: app entry (`index`/`main`/`app`), API routes/controllers, public `index.ts` exports, CLI entries, tests.
```bash
find . -type d -not -path "*/node_modules/*" -not -path "*/.git/*"
find . -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.jsx" | wc -l
```
**Done when** every source file has a todo entry.

### 2. Detect dead code
For each file in the todo: list its exports, then search every export for imports or usages elsewhere.
```bash
grep -rn "ExportName" . --include="*.ts" --include="*.tsx"
```
Before recording anything as dead, clear it against the "Not dead" list in `references/detection-catalog.md → Dead code` — a symbol used only in tests, loaded dynamically, reached by string/reflection, exposed as public API, or wired as a framework hook counts as USED. Record each finding as `file:line`, category, and confidence per that section, then mark the todo item complete.
**Done when** every todo file's exports are usage-checked and categorized.

### 3. Detect duplication
Surface candidates by similar names, repeated blocks, and competing implementations of one concept.
```bash
grep -rn "function validateEmail" . --include="*.ts"
```
Confirm each candidate group by reading the implementations, then classify and rank it using `references/detection-catalog.md → Duplication`.
**Done when** every candidate group is read and classified.

### 4. Detect anti-patterns
Inspect the largest files and trace import chains for cycles.
```bash
find . -name "*.ts" -exec wc -l {} + | sort -rn | head -20
grep -rn "from.*auth" src/ --include="*.ts"   # who imports a suspected hub
```
Check findings against the full set in `references/detection-catalog.md → Anti-patterns`.
**Done when** each of the largest files is judged and import cycles among entry modules are traced.

### 5. Detect type issues
```bash
grep -rnE ": any|: unknown|as any|as unknown|@ts-ignore|@ts-expect-error" . --include="*.ts" --include="*.tsx"
```
For each hit decide whether a proper type is possible or a real error is being masked; classify per `references/detection-catalog.md → Type issues`.
**Done when** every hit is judged.

### 6. Detect code smells
Sweep for long functions, long parameter lists, complex conditionals, magic numbers/strings, dead commented-out code, and poor naming.
```bash
grep -rnE "^[[:space:]]*//.*(function|class|const)" . --include="*.ts"   # commented-out code
```
Thresholds for each smell are in `references/detection-catalog.md → Code smells`.
**Done when** every smell category has been swept.

### 7. Write the report
Populate `assets/report-template.md` and write it to `.audits/architectural-analysis-[timestamp].md`, filling every placeholder from steps 2–6. Keep every section present; where a count is zero, write "None found" rather than deleting the heading.
**Done when** every placeholder is replaced and every section is present.

### 8. Summarize for the user
Populate `assets/summary-template.md` and emit it inline in chat, linking to the full report at the end.
