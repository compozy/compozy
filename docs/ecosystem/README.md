# AGH Ecosystem Opportunity Map

- **Language:** English · [Português (Brasil)](ptbr/README.md)
- **Status:** research reference and product opportunity map, not a committed roadmap
- **Research snapshot:** 2026-07-17
- **Companion catalogs:** [Atomic extensions and integrations](extension-opportunities.md) · [Outcome bundles](bundle-opportunities.md)

## Purpose

This document answers two different ecosystem questions:

1. Which reusable services and integrations should AGH make reachable through atomic extensions?
2. Which complete outcomes should people be able to activate without understanding agents, MCP, hooks, schedules, or runtime configuration?

The distinction is the center of the strategy:

> Extensions make services reachable. Bundles make outcomes repeatable. AGH keeps those outcomes running.

The catalogs deliberately extend beyond software development and operations. They include personal productivity, family administration, local business, sales, support, marketing, creative work, commerce, finance, legal intake, recruiting, education, research, nonprofit work, hospitality, property operations, and other domains where a durable agent system can remove recurring coordination work.

Every candidate is an opportunity, not a claim that the integration exists, that its upstream API is open, or that it has product-market fit. Provider access, commercial terms, regional availability, data sensitivity, and API stability still need validation before implementation.

## Catalog at a glance

| Catalog layer | Count | Treatment |
| --- | ---: | --- |
| Existing in-tree bridge providers | 8 | Baseline only; excluded from new-opportunity totals |
| Platform-enabling investments | 18 | New opportunities that require daemon, registry, SDK, permission, composition, or lifecycle work |
| Atomic service/provider extensions | 193 | New integration candidates across twelve domains |
| Detailed bundle blueprints | 43 | New flagship outcomes with full composition and safety analysis |
| Additional compact bundles | 160 | New long-tail persona, outcome, and vertical candidates |
| **Total new opportunity records** | **414** | **211 platform/extension records plus 203 bundle records** |

These are catalog records, not committed roadmap items or an estimate of implementation effort. Bundle records intentionally refer to atomic records they would compose; the total counts artifacts, not unique upstream vendors.

## Executive recommendation

AGH should expose two primary marketplace doors and one editorial layer:

| Door | User question | Artifact | Primary author |
| --- | --- | --- | --- |
| **Connect a service** | “Can AGH reach the system I already use?” | Atomic extension or provider implementation | Integration developers and vendors |
| **Get an outcome** | “Can AGH run this recurring job for me?” | Outcome, persona, or vertical bundle | Domain experts, operators, consultants, and developers |
| **Choose a starter collection** | “What should someone like me install first?” | Curated collection plus guided starter | AGH maintainers, partners, and community curators |

This creates a larger contributor surface than a developer-only plugin directory. Developers can publish reliable connectors and runtime providers. Domain experts can compose them into useful operating packages without reimplementing authentication or transport. Curators can package trusted starting points for an audience or market.

The first public collection should be small enough to verify rigorously but broad enough to demonstrate that AGH is not only a coding tool:

- Personal Chief of Staff
- Meeting to Action
- Local Business Front Desk
- Sales Call Prep and Follow-up
- Support Triage to Knowledge
- Campaign in a Box
- Content Repurposing Engine
- Ecommerce Daily Operator
- Executive Business Review
- Product Launch Command Center
- Community Manager
- Fix a Linear Issue

## The current AGH model

The opportunity map was derived from the implemented model before competitor research began. The current source of truth is the extension manifest and resource registry under `internal/extension/`, the bundle model under `internal/bundles/`, the public contracts under `internal/api/contract/`, and the operator documentation under `packages/site/content/runtime/core/`.

### Extension responsibility

An extension is the installable, versioned, trust-bearing unit. It can be resource-only or run as a capability-gated subprocess. Its manifest can package static resources, and a running extension can publish supported resources through the host API.

| Extension concern | Current AGH shape |
| --- | --- |
| Lifecycle | Search, list, install, inspect through status/provenance, enable, disable, update, remove |
| Provenance | Source, source tier, trust state, checksum, marketplace metadata |
| Static resources | Skills, loops, agents, bundles, hooks, tools, and MCP servers |
| Service surfaces | Runtime-registered providers such as memory backends, bridge adapters, tool providers, model sources, and loop watch sources |
| Dynamic resources | Hook bindings, tools, agents, MCP servers, skills, automation jobs, automation triggers, and bundle catalog resources |
| Security | Declared method grants and capability grants gate host API access |
| Management | CLI, HTTP/UDS, and native `agh__extensions_*` and `agh__resources_*` tools |

An atomic integration should normally own one reusable responsibility: a service connector, bridge transport, provider implementation, event source, policy hook, migration adapter, document/media provider, or infrastructure adapter. It should not silently install a persona or start recurring work.

### Bundle responsibility

