---
id: ET-web-mcp-guided-install
area: ET
title: Install and configure an extension with packaged inputs
persona: Bruno
journey: J-marketplace-acquisition
expected: The extension confirmation renders declared typed inputs, preserves the approved artifact digest and explicit trust decision, and recovers a refused update using only missing candidate fields. Stored values and secret references do not appear in responses or logs.
entry_points: /marketplace/$entryId; catalog Install; Installed Update; Install from GitHub or local build
qa_status: untested
bug_ids:
fix_status:
retest_status: untested
fix_commits:
evidence:
last_report:
overlaps: ET-web-marketplace-detail-redesign; ET-agent-plugin-marketplace-install; ET-web-marketplace-installed-management
---

Marketplace task03 replaces the retired MCP installer with the extension input step. The scenario ID
remains the journey record; old MCP routes, manual-MCP Vault import and inline Vault creation are
not supported acquisition paths. Manual MCP settings and stored credentials remain intact.
Tasks09/10 own the live and visual walk; earlier MCP-installer evidence does not verify this flow.

Install Context7 with its optional key empty, Brave Search with its required brave_api_key, and
Supabase with project_ref. Sentry and Linear use OAuth and have no packaged inputs. For the typed
boolean/default case, use the authored durable-input-kit fixture. Check password/text/switch controls,
labels, optional markers, defaults, false values, required gating, keyboard submission and a narrow
window. At 8193 UTF-8 bytes, show "Too long (max 8 KB)" and refuse submission; 8192 bytes pass.

Inspect the mutation: source, ref and expected_digest address the approved package; inputs retain
wire types, optional empty fields are absent, and secrets are not logged. Editing input fields must
not download another preview. Closing or changing the acquisition clears secret drafts. A changed
source requires a fresh listing/confirmation and any new trust consent before another install.

Publish an update with a new required input. The failed update leaves the working package and values
intact, returns extension_inputs_required with input_definitions, and opens the same input step.
Fill only missing fields and retry the same name/profile/workspace and existing consent. If another
field is required on retry, preserve values already submitted. Publication errors keep the step
editable; duplicate confirmations send one request. The installed row reports "Needs configuration"
from missing_inputs. Installation does not start OAuth; explicit Authorize remains a separate action.
