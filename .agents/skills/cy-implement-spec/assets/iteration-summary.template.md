# cy-implement-spec — Iteration {{ iteration }} summary

- **Slug:** {{ slug }}
- **Phase entered:** {{ phase_in }} → **Phase exiting:** {{ phase_out }}
- **Action taken:** {{ action }}
- **Outcome:** {{ outcome }}     <!-- completed | partial | blocked -->
- **Criteria:** {{ met }}/{{ total }} met{{ criteria_flipped_this_iteration_or_none }}
- **Memory written:** {{ memory_paths_csv }}
- **State updated:** `.compozy/tasks/{{ slug }}/state.yaml`
- **Verify:** {{ verify_status }} ({{ verify_evidence }})
- **Checkpoint commit:** {{ commit_sha_or_skip_or_none }}     <!-- short SHA, "SKIP: no changes", or "n/a (phase != B/D)" -->
- **Blockers (if any):** {{ blockers_or_none }}
- **Next phase per detect-phase.py:** {{ next_phase }}

<!--
This is a human-filled substitution template. No bundled helper renders
the {{ placeholders }} automatically.

Print this block after every completed iteration. Intermediate failures stay
inside the phase action and do not render this block. On completed non-E
outcomes the agent continues at detect in the same turn; on a proven external
blocker or Phase E it stops. When phase_out == E, the agent ALSO emits the
content of assets/done-signature.txt on a line of its own. The codex-loop
goal-check confirmation prompt scans for these blocks as progress
evidence.
-->