A bundle is an extension-provided catalog with activatable profiles. A profile activation is global or workspace-scoped and currently projects a specific set of runtime resources.

| Bundle profile field | Current activation behavior |
| --- | --- |
| Channels | Declares available network channels and may bind one primary default |
| Agents | Projects activation-owned agents from extension-local agent directories |
| Agent sidecars | Projects optional persona/principles `SOUL.md` and wake/reentry-policy `HEARTBEAT.md` alongside a bundled agent; neither grants operational authority |
| Jobs | Projects activation-owned automation jobs |
| Triggers | Projects activation-owned automation triggers |
| Bridge presets | Projects activation-owned bridge instances in a disabled state; secret slots remain catalog/preview metadata until a separate connection and enablement step |

CLI and HTTP/UDS expose catalog, preview, activate, list, get, update, deactivate, and network-settings operations. Native tools expose only `agh__bundles_list`, `agh__bundles_info`, `agh__bundles_activate`, `agh__bundles_deactivate`, and `agh__bundles_status`; agents use the structured CLI or API fallback for preview, update, and network settings. Preview is non-mutating and exposes the resources that activation would project.

### The composition boundary that must remain explicit

Current bundle profiles do **not** profile-scope skills, loops, hooks, tools, or extension-level MCP servers. Those resources are enabled with their owning extension. A bundle also cannot install another extension or resolve an implementation from a provider contract.

Bundle-declared jobs and triggers currently target a named agent, not a Loop. An authorized target agent can start an installed Loop through `agh__loop_run`; direct job/trigger-to-Loop binding would require a bundle-contract change.

The current-compatible package therefore looks like this:

```text
outcome extension (installed and enabled)
├── extension-scoped static resources
│   ├── skills
│   ├── loops
│   ├── hooks
│   ├── tools
│   └── MCP servers
└── bundle catalog
    └── selected profile activation
        ├── channels
        ├── agents + persona Soul + wake/reentry Heartbeat
        ├── jobs
        ├── triggers
        └── disabled bridge instances from presets
```

This shape lets one self-contained extension prototype an outcome under the current model, or lets a profile activate around integrations an operator has already installed and connected. It does not implement any named showcase package in this catalog. A genuinely plug-and-play community bundle still needs to install cross-extension dependencies, activate only its selected static resources, resolve “one of HubSpot or Salesforce,” and roll the graph back transactionally.

### Fit labels used in the catalogs

| Label | Meaning |
| --- | --- |
| **Current** | The artifact can be represented by one installed owning extension, configured ACP runtime/provider for each packaged agent, and the current profile projection model. |
| **Current with preinstalled dependencies** | Activation is representable, but required service extensions and runtimes must already be installed, configured, authorized, and healthy; static skills, loops, hooks, tools, and MCP servers remain extension-scoped. Bridge presets still materialize disabled. |
| **Platform evolution** | The desired install experience needs dependency resolution, profile-scoped static resources, provider contracts, setup schemas, new resource families, or another runtime change. |

These labels describe packaging fit, not implementation effort or upstream API availability.

## Product taxonomy

### Atomic extension

One versioned, reusable integration or runtime provider. Examples include Google Workspace, HubSpot, document extraction, browser automation, a memory backend, a webhook source, or a voice provider.

### Extension family

Separate provider implementations that satisfy the same user-facing need. Examples include Asana, ClickUp, Jira, Linear, and Monday for work management, or HubSpot, Salesforce, Attio, and Pipedrive for CRM. A future provider contract can let a bundle require the need rather than a vendor.

### Outcome bundle

A package for a bounded job with a visible completion condition, such as converting a meeting into assigned actions, recovering a failed invoice, or fixing a Linear issue through a reviewed pull request.

### Persona bundle

A broader operating profile for a role, such as a personal chief of staff, sales assistant, recruiter, community manager, or creator studio. It normally contains several bounded outcomes and recurring jobs.

### Vertical bundle

A package that combines general integrations with domain constraints, terminology, approvals, and operating cadence for a market such as real estate, ecommerce, nonprofit fundraising, property maintenance, restaurants, or clinic administration.

### Collection and starter

A collection is editorial curation; it should not hide independent package ownership. A starter is a guided setup experience that selects a bundle variant, connects required services, chooses autonomy and delivery settings, runs a demo, and produces the first useful result.

Collections and starters are recommendations in this document, not current AGH runtime artifacts.

## Prioritization model

Each candidate is assigned a directional priority:

| Priority | Meaning |
| --- | --- |
| **P0** | Broad reach or high composition leverage, understandable first value, credible integration path, and a safe initial mode. |
| **P1** | Strong audience or vertical value after the foundation exists; may require more writes, data modeling, or domain-specific review. |
| **P2** | Long-tail, partner-gated, platform-specific, sensitive, experimental, or primarily a differentiating niche. |

Prioritization should be revisited with evidence, using the following dimensions:

