# Skill improvement guidelines

## Method

1. Cluster the findings by root cause, across scorers and classifications, after attribution.
2. Prioritize clusters by frequency times severity.
3. Verify each finding against the current repository and the agent's configuration before proposing any improvements. Drop what does not verify.
4. You are editing another agent's instructions. Keep those edits small and general. Before editing, state the intended behavioral rule and owning surface in one sentence, then make the smallest change that expresses it.
5. Prefer **replacing** existing guidance over **appending** another paragraph.

## When to propose changes

Do not propose changes by default. Proceed only when a concrete instruction is missing or wrong and amending it would have prevented the scored failure. Ask: would a competent agent with the current instructions still be expected to fail this way? If yes, there is a gap. If no, defer.

File only when all of these are true:

- The failure is caused by a missing, wrong, or underspecified instruction on a concrete surface: the owning actor's configuration, a skill, or in-repo guidance.
- You can name that owning surface and the one reusable rule it should have stated.
- If that rule had been present and followed, the scored failure would not have happened.
- The same gap appears in more than one source run, or is severe enough that a single occurrence still proves a missing contract.

Do not file when:

- The existing instruction already required the correct behavior and the model ignored it
- The failure is model variance: same prompt, same tools, different choice
- The only available edit is restating, hedging, or adding examples from these runs
- The real fix is product, infra, scorer, or code outside instruction surfaces

When nothing clears this bar, open no change and say, per finding, why not — that is a success. A speculative change is worse than none.



