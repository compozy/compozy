# BUG-20261103-link-trigger-stays-on-scope: A link automation can't pick where a stays-on agent runs

- Status: open
- Impact (user-side): Blocks-Completion
- Severity: Medium; Priority: P2
- Persona Affected: Ada
- Journey Step: New automation → When another app calls a link → Tell a stays-on agent (S10)
- Scenarios: TA-web-automation-editor
- Origin: stays-on hardening QA re-walk, 4056dbcc2
- Report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md

## Reproduction

Open New automation and choose When another app calls a link, then Tell a stays-on agent and pick Inbox, an agent with an instance in Northstar Pay Launch HQ. The agent selection sticks (`value: inbox`). Where it runs then shows "inbox isn't running in Global. Start it there, or pick where it runs.", but both of its options ("Choose where…" and "Northstar Pay Launch HQ") are disabled. The automation can't be completed, and the copy asks for an action the user can't take (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/10c-s10-link-scope-disabled.png`).

With When something happens, the same picker enables the project and clears the error (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/10-s10-trigger-target.png`). The CLI also creates a workspace-scoped webhook trigger targeting the same instance. So either link automations should allow a workspace scope for this target, or the Web should explain that a link can't reach a project stays-on agent.