1. **Audience reach:** how many distinct personas can use it?
2. **Frequency:** does the job recur daily, weekly, or only rarely?
3. **Visible first value:** can a user recognize success within ten minutes?
4. **Composition leverage:** how many bundles does the atomic extension unlock?
5. **Safe starting mode:** can it begin read-only or draft-first?
6. **Integration viability:** is there a stable official API, MCP server, webhook, CLI, or partner path?
7. **Setup burden:** how many credentials, scopes, paid services, and local dependencies are required?
8. **Operational burden:** rate limits, retries, idempotency, recovery, and ongoing maintenance.
9. **Differentiation:** does the result use AGH channels, jobs, triggers, wake/reentry policies, Loops, and managed agents rather than duplicate a generic chat connector?

### Risk labels

Risk should be visible at listing, preview, activation, and action time:

| Risk | Examples | Default posture |
| --- | --- | --- |
| **R0 — read-only** | Search, summarize, report, monitor | Allow after explicit connection and scope selection |
| **R1 — private reversible write** | Draft, create internal task, add private note | Preview or undo path; configurable approval |
| **R2 — external or public write** | Send email, publish content, contact lead, change customer state | Human approval by default |
| **R3 — high impact** | Payment, refund, purchase, deletion, legal filing, access change, regulated decision | Mandatory explicit approval and narrow policy; some actions should remain unsupported |

Personal, health, financial, legal, employment, and other sensitive-data bundles also need clear scope, retention, egress, and escalation policies even when their actions are read-only.

## Recommended P0 portfolio

### Use the in-tree bridge base

The repository contains first-party bridge implementations for Discord, Google Chat, GitHub, Linear, Slack, Microsoft Teams, Telegram, and WhatsApp. Released `agh` artifacts do not bundle these provider executables or install them automatically; a trusted source checkout builds and installs them explicitly. Even with that distribution caveat, the implementations are a valuable foundation: outcome bundles can meet users in tools they already open every day.

A bridge should not automatically be treated as a complete business connector. Each in-tree bridge implementation should be audited for bidirectional commands, threads, attachments, approvals, delivery receipts, webhook authenticity, identity mapping, rate-limit diagnostics, and the service-specific action surface required by bundles. Missing business operations can be added as a separate tool-provider extension or a deliberate expansion of the existing integration contract.

### New P0 atomic foundations

| Foundation | Why it belongs in the first wave |
| --- | --- |
| Google Workspace | Email, calendar, documents, files, and spreadsheets unlock personal, team, sales, finance, and operations bundles. |
| Microsoft 365 | Outlook, Calendar, OneDrive, SharePoint, and enterprise identity unlock the corresponding business audience. |
| Notion | A common knowledge and lightweight operations surface across many functions. |
| HubSpot | Broad CRM, marketing, and support leverage for small and mid-sized teams. |
| Intercom or Zendesk | A ticket/event foundation for support, success, product feedback, and bug escalation. |
| Meeting intelligence | Transcript-ready events from Zoom, Meet, Teams, Fireflies, Granola, Otter, or a normalized meeting contract. |
| Document intelligence | PDF, image, audio, and office-document extraction with field provenance. |
| Web research and RSS | Cited research, monitoring, competitive intelligence, and content inputs. |
| Browser automation | A constrained fallback for services without a suitable API, never a substitute for a stable connector when one exists. |
| Webhook ingress and egress | A universal bridge to forms, internal systems, CI, Zapier, Make, and n8n. |
| Home Assistant | A self-hosted, read-first gateway for household state, events, and explicitly approved device or scene actions. |
| Shopify | A coherent vertical with orders, inventory, customers, payments, support, and measurable exceptions. |
| Stripe | Payments, invoices, subscriptions, and failure events across many business bundles; read-only by default. |
| Canva or a normalized design asset provider | Makes campaign and creator bundles produce visible, shareable results. |
| Integration discovery | Converts integrations.sh, OpenAPI, MCP, GraphQL, and CLI metadata into reviewed extension candidates. |
| Migration assistants | Import compatible skills, MCP configuration, agents, and profiles from Claude, Codex, Pi, OpenClaw, Hermes, and Paperclip with a resource-by-resource preview. |

### P0 showcase bundles

