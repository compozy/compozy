# Instruction Improvement Evidence

1. Group avoidable cost and defects by cause, using both failed runs and successful controls. Prioritize repeated or severe effects.
2. Check current files and user decisions. An already-fixed historical failure, intentional trade-off, or product/tool bug does not justify another prompt rule.
3. Identify the owning instruction and the behavior that would change. Consider removal, narrower triggers, merged ownership, fewer mandatory reads/reports, and helper fixes alongside missing contracts.
4. Prefer replacing or deleting guidance over adding it. Preserve concrete safety/product invariants; distinguish the necessary contract from one historical way of checking it.
5. Validate changes at the affected scope. Behavioral comparisons should keep model/harness/task class comparable; report uncertainty when only static review or a small historical sample exists.

A useful finding names evidence, ownership, and the expected effect. Skill invocation counts alone prove neither value nor harm. If the model ignored a correct instruction, examine conflicting context and trigger cost before restating it. If nothing warrants a change, say so briefly; no mandatory per-finding disposition file is needed.
