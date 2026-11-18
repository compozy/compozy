---
title: Smaller fixes
type: fix
---

- **Large skills no longer fail the prompt.** Invoking a skill whose instructions are too large for the turn used to fail the whole prompt with "session prompt dispatch is indeterminate". The agent now gets a pointer to read the skill with `skill_view` instead. Thanks to @madtocc for the fix.
- **Large tool results in User sessions.** Oversized results such as a long `skill_view` are now saved and readable in User sessions, not just in projects. Thanks to @zaptoscompany for the report.
- **Moving a session with stopped sub-agents.** A session whose sub-agents are stopped but not archived can now move to another profile.
- **Add MCP server form.** The form no longer shows "required" errors before you type anything. Errors appear after you leave a field empty or try to save.