| Bundle | Audience | First proof of value | Safe starting posture |
| --- | --- | --- | --- |
| Meeting to Action | Any meeting-heavy role | Decisions and task drafts linked to a transcript | Read transcript; approve task and message writes |
| Personal Chief of Staff | Individuals, founders, managers | Morning brief from calendar, inbox, tasks, and reminders | Read-only brief before enabling drafts |
| Local Business Front Desk | Local services and small businesses | Qualified inquiry and proposed appointment | Human handoff and approval before booking or messaging |
| Sales Call Prep and Follow-up | Founders, sellers, account teams | Account brief plus CRM and follow-up drafts | CRM writes and sends require approval |
| Support Triage to Knowledge | Support and small teams | Classified ticket, evidence-backed reply draft, and KB draft | Auto-route only; approve customer-facing reply |
| Campaign in a Box | Marketing generalists and small teams | Brief transformed into destination-ready draft assets | Draft-only until launch approval |
| Content Repurposing Engine | Creators, educators, marketing | One long-form input transformed into a reviewed content set | No automatic publishing |
| Ecommerce Daily Operator | Store owners | Morning exception brief for orders, inventory, refunds, and support | Read-only report; approve mutations |
| Executive Business Review | Founders and leaders | Evidence-linked weekly narrative across core systems | Read-only synthesis |
| Product Launch Command Center | Product, marketing, sales, support | Dependency map and stakeholder-ready status | Internal writes configurable; external publish approved |
| Community Manager | Communities and open-source projects | Unanswered-question queue and weekly community digest | Draft moderation and responses first |
| Fix a Linear Issue | Software teams | Tested change, reviewed pull request, and synchronized issue status | Workspace permissions, CI gates, and PR review required |

## The plug-and-play platform gap

