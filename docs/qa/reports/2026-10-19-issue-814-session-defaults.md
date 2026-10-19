# 2026-10-19 — Session defaults on General (#814)

Targeted lab (isolated `COMPOZY_HOME`, HTTP port, UDS socket; web dev server proxied to the lab
daemon; operator-home Claude and Codex CLIs). Build: branch `feat/issue-814-settings-defaults-general`.
Scenario: `MS-web-settings-session-defaults`.

## Walk

| Step | Action | Observation | Verdict |
| ---- | ------ | ----------- | ------- |
| A | Config with only `[defaults] agent = "general"`, `provider = "claude"`; `compozy session new` without runtime; one prompt | Runtime resolved Claude's provider default `claude-sonnet-5`, the pre-change resolution. The local Claude CLI does not advertise that ID (`model_unavailable`), an environment catalog gap unrelated to #814. | pass (compat) |
| R | Open `/settings/defaults` | URL replaced by `/settings/general`; Session defaults is the first group | pass |
| B1 | General → Model → search `gpt-6-luna` → pick Codex GPT-6-Luna → reasoning slider to Low → Save | Save bar settles; Restart needed notice; `GET /api/settings/persona` and `config.toml` report `provider = "codex"`, `model = "gpt-6-luna"`, `reasoning_effort = "low"` | pass |
| B2 | Restart daemon; `compozy session new` without runtime; one prompt | `runtime.effective` = codex / gpt-6-luna / low (`transition: initial_bind`); ACP `model.current_value_id = gpt-6-luna` | pass |
| C1 | `compozy config set defaults.provider claude` | `defaults.model` and `defaults.reasoning_effort` removed from the same layer; lifecycle restart-required | pass |
| C2 | `config set defaults.model claude-opus-5`, then re-set the same provider | Model kept | pass |
| S | Screenshots of General in light and dark | Rendered via the real shell | pass |

Evidence: <https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/general-dark.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/general-light.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/03-selector-low.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/05-saved.png>,
<https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/01-redirect-landed.png>.

Teardown: lab teardown reported `clean: true`.

## Session Debriefs

- Not covered: profile-scope save walked only through unit/service tests (`TestPersonaSectionPersistsSessionRuntimeDefaults`), not through a second profile in the lab.
