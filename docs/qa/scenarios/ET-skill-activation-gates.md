---
id: ET-skill-activation-gates
area: ET
title: Offer only skills whose runtime activation gates pass
persona: Ada
journey: J-offer-runnable-capabilities
expected: A skill with satisfied metadata.agh.when gates appears in startup and current-turn catalogs; one with an unmet platform, environment, tool, or authored-capability gate remains manageable with structured inactive reasons but is absent from both catalogs; restoring a required callable tool makes the next catalog projection offer it without restarting AGH.
entry_points: SKILL.md metadata.agh.when; agent startup and current-turn prompt catalogs; GET /api/skills; agh skill list|inspect|view; agh__skill_list|view; Web /skills
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-001; ET-002; ET-003
---

Exercise gate-family AND composition, any-of platform/environment matching, all-of tool/capability
matching, and fail-closed unavailable contexts. Confirm unknown `when` keys and malformed values fail
parsing. Verify administrative enabled state remains independent from runtime activation in CLI,
HTTP/UDS, native tools, and Web detail/list views. Explicit skill reads must continue to work for an
inactive skill.

QA impact 2026-07-18: new offer-time skill activation behavior and public inactive diagnostics.
Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Ada and linked to J-offer-runnable-capabilities;
settles the skills half of US-011 (ADR-009 §2) — RT-mcp-dead-recovery owns the dead-entity half.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Advertised-set token assertion on the gated fixture (measured drop) and the agent-prompt
  exclusion capture.
- Inactive-with-reason listing across CLI, HTTP/UDS, native, and Web.
- The revive-without-restart capture after the required tool becomes available, and the
  unknown-`when`-key parse error.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-011-only-runnable-skills-are-offered-dead-sidecars-self-recover
