---
date: 2026-08-13
status: pass
scenarios:
  - MS-web-agent-create-simple-advanced
---

# PR #367 agent-name validation re-walk

Fresh isolated lab: `compozy-pr-367-agent-name-validation-rewalk-20260813-20260813-185923-558685`.
The manifest used a dedicated `COMPOZY_HOME`, HTTP port `60233`, UDS socket, tmux socket,
`COMPOZY_WEB_API_PROXY_TARGET=http://127.0.0.1:60233`, native-provider operator home, and
`browser-use`. The worktree build identified itself as `v0.3.0-beta.13-14-ga1b0b331-dirty`.

## Public walk

- The desktop onboarding selected Codex / GPT-5.6 Terra through its native CLI option, then skipped
  workspace setup to Global. The menubar showed Global scope and the create footer said
  `Creates in Global — visible to every workspace.`
- A 107-character name stayed in the Agent name input, showed the inline message ending in
  `at most 106 characters.`, disabled Create, and emitted no `/api/agents` request. Screenshot:
  `/Users/pedronauck/dev/qa-labs/compozy-pr-367-agent-name-validation-rewalk-20260813-20260813-185923-558685-lab/qa-artifacts/qa/evidence/agent-invalid-overlength.png`.
- Replacing it with 106 lowercase `a` characters enabled Create. The browser POST returned 201;
  its body contained a global agent definition and a resolved Codex runtime. The request omitted
  `workspace`, as required by the Global destination contract. Screenshot:
  `/Users/pedronauck/dev/qa-labs/compozy-pr-367-agent-name-validation-rewalk-20260813-20260813-185923-558685-lab/qa-artifacts/qa/evidence/agent-boundary-created.png`.
- Independent public HTTP and UDS reads returned that same global agent and effective Codex runtime.
- The public CLI rejected `compozy agent create "audio designer"` before dispatch. A UDS create with
  107 characters returned HTTP 400 and the exact canonical validation message.

## Network Live

The desktop Start session dialog exposes Network participation. Its first Live attempt correctly
reported `network_channel_unknown` before session creation because the named channel did not exist.
The public HTTP channel-create endpoint then created `boundary-live` with the 106-character agent.
It started `sess-4eeb08c0512083f0` through native Codex in Live mode. The returned peer ID was
exactly 128 characters (`106 + '.' + 21`), the session was `active`, and Network status reported
one local peer and one channel. No `backend_unhealthy` state was observed. The session was stopped
through the public stop route (HTTP 204) before teardown.

Journey log: `/Users/pedronauck/dev/qa-labs/compozy-pr-367-agent-name-validation-rewalk-20260813-20260813-185923-558685-lab/qa-artifacts/qa/journey-log.jsonl`.
The strict audit and literal manifest teardown are recorded with that lab; teardown reports
`clean: true`.