> **Product decision — 2026-07-20 (binding). The ten platform gaps below are `rejected` as core/engine roadmap work — do not implement them.** They were reviewed and pruned; the only accepted new core investment from the platform-enabling catalog is **credential/secret liveness health** (see PE-015, "accepted — health only", in [extension-opportunities.md](extension-opportunities.md#platform-enabling-opportunities)). The accepted product direction is that connectors converge on MCP servers (OAuth handled by the existing MCP layer) or subprocess extensions that ship real code and read API keys from the vault, so no-code import/compilation, a generic OAuth broker, signing/registry/conformance tooling, and bundle composition (dependency resolution, profile-scoped resources, capability contracts) are out of scope. The sections below are retained as historical analysis only.

The current runtime can validate bundle projection mechanics with a self-contained prototype. It cannot make the named showcase packages plug-and-play without explicit platform work.

### 1. Transactional dependency resolution

A bundle should declare required, optional, and one-of dependencies. Installation should resolve versions, verify compatibility and trust, preview every resource and permission, apply atomically, run health checks, and retain rollback metadata. Missing optional dependencies should produce an explicit degraded mode, never a silent skip.

### 2. Profile-scoped static resources

AGH needs either profile-scoped activation for skills, loops, hooks, tools, and MCP servers or a comparably clear composition mechanism. Enabling one outcome profile should not unintentionally expose every static resource in its extension.

### 3. Provider contracts

Bundles should be able to require `work-management`, `crm`, `calendar`, `speech-to-text`, `document-extraction`, or `image-generation` and let the user select a compatible implementation. Contracts must stay concrete enough to guarantee the operations the bundle uses; a generic label without behavioral conformance would only move failure to runtime.

### 4. Connection and setup schemas

Authentication is a primary product surface, not a README step. Each extension should declare credential acquisition, OAuth scopes, tenant/account selection, connection health, rotation and revoke behavior, paid-service dependencies, regional constraints, and whether authentication happens at install or first use.

Each bundle should declare an intent-oriented setup form: stack choices, recipients, schedule, escalation destination, autonomy level, budget, retention, and delivery destination. Technical transport details belong behind progressive disclosure.

### 5. Permission, budget, and recovery contracts

Preview should summarize what the package can read, write, send, publish, purchase, delete, execute, and schedule. Recurring bundles should declare token/cost budget, maximum runs, catch-up behavior, retry policy, idempotency key, failure threshold, circuit breaker, silence/no-op semantics, and escalation path.

### 6. Install is not activation

The lifecycle should remain legible:

```text
discover → inspect → install → enable → connect
         → select optional providers/integrations → preview → activate
```

Imported jobs, triggers, bridge listeners or event subscriptions, and autonomous loops should be paused until the user explicitly activates them. This is a consistent lesson from Hermes profile distributions, OpenClaw permission surfaces, and Paperclip import behavior.

### 7. Demo, dry run, and first-success contract

Every listing should define:

- a fixture-backed demo that sends nothing externally;
- a real-data dry run that performs no external mutation;
- required connection health checks;
- one observable first-success condition, such as “brief generated,” “task draft created,” or “message delivered”;
- expected setup time and common remediation paths.

“Installed” must never be presented as “operational.” Import and activation reports should distinguish `supported`, `detected-only`, `missing dependency`, `blocked`, `degraded`, and `healthy` resources.

### 8. Trust and supply-chain metadata

Use distinct trust lanes:

- **Official:** maintained by AGH with a defined compatibility and support policy.
- **Verified:** publisher identity, source ownership, security review, conformance tests, and signed immutable release are verified.
- **Community:** open publication with mandatory scanning and explicit absence of support guarantees.
- **Workspace or local:** private packages outside the public registry.

Identity verification and content review should be separate badges. A release should carry source revision, checksum or signature, license, compatibility range, dependency lock, permission summary, support contact, last verified runtime version, and uninstall behavior. Updates that add OAuth scopes, subprocess access, hooks, event subscriptions, jobs, triggers, or high-risk tools require a permission diff and reapproval.

### 9. Ownership, update, and uninstall semantics

Packages should classify material as bundle-owned, user-owned, mergeable, generated, secret, immutable, or migratable. An update preview must show how each item changes. Deactivation and uninstall must stop owned jobs, triggers, listeners, bridge instances, and subprocesses; explain which data remains; and offer credential revocation without deleting user-owned data.

### 10. Workspace isolation

Every resource and datum must declare global, workspace, session, or agent scope. Connection credentials, cache entries, memory, events, delivery targets, and bundle inventory must preserve that boundary across CLI, HTTP, UDS, core, stores, SSE, and native tools. Agency, consulting, support, family, HR, legal, and finance packages make this non-negotiable.

## Marketplace and onboarding design

### Outcome-first discovery

The landing surface should lead with jobs and audiences, then expose technical filters. Example collections:

- Get your inbox under control
- Run a small business
- Founder essentials
- Sales and customer success
- Support operations
- Launch and market a product
- Creator starter pack
- Ecommerce operations
- Personal productivity
- Home and family
- Research and education
- Local-only and privacy-first
- Read-only safe starters
- Developer delivery

A bundle listing should show the outcome, audience, sample result, time to first value, required accounts, supported provider variants, resources activated, schedule, estimated usage/cost, data egress, permission budget, approval defaults, maintainer, support path, last verification, and a complete composition graph.

### Guided first run

A non-technical path should be:

1. Choose an outcome.
2. Select the tools already in use.
3. Connect the minimum required accounts.
4. Choose where results arrive.
5. Choose an autonomy and approval level.
6. Run a fixture-backed demo.
7. Preview a dry run against real data.
8. Approve activation of channels, jobs, triggers, and agents.
9. Receive and inspect the first useful result.
10. See health, cost, schedule, permissions, and a one-click pause path.

The first screen should not require cron syntax, MCP transport, TOML, tool schemas, or internal event names. Those details remain available under advanced inspection and through agent-manageable structured output.

### Credential-first integration records

The integrations.sh research shows why a directory of endpoints is insufficient. AGH integration records should preserve interface type, endpoint, authentication scheme, credential acquisition URL, required scopes, source provenance, discovery method, and freshness. When one service has MCP, REST, GraphQL, and CLI surfaces, the extension author should choose or combine them deliberately rather than expose duplicates to the user.

### Machine-readable discovery

A future ecosystem manifest should be discoverable through a stable API and CLI and may adopt an owner-published `/.well-known` record. Machine discovery can create candidates, but it must not confer trust or silently enable write-capable tools.

## Community flywheel

The strongest contributor model is:

> Developers publish extensions. Domain experts publish bundles. Curators help people find a trusted starting point.

That model needs the following support:

1. `agh extension create` and `agh bundle create` scaffolds with representative examples.
2. Local validation for manifest shape, dependency graph, workspace scope, secrets, permissions, install/update/uninstall, and health checks.
3. A fixture-backed test harness plus positive, negative, credential-failure, rate-limit, partial-failure, idempotency, and cleanup cases.
4. An automatic listing preview with composition, permission, and compatibility reports.
5. Publisher namespaces verified through repository, OAuth/OIDC, DNS, or HTTP ownership.
6. Git, registry, and local development sources with immutable public releases.
7. Fork and remix as first-class operations, preserving attribution and exposing dependency changes.
8. Public bounties for missing extension dependencies required by popular bundle ideas.
9. Editorial collections, build challenges, field notes, short demos, and verified use cases.
10. Migration assistants for existing Claude, Codex, Pi, OpenClaw, Hermes, Paperclip, MCP, and automation-platform assets.

Marketplace ranking should not reduce trust to stars or downloads. Better signals include clean-install success, first-value success, retained active use, recent maintenance, compatibility CI, permission stability, support responsiveness, low failure rates, and verified field reports. Adoption metrics should be privacy-preserving and opt-in.

## Suggested delivery sequence

### Wave 0 — prove the current model

- Build one outcome extension with a current bundle profile and no cross-extension dependency installation.
- Exercise preview, workspace activation, agent/persona/wake-policy projection, jobs, triggers, disabled bridge-instance projection, separate bridge enablement, pause/deactivation, and cleanup.
- Record the friction created by extension-scoped static resources.
- Use `Meeting to Action` or `Fix a Linear Issue` because each has an observable end state and exercises multiple runtime primitives.

### Wave 1 — foundation connections

- Complete connection health and permission summaries.
- Add Google Workspace, Microsoft 365, Notion, one CRM, one support desk, meetings, document intelligence, web research, webhook automation, Shopify, Stripe, and one creative asset provider.
- Audit the in-tree bridge implementations for bidirectional outcome support.

### Wave 2 — non-technical showcase

- Publish five to eight verified bundles across personal productivity, local business, sales, support, marketing/creator, ecommerce, and leadership.
- Require demo, dry run, first-success, safety defaults, and complete deactivation evidence for each.
- Test onboarding with people who do not use a terminal.

### Wave 3 — composable marketplace

> **Rejected by the 2026-07-20 decision — do not implement.** Dependency resolution, provider/capability selection, profile-scoped resource semantics, ownership-aware update diffs, and signed releases are out of scope (see the platform-gap decision note above and the PE table in extension-opportunities.md). This wave is retained as historical analysis only.

- ~~Add dependency resolution, provider selection, profile-scoped resource semantics, ownership-aware updates, and signed releases.~~
- ~~Open verified partner and community lanes.~~
- ~~Add collections, remix, bounties, and migration paths.~~

### Wave 4 — vertical expansion

- Promote bundle families backed by real field evidence: agency operations, recruiting, finance close, legal intake, real estate, nonprofit grants, hospitality, property operations, education, home/family, and other regional opportunities.
- Prefer a credible domain maintainer and explicit risk boundary over raw catalog breadth.

## Evidence map

The companion catalogs use short source codes. A source code means the ecosystem supplied a precedent, integration surface, or demand signal. It does not independently validate the proposed AGH product.

### AGH source of truth

- `internal/extension/manifest.go`
- `internal/extension/bundle.go`
- `internal/extension/surfaces/registry.go`
- `internal/extensionprotocol/host_api.go`
- `internal/bundles/model/model.go`
- `internal/bundles/resource.go`
- `internal/bundles/resource_store.go`
- `internal/api/contract/bundles.go`
- `internal/tools/builtin/extensions.go`
- `internal/tools/builtin/bundles_resources.go`
- `extensions/bridges/README.md`
- `extensions/bridges/{discord,gchat,github,linear,slack,teams,telegram,whatsapp}/`
- `packages/site/content/runtime/core/extensions/`
- `packages/site/content/runtime/core/loops/extensions.mdx`
- `packages/site/content/runtime/core/resources/bundles.mdx`
- `skills/agh/references/capabilities-and-bundles.md`
- `docs/rfcs/005_capability-catalogs-agent-directories.md`

### Local competitor snapshots

| Code | Source | Most relevant evidence |
| --- | --- | --- |
| `PI` | `.resources/pi/` at `8479bd84743e8889f728acb21a62794102db0529` | In-process extensions, lifecycle middleware, packages, skills, prompt templates, guided UI, provider plugins, non-persistent temporary trials, and subagent compositions. Temporary execution does not reduce the package's process permissions or trust requirement. Key files include `packages/coding-agent/docs/extensions.md`, `packages/coding-agent/docs/packages.md`, and `packages/coding-agent/examples/extensions/`. |
| `OC` | `.resources/openclaw/` at `7a456e362d0bafaa898b02015792c27ed0888048` | Manifest-first plugins, capability contracts, channels, hook packs, Task Flow, standing orders, cron, webhooks, permissions, migrations, ClawHub, and store discovery. Key files include `docs/plugins/`, `docs/automation/`, `docs/clawhub/`, and `extensions/`. |
| `HER` | `.resources/hermes/` at `4281151ae859241351ba14d8c7682dc67ff4c126` | 89 plugin manifests, 174 bundled/optional skills, optional MCP records, profile distributions, automation blueprints, trust tiers, desktop onboarding, dashboard extensions, and provider families. Key files include `website/docs/user-guide/features/`, `website/docs/user-guide/profile-distributions.md`, `cron/`, `plugins/`, and `apps/`. |
| `CODEX-LOCAL` | `.resources/codex/` at `9e552e9d15ba52bed7077d5357f3e18e330f8f38` | Plugin manifest parsing, marketplace and loader behavior, connector/skill composition, and install policy. Key files include `codex-rs/core-plugins/src/` and `docs/skills.md`. |

### Primary external sources

| Code | Source | Most relevant evidence |
| --- | --- | --- |
| `INT` | [integrations.sh](https://integrations.sh/), [`api.json`](https://integrations.sh/api.json), [registry revision `7ae23b7c`](https://github.com/UsefulSoftwareCo/integrations/commit/7ae23b7cb75c0b62fd32a08bb1e241f85b829d8e), and [publishing guide](https://integrations.sh/publishing/) | On 2026-07-17, the API payload contained 5,758 MCP/API/GraphQL/CLI records; the homepage published 3,230 domains. Credential metadata, endpoint discovery, and provenance are the durable product evidence; the counts are time-bound and reproduced below. |
| `ANTH` | [Claude plugins overview](https://code.claude.com/docs/en/plugins), [Claude Code plugin reference](https://code.claude.com/docs/en/plugins-reference), [official plugin directory](https://github.com/anthropics/claude-plugins-official), [pinned manifest](https://github.com/anthropics/claude-plugins-official/blob/ded0c09c1ff6003a16d52fc28eae33ed55e4eb87/.claude-plugin/marketplace.json), and [knowledge-work plugins](https://github.com/anthropics/knowledge-work-plugins) | Plugins combining skills, connectors/MCP, commands, agents, hooks, monitors, and role-focused packages for sales, support, product, marketing, legal, finance, data, research, and productivity. The pinned manifest contains 256 entries. |
| `OAI` | [OpenAI plugin guidance](https://help.openai.com/en/articles/20001256-plugins-in-codex/), [build plugins](https://learn.chatgpt.com/docs/build-plugins), [submit plugins](https://learn.chatgpt.com/docs/submit-plugins), [official plugin repository](https://github.com/openai/plugins), [pinned manifest](https://github.com/openai/plugins/blob/11c74d6ba24d3a6d48f54a194cd00ef3beea18f9/.agents/plugins/marketplace.json), and [role-specific plugins](https://github.com/openai/role-specific-plugins) | Plugins combining skills, MCP-backed apps/connectors, hooks, install policy, listing metadata, trust review, and role-focused packages. The pinned manifest contains 180 entries across 11 categories. |
| `MCP` | [Official MCP Registry](https://github.com/modelcontextprotocol/registry) | Publisher namespace ownership and an ecosystem-scale server registry. |
| `PAP` | [Paperclip repository](https://github.com/paperclipai/paperclip), [official documentation](https://docs.paperclip.ing/), [company package repository](https://github.com/paperclipai/companies), and [Paperclip Community](https://paperclip.community/) | Portable companies, agents, Soul, Heartbeat, goals, tasks, budgets, approvals, adapters, import/export, community field notes, and company templates. |

`.resources/paperclip` is not present in this checkout. The Paperclip research therefore used the primary public repository, documentation, company-template repository, and community site. Paperclip's plugin specification describes a proposed or early system in places; proposed tools, jobs, events, UI slots, and lifecycle behavior are treated as architectural reference, not as fully shipped product claims.

### Reproducing volatile catalog counts

The following commands reproduce every exact external catalog count used in the companion documents. The two plugin counts are derived from immutable Git revisions. integrations.sh's deployed payload does not expose its source commit: revision `7ae23b7c` landed 32 seconds before the payload's `generatedAt` timestamp and is the nearest known revision, but treating it as the exact deployment identity would be an inference.

```bash
# integrations.sh: 5,758 records and the transport mix
curl -fsSL https://integrations.sh/api.json \
  | jq '{version, generatedAt, surfaces:(.data|length), kinds:(.data|sort_by(.kind)|group_by(.kind)|map({kind:.[0].kind,count:length}))}'
curl -fsSL https://integrations.sh/api.json | sha256sum

# integrations.sh: published homepage count, observed on 2026-07-17
curl -fsSL https://integrations.sh/ \
  | grep -oE '[0-9][0-9,]* domains' \
  | sort -u
curl -fsSL https://integrations.sh/ | sha256sum

# OpenAI: 180 plugins across 11 categories at revision 11c74d6b
curl -fsSL https://raw.githubusercontent.com/openai/plugins/11c74d6ba24d3a6d48f54a194cd00ef3beea18f9/.agents/plugins/marketplace.json \
  | jq '{plugins:(.plugins|length), categories:([.plugins[].category]|unique|length)}'

# Anthropic: 256 plugins at revision ded0c09c
curl -fsSL https://raw.githubusercontent.com/anthropics/claude-plugins-official/ded0c09c1ff6003a16d52fc28eae33ed55e4eb87/.claude-plugin/marketplace.json \
  | jq '{plugins:(.plugins|length)}'
```

Expected integrity values:

| Snapshot | Expected value |
| --- | --- |
| integrations.sh `api.json` | `generatedAt=2026-07-08T01:44:23.703Z`; 5,758 records: 565 CLI, 113 GraphQL, 1,274 MCP, and 3,806 OpenAPI; SHA-256 `887d050c487bacae08c1bc708daa3ea06e86780d513725671ab72fd85e72109e`; ETag `5a99f8307da2aa9bf4e7059cddbfcd69` |
| integrations.sh homepage | Published count `3,230 domains`; SHA-256 `c5705d3346b4f60c3f9937bbc83e67016bb628b67455e89281f104ec0a76acc4` |
| OpenAI pinned manifest | 180 plugins, 11 categories; SHA-256 `0b19caddb65a6125b7af3138634ba78da1c0fb204290e0599fa9a6fdc4c5258f` |
| Anthropic pinned manifest | 256 plugins; SHA-256 `0fdcf8b09c97b13b1a79bb77fa7c752ca038bcbbab6bd1ed4292f8841fd81bfc` |

The integrations.sh homepage count includes discovered domains with no normalized surface record. It is therefore intentionally reported as a published observation; it should not be recomputed from unique `api.json` domains or from the separately live-merged `api/domains.json` endpoint.

### Community breadth and demand signals

| Code | Source | Use in this research |
| --- | --- | --- |
| `N8N` | [awesome-n8n-templates](https://github.com/ScraperNode/awesome-n8n-templates) | Large community signal for complete jobs across leads, documents, content, productivity, marketing, CRM, support, HR, invoices, and commerce. |
| `MCP-A` | [punkpeye/awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers) and [wong2/awesome-mcp-servers](https://github.com/wong2/awesome-mcp-servers) | Breadth signal for MCP-accessible domains; every candidate still requires individual trust and maintenance review. |
| `SKILL-A` | [awesome-claude-skills](https://github.com/ComposioHQ/awesome-claude-skills) and [awesome-claude-code](https://github.com/subinium/awesome-claude-code) | Breadth signal for reusable skills, plugins, browser tooling, memory, security, orchestration, business, creative, and productivity use cases. |
| `PAP-A` | [awesome-paperclip](https://github.com/gsxdsm/awesome-paperclip) and [Paperclip community resources](https://paperclip.community/resources) | Community plugins, adapters, UIs, memory, messaging, and operating field notes. |

Awesome lists and community catalogs are discovery inputs, not trust authorities. Candidate inclusion does not endorse a particular third-party implementation.

## Research conclusions by reference

| Reference | Strongest lesson for AGH |
| --- | --- |
| integrations.sh | The atomic map must include authentication and provenance, not only an endpoint or tool list. |
| Pi | A narrow core plus flexible extensions, guided UI, non-persistent temporary trials, and reusable skill/package primitives can accelerate experimentation; temporary execution keeps the same trust boundary. |
| OpenClaw | Capability families, broad channels/providers, recurring automation, permissions, migrations, and marketplace trust are all adoption surfaces. |
| Hermes | Install, enable, connect, and schedule are distinct consent steps; profile distributions and human-friendly automation forms are strong proto-bundle patterns. |
| Paperclip | Portable operational organizations, budgets, approvals, Heartbeats, handoffs, and recovery make multi-agent packages legible as an operating system rather than a prompt collection. |
| Claude | Role-focused plugins show that functions such as sales, support, finance, legal, marketing, product, and research can be first-class distribution categories. |
| OpenAI | Plugin manifests, listing metadata, install/auth policy, connector-plus-skill composition, and structured review provide a strong marketplace contract. |
| n8n and awesome lists | Community demand clusters around complete outcomes and long-tail niches; breadth is useful for discovery but must be filtered through AGH trust and runtime fit. |

## AGH Impact Audit

- **Native tools:** no runtime impact. Checked `skills/agh/references/native-tools.md`, `internal/tools/builtin_ids.go`, and the current source-defined tool map: extensions expose search/list/info/install/update/remove/enable/disable; bundles expose list/info/activate/deactivate/status; resources expose list/info/snapshot. Bundle preview, update, and network settings remain structured CLI/HTTP/UDS fallbacks. The documents add no tool ID, toolset, descriptor, schema, digest, risk flag, availability diagnostic, capability gate, or fallback.
- **Extensibility and hooks:** no runtime impact. The documents describe the current manifest, resource registry, bundle projector, hook/tool/skill/MCP relationship, bridge presets, marketplace lifecycle, and future opportunities. They do not change extensions, hooks, registries, bridge SDKs, MCP sidecars, bundle behavior, or `config.toml`.
- **Workspace data isolation:** not applicable to runtime state because this editorial diff adds no datum. Checked global/workspace validation and projection in `internal/bundles/model/model.go`, `internal/bundles/service.go`, `internal/bundles/resource.go`, `internal/bundles/resource_projection.go`, `internal/bundles/resource_store.go`, and `internal/daemon/native_bundle_resource_tools.go`, plus the owning bundle service/resource tests. The opportunity requirements call for future credentials, memory, caches, events, SSE, and delivery to preserve global/workspace/session/agent scope; they do not claim that new paths exist.
- **Official AGH skill:** no impact. Checked `skills/agh/SKILL.md`, `skills/agh/references/capabilities-and-bundles.md`, `skills/agh/references/tools-and-skills.md`, and `skills/agh/references/native-tools.md`. No public behavior, tool ID, CLI path, event, capability, bundle/resource contract, memory/network/task semantic, or fallback changed, so the bundled skill needs no update.

## Web, docs, config, and QA impact

- `web/`: no impact; no route, component, hook, or cache behavior changed.
- `packages/site/`: no impact; these are internal ecosystem research artifacts, not public runtime documentation or a claim of shipped support.
- `config.toml`: no keys, defaults, lifecycle behavior, or examples changed.
- `docs/qa/scenarios/`: no impact; the diff changes no user-visible runtime behavior. No scenario was added or reset.
