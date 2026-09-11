# Unpublished article review and publication

## Scope and provenance

Reviewed all eight `draft.md` articles under
`/Users/pedronauck/Dev/courses/pedronauck/second-brain/conteudo/compozyos/articles`.
The initial site contained only `introducing-compozyos.mdx` and
`graph-loop-editor-local-gateway.mdx`; none of the eight drafts had a published counterpart.
Existing posts and source drafts were preserved. Publication edits live in the site.

The user authorized publication and direct commit/push to `main`.
Technical review baseline: `f94f1fc68233627e32274a3adbeb8d956bd4e0b6`.
External documentation checked: September 11, 2026. Before delivery, fast-forwarded to
`0fe6e1651` (release preparation); its changes did not alter the reviewed runtime contracts.

| Source directory | New blog slug | Reader outcome |
| --- | --- | --- |
| `02-cursor-vs-claude-code` | `cursor-vs-claude-code` | Configure and diagnose two ACP launch paths |
| `03-langchain-alternatives-production-ai-agents-2026` | `langchain-alternatives-production-ai-agents` | Identify the application/execution boundary to replace |
| `04-langgraph-alternatives-when-you-need-more` | `langgraph-alternatives` | Evaluate checkpoints, approval, side effects, and migration |
| `05-defining-agent-sessions-compozyos-providers-souls` | `defining-agent-sessions-compozyos` | Author agent/persona files and inspect session lifecycle |
| `06-autonomous-coding-agent-setup-permissions-review` | `autonomous-coding-agent-setup` | Configure policy, bounded spawn, and task-run review |
| `07-orca-vs-openhands-comparing-two-local-first` | `orca-vs-openhands` | Compare actual product boundaries and the OpenHands integration |
| `08-crewai-alternatives-7-frameworks-that-handle` | `crewai-alternatives` | Separate memory, scheduling, and authority |
| `geo-01-what-is-an-os-for-ai-agents` | `what-is-an-os-for-ai-agents` | Understand runtime ownership and research terminology |

Routes are `/blog/<slug>/`. Draft-only planned URLs were shortened before first publication;
no live route was renamed.

## Material corrections

- Applied `writing-tech-post` and its pre-publication checklist. Removed authoring metadata,
  search-ranking anecdotes, keyword-led introductions, and repetitive sales/installation sections.
- Corrected outdated LangChain claims; credited LangGraph persistence, memory, and human
  intervention, LangSmith scheduling/deployment, Temporal Schedules, Inngest retries/triggers,
  and Windmill scheduling. Compared libraries separately from deployment products.
- Credited CrewAI Memory, Flows, and AMP; removed the unsupported seven-framework URL promise.
- Credited Orca scheduling, restoration, and hooks/memory. Distinguished OpenHands' SDK,
  CLI, server, Canvas, and separate beta automation service for Cloud.
- Corrected Claude attribution to the Claude Agent SDK-based ACP adapter; kept Cursor's direct
  ACP entry point. Replaced aging model defaults with catalog inspection.
- Corrected `deny-all`: current ACP requests reject rather than ask. Distinguished direct host
  operations, interactive approvals, filesystem roots, workspace crossing, and sandbox limits.
- Completed review flags and identified sample verdicts/IDs. Explained managed-session identity
  for spawn, default TTL governance, and informational `session new --parent` provenance.
- Preserved active/unbound creation, first-prompt binding, attach-only resume, terminal failure
  limits, and the separation between forensic session history and memory.
- Added language-tagged code, decision tables, logical diagrams, claim-adjacent citations, and
  contextual internal links. No invented measurements, customer stories, or comparison wins.

## Evidence anchors

| Claim family | Implementation and maintained references |
| --- | --- |
| Provider launch/auth | `internal/config/provider_builtin.go`, `provider.go`, `provider_effective.go`; site `agents/providers` |
| Session state | `internal/session/manager_create_prepare.go`, `manager_prompt_runtime.go`, `manager_prompt_contract_test.go`; site `sessions/lifecycle` |
| Definitions/persona | `internal/config/agent_test.go`, `internal/soul/soul.go`; site `configuration/agent-md`, `agents/soul` |
| Tool policy | `internal/acp/permission.go`, `handlers_test.go`; site `sessions/permissions` |
| Workspace crossing | `internal/workspaceaccess/policy.go`; site `sessions/permissions` |
| Spawn | `internal/session/spawn_permissions.go`; site `autonomy/safe-spawn`, generated CLI `spawn` |
| Reviews | Site `autonomy/review-gate`, generated CLI `task/review/submit` |
| Scheduling | `internal/config/automation.go`; site `automation/jobs`, generated CLI `automation/jobs/create` |

External primary sources are linked beside claims in the articles, including the AIOS paper and
the attributed harness-engineering essay. No secrets, private customer information, or unpublished
security incident details were included.

## SEO and impact

Every article has a distinct title/description, date, author, category, tags, H2 structure, and
related links. Existing templates own the single H1, canonical, Article/Breadcrumb JSON-LD,
OpenGraph/Twitter metadata, social-image generation, RSS, and sitemap discovery.

Cross-surface impact: **not applicable — editorial only** for native tools, hooks/extensibility,
configuration contracts, workspace data isolation, and `skills/compozy/`. No behavior or schemas
change. Web/Docs impact is the eight blog routes and their generated discovery surfaces.
Rendered review also exposed unstyled blog tables and indistinguishable prose links. The existing
blog MDX component map now uses semantic table headers, a keyboard-focusable horizontal scroll
region, and the changelog's table/link token conventions. A minimum table width preserves readable
columns on small screens. No dependencies or design tokens were added.
The owning scenario is [ET-compozy-public-brand-navigation](../scenarios/ET-compozy-public-brand-navigation.md);
the publication slice does not reclassify its historical untested surfaces.

## Validation

- Initial Turbo content generation passed.
- All eight articles passed the skill's heuristic linter with zero findings; factual review was
  performed separately.
- Source-level internal-link resolution passed for every authored article link.
- Bash syntax and current source-built CLI help validated 40 distinct command/flag combinations
  without dispatching provider turns or changing operator state.
- `bunx turbo run test typecheck build --filter=./packages/site`: passed (56 test files, 329 tests).
- `npx react-doctor@latest --verbose --diff`: 100/100, no issues in the two changed React files.
- Rendered HTTP checks: all eight routes return 200 with one H1, descriptions, canonicals, Article
  JSON-LD, and PNG social images. Every route is present in the index, RSS, and sitemap.
- `make gate`: passed the affected `js-packages-site` lane with zero warnings/errors;
  `bunx turbo run build --filter=./packages/site`: passed after the final table change.
- Browser validation on the production build: all eight routes at 390px have one H1, valid local
  anchors, and document width equal to viewport width. Tables retain 576px columns inside 358px
  scroll regions. Keyboard ArrowRight moved the focused table region; code-copy feedback reported
  `Copied`, with no browser console errors on the checked article.
- Desktop review confirmed table spacing, visible references, author/date, related reading, and
  the index showing ten articles (eight new plus two preserved).
- Delivery target: direct commit/push to `main`; production deployment is checked after the push.

Provider login/model calls and competitive performance tests are not implied by content validation.
Examples name their prerequisites and placeholder IDs; pseudocode, planning records, and example
verdicts are explicitly labeled.
