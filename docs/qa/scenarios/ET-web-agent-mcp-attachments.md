---
id: ET-web-agent-mcp-attachments
area: ET
title: Attach configured and new MCP servers from an agent's Tools sheet
persona: Dora
journey: J-31
expected: Editable agents attach configured servers by name or new connections through the shared MCP editor. Saves write only the agent sidecar, retain existing frontmatter and sidecar entries, keep allowed tools unchanged, and store credentials in Vault. Removed configured targets show Broken reference. Package-owned agents are read-only.
entry_points: Agents → agent profile → Tools → MCP servers → Add MCP server
qa_status: pass
bug_ids: ""
fix_status: fixed
retest_status: verified
fix_commits: ""
evidence: docs/qa/reports/2026-11-05-agent-mcp-attachments.md
last_report: docs/qa/reports/2026-11-05-agent-mcp-attachments.md
overlaps: MS-web-agent-permissions, MS-web-mcp-editor-simple-advanced
---

1. Bootstrap an isolated targeted API/CLI/Web lab. Create an editable agent with an existing frontmatter server and configure a manual MCP server.
2. Open its Tools sheet, choose Add MCP server, pick the configured server, and Save. Reload and verify the attachment through `agent info -o json` and the HTTP read. The sidecar stores only `{ "reference": true }` under that name. Allowed tools and the old server remain unchanged.
3. Add a new HTTP server through Add new server. Enter a header credential and Save. Reload; verify the server remains attached, the header value exists only in Vault, and agent files and public read responses do not contain plaintext credentials or Vault refs.
4. Change the configured server's endpoint. The attachment follows the current definition. Remove that server and reload: Broken reference is visible; the agent never launches a stale copied definition.
5. Add a stdio server with a Vault environment binding. Verify persistence through CLI and HTTP; repeat an unrelated Tools edit and confirm existing attachments and bindings survive.
6. Open a package/extension-owned agent. The MCP list is visible and the add action is absent; a crafted write is rejected by the existing package ownership boundary.
7. Add two HTTP header rows that differ only in letter case or surrounding whitespace. Add stays disabled until a duplicate is removed. Reject invalid header names before staging. Leave an HTTP header incomplete, switch to stdio, supply its command, and verify that hidden HTTP rows do not block Save.
8. Attempt to attach private, other-workspace or daemon-owned Vault refs to a new agent-local connection: reject them; configured name references and explicitly shared refs remain supported. Managed HTTP/UDS callers must use references rather than plaintext credential values. Invalid header values and overlapping ref/value names are rejected before Vault writes.
9. Submit two concurrent Saves with the same digest: exactly one succeeds; the stale request conflicts without losing the winning addition.
10. Use a stale digest to add a server. The request conflicts without changing files or Vault. Discard an unsaved connection and confirm no Vault write took place.

Adjacent canary: change an allowed-tool entry and confirm it still saves independently of MCP attachments.
