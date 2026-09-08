---
name: systematic-debugging
description: Investigate a failure with an unclear cause, repeated unsuccessful fixes, or a cross-component boundary. Use existing reproduction evidence for a localized bug; not a mandatory multi-stage ritual for every edit.
---

# Diagnose Before Repair

For a clear localized failure, use the short path: read the diagnostic and relevant change, reproduce or reuse its evidence, locate the owning cause, repair it, and verify the symptom in the existing suite/probe.

Expand investigation when the evidence does not explain the failure:

- Trace the bad value or lifetime to its source; compare relevant working code and contracts. `root-cause-tracing.md` describes backward tracing when the call chain is unclear.
- Instrument only the unresolved boundary, using safe summaries rather than dumping credentials or whole environments. Gather enough evidence to distinguish hypotheses.
- Test a concrete causal hypothesis with the smallest useful experiment. After a failed attempt, update the hypothesis from the result; an arbitrary attempt count does not itself require an architecture meeting.
- Use `condition-based-waiting.md` for readiness/timing problems. `defense-in-depth.md` applies when independently trusted boundaries need validation, not as a reason to duplicate checks throughout domain code.

Add a focused regression where no existing check owns the invariant. A failing production regression is fixed in production; an invalid test needs contract evidence before its expectation changes. Preserve required compatibility adapters and avoid unrelated cleanup.

Run the affected check and required project gates. Reuse evidence for unchanged inputs; widen only for relevant changes, failures, or unresolved risk. If reproduction is unavailable, state the evidence and uncertainty and continue useful investigation. Ask only for a decision or information that actually blocks progress.

For external defects, isolate the necessary handling and its removal condition with `no-workarounds`. Do not claim a causal fix or completion beyond the available evidence.
