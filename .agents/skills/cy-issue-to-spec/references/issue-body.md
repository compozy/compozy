# Issue body — skeleton and sources

The body is a summary that sends readers to the documents, not a replacement for them. Everything below is drawn from the spec; nothing is invented at writing time.

## Order

1. **State callout.** `> [!NOTE]` naming where the work stands — planned, in execution, delivered in PR #N — plus any global precondition ("after X merges to `main`"). When the issue was split, say which issue took the other half.

2. **Spec-set link block.** The folder link, then one line of per-document links carrying their size so a reader can judge the depth before clicking:

   > **Full spec set:** [compozy/compozy-specs → YYYY-MM-DD-slug](…)
   > [Spec](…) (Part I product · Part II technical) · [User stories](…) (N) · [DX contract](…) · [UI change map](…) (S1–SN) · [Test contract](…) (N cases) · [ADRs](…)

   Close with one sentence: the summary follows, the documents are the authority.

3. **The problem.** From Part I *Overview*. Lead with what does not work today and what it costs, not with the solution. Where the spec quantifies the gap ("three of ~14 domains"), keep the number.

4. **Goals.** From Part I *Goals*, one line each, keeping the spec's own framing.

5. **Features.** From Part I *Core Features*. A table when they are parallel and short; bolded paragraphs when each needs a sentence of mechanics. Keep the spec's identifiers (F1, US-00N) so a reader can jump to the full text.

6. **Rules that hold everywhere.** The handful of *Business Rules* that shape the design — the ones a reader would otherwise get wrong. Name the count of the full set so the selection reads as a selection.

7. **Architecture.** From Part II *Executive Summary*: the new package, the one authoritative implementation of anything that could drift, the boundaries. Add **delete targets** when the spec names them.

8. **Surfaces.** The agent-manageability triple (CLI / HTTP+UDS / native tool), config keys, events, and the web surface count — from `_dx.md` and `_uiux.md`.

9. **Build order.** The `_tasks.md` table, reduced to task number, phase, and scope. Mark the MVP boundary. Close with the test-contract count and its split by layer.

10. **Non-goals.** From Part I *Non-Goals*, each with the reason it was excluded. A non-goal that reads as a limitation instead of a decision is miswritten — say "excluded by construction" or "deferred, and here is what makes it cheap later" when that is what the spec decided.

11. **Status.** Peer-review rounds and findings incorporated, counts of stories, ADRs, test IDs, and tasks, and what the work is waiting on.

## Rules

- **Plain language.** Everyday words; a technical term that must stay gets a short defining clause on first use.
- **Runtime truth beats the spec.** When the shipped behaviour and the spec disagree, describe what ships and say the spec is historical.
- **Claims match evidence.** Apply `COPY.md` claim standards before "shipped", "supported", or a product count. A provider or platform proven end to end is named; one that is not becomes a stated limitation.
- **Counts are real.** Read them out of the documents (`grep -c` the story or test identifiers) rather than estimating.
- **A closed issue's body describes what shipped**, links the PR, and lists the verification that backs it — gates, e2e lanes, QA verdict, evidence files.
