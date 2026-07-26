package memory

import "strings"

const consolidationPromptTemplate = `
# Dream Consolidation

You are running a one-shot AGH dream consolidation session.

Your job is to distill durable, cross-session knowledge from recent session
outputs and team-memory artifacts into the persistent memory store. Work in four
phases and stop when the consolidation pass is complete.

## Phase 1: Orient

- Inspect the existing global and workspace MEMORY.md indexes first.
- Read any existing memory files that look relevant before changing them.
- Keep the persistent memory taxonomy intact: user, feedback, project, reference.
- Review how many completed sessions triggered this run and how long it has been since the last consolidation.
- Do not rewrite correct memories just to restate them.

## Phase 2: Gather

- Review recent completed session artifacts, especially session event databases
  for sessions completed since the last consolidation.
- Extract high-signal facts, repeated patterns, stable decisions, user
  corrections, and durable references that would help future sessions.
- Ignore transient debugging chatter, routine tool noise, speculative ideas without evidence, and secrets.

## Phase 3: Consolidate

- Merge new signal into the smallest durable set of memory files.
- Prefer updating or merging existing files over creating near-duplicates.
- Keep frontmatter accurate and descriptions concrete enough for MEMORY.md discovery.
- Convert relative dates into absolute dates so the memories remain understandable later.
- If sources conflict, prefer the newest verified evidence or record the uncertainty explicitly.

## Phase 4: Prune

- Remove duplicate, obsolete, or low-signal memory content.
- Rebuild or tighten MEMORY.md entries for both scopes so each remains concise and discoverable.
- Keep each index under 200 lines and under roughly 25KB.
- Leave the memory store cleaner and easier to navigate than you found it.

## Output Discipline

- Persist only durable knowledge that should survive across sessions.
- Never store secrets, one-off operational noise, or temporary execution steps.
- Make concrete memory-file updates only when justified by the evidence you gathered.
- Finish once the memory store and indexes reflect the best durable synthesis you can produce.
`

// ConsolidationPrompt returns the embedded four-phase consolidation prompt.
func ConsolidationPrompt() string {
	return strings.TrimSpace(consolidationPromptTemplate)
}
