# L-015: Provider Auth Ownership Must Be Explicit

**Class:** Architecture / Security

## Incident

Native ACP providers were modeled like API-key providers because AGH inferred provider
authentication from `credential_slots`. That made Claude Code, Codex, Gemini CLI, OpenCode,
Hermes, OpenClaw, and similar subprocess providers appear unconfigured unless AGH could resolve a
provider API key, even though those tools already own native login/session state.

The same mistake can happen through transport adapters. The direct `pi` provider uses `pi_acp`, but
Pi itself owns `/login`, OAuth/API-key storage, token refresh, and auth-file lookup. Treating every
`pi_acp` provider as an AGH-bound secret signal hides Pi's native auth store, especially if AGH
sets `PI_CODING_AGENT_DIR` to a per-session runtime directory.

The inverse is also true: wrapped API-key providers such as OpenRouter, z.ai, Moonshot/Kimi,
Vercel AI Gateway, xAI, MiniMax, Mistral, and Groq may use `pi_acp` under the hood while exposing a
provider-key contract to the operator. For those providers, `pi_acp` is an implementation detail and
`bound_secret` is correct because the built-in defines required `credential_slots`.

The same shape also created a security ambiguity: daemon environment secrets could be treated as
normal launch inputs for native CLIs, making it unclear whether the provider's own login state or an
AGH-bound key was authoritative.

## Root cause

Authentication ownership was implicit. A launch-time secret binding was doing two jobs at once:
describing the provider's auth contract and injecting a concrete secret into the child process.
That collapsed native CLI auth and AGH-managed API-key auth into one model.

## Fix / Rule

Provider authentication must declare ownership before secrets are considered:

- `auth_mode = "native_cli"` means the provider CLI owns login/session state, and AGH must not
  require or inject `credential_slots`.
- `auth_mode = "bound_secret"` means AGH owns launch-time secret resolution for that provider and
  injects only configured `credential_slots`.
- `auth_mode = "none"` means AGH launches without native diagnostics or secret injection.

Environment and home policy are part of the same boundary. `env_policy` defines what daemon
environment reaches the child, and `home_policy` defines whether native CLI state comes from the
operator home or an AGH-owned provider home. Do not add compatibility aliases or optional native
slots to bridge the models; use one hard contract and update code, API, web, docs, and tests
together.

Harnesses and adapters are transport choices, not auth ownership. A provider should become
`bound_secret` because it has explicit `credential_slots`, not because it uses `pi_acp`; the direct
`pi` provider stays `native_cli` because it has no AGH credential slot and owns login through Pi.

## Evidence

- Accepted implementation plan: `.codex/plans/2026-05-03-native-acp-auth-isolation.md`
- Provider config contract: `internal/config/provider.go`
- Provider launch boundary: `internal/session/provider_runtime.go`
- Shared env/home policy helper: `internal/providerenv/env.go`
- Operator CLI surface: `internal/cli/provider.go`
- Public docs: `packages/site/content/runtime/core/agents/providers.mdx`
- Runtime config docs: `packages/site/content/runtime/core/configuration/config-toml.mdx`
