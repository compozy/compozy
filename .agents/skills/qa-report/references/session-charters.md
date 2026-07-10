# Session Charters

A charter is a written mission for one QA session: *"for the next 60 minutes, this persona walks this journey through this lens."* The charter is the **atomic unit of QA planning** — sessions are what get planned, run, and counted. Test cases don't accumulate; sessions get walked.

> "Exploratory testing is not ad hoc testing. Skilled exploratory testers use charters, time-boxes, and structured note-taking." — TestCollab

## Contents

- Charter anatomy
- Charter file format
- Charter modes
- Time-box guidance
- Cycle planning and cadence tiers
- The coverage inversion
- The debrief is mandatory
- Anti-patterns

## Charter anatomy

Every charter has six parts:

1. **Mission** — one sentence: what to investigate and why it matters.
2. **Persona** — who you are while testing (`<qa-docs-path>/personas.md`).
3. **Journey** — which journey (`J-NN`) the session walks.
4. **Tour** — the thematic lens driving off-script exploration. The canonical tour catalog lives in the `qa-execution` skill (routed from its SKILL.md). Exactly one; mixing dilutes findings.
5. **Time-box** — a hard ceiling: 30 / 60 / 90 minutes.
6. **Scenarios in scope** — the `state.csv` ids this session can settle.

## Charter file format

One file per charter at `<qa-docs-path>/charters/CH-<NNN>.md` (template: `<qa-docs-path>/templates/charter.md`, seed: `assets/charter-template.md`). `CH-NNN` ids are global and monotonic like every other id in the tree. Charters are durable: a charter written for one cycle is re-run in later cycles (smoke/regression cadence) with a fresh debrief appended per run.

## Charter modes

Pick the mode before writing the mission:

- **Charter-with-tour (recommended default)** — mission constrained by one tour. *"As Marina (Mobile User) on slow 3G, run the Back-Button Tour through checkout — pressing back at every step must do something sensible."*
- **Freestyle** — mission + persona + time-box only; improvise from what you observe. Use for brand-new surfaces where edges aren't understood yet.
- **Scenario-based** — walk a realistic end-to-end story in plain language. *"As Rui (Casual User) who got an email coupon: click through, sign in to a 6-month-dormant account, apply the coupon, complete a purchase."* Use to validate a journey under realistic conditions.
- **Strategy-based** — apply one technique throughout (error guessing, state-transition probing). Use for input-heavy or stateful surfaces.
- **Collaborative** — two personas pair on one surface; one drives, one proposes. Use for cross-domain features.

## Time-box guidance

| Box | When |
|---|---|
| 30 min | Narrow scope, smoke re-walk, sanity check. Expect 2-5 findings. |
| 60 min | Default for charter-with-tour. Best depth-vs-fatigue balance. |
| 90 min | Complex multi-step journey or first pass on a new area. Break in the middle. |

**Hard rule:** the box ends when it ends. Findings piling up at the deadline become a follow-up charter, not an extension. Fatigue produces false positives.

## Cycle planning and cadence tiers

A **cycle** is one planned batch of sessions (a branch/PR pass, a release pass, a periodic re-walk). Pick the tier first; the tier picks the journeys:

| Tier | Scope | Sessions | When |
|---|---|---|---|
| **Smoke** | 2-4 highest-value journeys, happy path + true end state | 30-min charters | After any deploy; daily cadence on active products |
| **Targeted** | Journeys touched by the diff + 1 adjacent journey as canary | 30-60-min charters | Every branch/PR with user-visible change |
| **Full** | All P0+P1 journeys, every project persona covered | 60-90-min charters | Release candidates, migrations, big refactors |
| **Sanity** | The fixed journey + 1 adjacent | 30-min charter | After a hotfix |

Order sessions by risk: highest-impact journey × highest-blast-radius tour first — run the fragile combinations while attention is fresh.

## The coverage inversion

The completeness question is **"was every journey in scope walked by a persona this cycle?"** — a session ledger, not a case count.

- Wrong: "every persona has at least one test case", "every CFR category has a TC". That instinct produces artifact accumulation and zero confidence.
- Right: every in-scope journey has ≥1 charter; every charter has a persona, one tour, and a time-box; every run charter has a debrief; every scenario it settles has its `state.csv` verdict updated.

A cycle with 5 deep sessions that walked 5 journeys end-to-end beats a cycle that generated 40 test-case files and walked nothing.

## The debrief is mandatory

A session without a debrief is wasted exploration. When the box ends (appended to the charter file per run by `qa-execution`):

1. Stop the timer — no "just one more thing".
2. Write findings within 5 minutes, before surprises normalize.
3. File bugs via the global bug registry (routed at Step 6 of the SKILL) — dedup first.
4. Update the settled scenarios in `state.csv`.
5. Suggest the next charter: what did this session not reach?
6. Note candidate tours: an improvised pattern that found something is a catalog candidate.

## Anti-patterns

- **No mission** — a charter without a one-sentence mission is tab-clicking.
- **Multi-tour charter** — one tour per box; a second theme is a second charter.
- **Drift outside the journey** — interesting bugs elsewhere go to a follow-up charter, not into this debrief's scope.
- **Charter-per-round duplication** — re-drafting the same mission each cycle instead of re-running the existing charter with a new debrief. Charters are durable; debriefs are per-run.
- **Skipping the debrief** — finding bugs without recording them equals not finding them.
- **Case-count completeness** — see the coverage inversion. If the plan's success metric counts files, the plan is wrong.
