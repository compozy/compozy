# AGH Atomic Extension and Integration Opportunities

- **Language:** English · [Português (Brasil)](ptbr/extension-opportunities.md)
- **Status:** research reference and opportunity catalog, not a committed roadmap
- **Research snapshot:** 2026-07-17
- **Companion documents:** [Ecosystem opportunity map](README.md) · [Outcome bundles](bundle-opportunities.md)

## Purpose

This catalog names reusable, atomic integrations and providers that could make existing services reachable from AGH. It contains **211 new opportunities**: **18 platform-enabling investments** and **193 service or provider extensions** across twelve domains. The eight bridge providers already implemented in the repository are recorded separately as baseline and are not counted as new candidates.

Every row is a proposal. Inclusion does not claim that AGH supports the service, that the provider will grant API access, that a catalog entry is official, that an MCP server is safe, or that the integration has product-market fit. Provider terms, scopes, rate limits, regional availability, commercial access, data handling, and maintenance ownership must be verified before implementation.

The unit of this catalog is deliberately atomic:

- A service extension connects one service or one coherent service boundary.
- An operation provider performs one reusable function, such as document extraction or webhook ingress.
- An atomic extension may package tools, skills, hooks, a watch source, or a local MCP server, but it should not silently install a persona or begin recurring work.
- Outcome, persona, and vertical packages belong in [the bundle catalog](bundle-opportunities.md), where several atomic extensions can be composed behind an explicit activation and approval boundary.

## Current AGH fit

The current implementation supports installable, versioned extensions with provenance, trust state, enablement, and lifecycle management. A manifest can package skills, Loops, agents, bundles, typed hooks, manifest tools, and command-launched MCP servers. A subprocess extension can provide runtime contracts such as tool.provider, loop.watch_source, memory.backend, model.source, and bridge.adapter, subject to declared Host API method and security capability grants.

Bundle profiles currently project AGH Network channels, agents, optional Soul and Heartbeat sidecars, jobs, triggers, and external messaging bridge presets. Profile activation does not install arbitrary extension dependencies and does not profile-scope extension-level skills, Loops, hooks, tools, or MCP servers. Therefore, most rows below are extension candidates; a plug-and-play outcome that composes several rows still needs preinstalled dependencies or future dependency resolution.

Two current constraints materially affect this catalog:

1. Remote APIs normally need a trusted tool.provider subprocess plus a narrowly scoped skill. An upstream OpenAPI, GraphQL, CLI, or hosted MCP surface is discovery evidence, not a current zero-code import path.
2. External messaging bridge adapters are production-capable in-tree or local-source implementations, but third-party bridge authoring is not yet a first-class public SDK and marketplace path. Any new messaging bridge adapter depends on PE-014 before it should be presented as a routine community extension.

Marketplace extensions also have a stricter read-oriented grant ceiling today. Write-capable connectors need an explicit trust and permission design; they must not be described as marketplace-ready merely because a provider exposes an API.

## Legend

### Priority

| Code | Meaning |
| --- | --- |
| **P0** | Broad reach or high composition leverage, credible initial access path, visible first value, and a safe read-first or draft-first slice. |
| **P1** | Strong audience or vertical value after foundations exist; usually adds write semantics, richer data modeling, or operational burden. |
| **P2** | Long-tail, niche, sensitive, expensive, partner-gated, region-specific, or still uncertain enough to require discovery before roadmap commitment. |

Priority is directional product judgment, not verified demand.

### AGH implementation surface

| Code | Current meaning |
| --- | --- |
| **TP** | Subprocess extension implementing tool.provider. |
| **TL** | Manifest-declared tool resource with explicit input/output schema and risk metadata. |
| **WS** | Subprocess extension implementing loop.watch_source for bounded polling. |
| **SK** | Extension-packaged skill describing safe, service-specific operation. |
| **HK** | Typed AGH lifecycle hook; not a generic substitute for provider webhooks. |
| **LP** | Packaged deterministic Loop. |
| **MCP-L** | Command-launched local MCP server declared by the extension. Hosted MCP enrollment requires PE-004. |
| **BD** | Bundle catalog or profile projection. |
| **BA** | bridge.adapter implementation. Third-party distribution requires PE-014. |
| **CORE** | Daemon, registry, SDK, permission, or lifecycle work; not representable as an extension alone today. |

The surface column is an implementation hypothesis. It does not assert that the named upstream currently offers a particular protocol.

### Risk and access

| Code | Meaning |
| --- | --- |
| **R0** | Read-only retrieval, search, transformation, or local computation. |
| **R1** | Private, reversible write such as a draft, internal record, or annotation. |
| **R2** | External or public write such as sending, publishing, or changing customer-visible state. |
| **R3** | High-impact action involving money, identity, access, deletion, production infrastructure, employment, legal state, or regulated data. |
| **A0** | Open protocol, local software, or public-data endpoint; terms and rate limits still apply. |
| **A1** | Documented developer surface with credentials, scopes, tenant setup, plan limits, or review still required. |
| **A2** | Enterprise, paid, admin-approved, or commercial relationship commonly required. |
| **A3** | Closed, partner-only, restricted, unstable, or insufficiently verified. Feasibility discovery is mandatory before scheduling implementation. |

Risk is the highest plausible operation in the proposed extension, not the default permission. Every R2 or R3 candidate should ship a narrower read-only or draft-only starting mode where the provider permits it.

### Evidence semantics

| Prefix | Meaning |
| --- | --- |
| **V:** | Verified ecosystem evidence: the named service, surface, plugin, or public API was observed in the cited primary catalog or provider documentation during the snapshot. This verifies presence only, not access, quality, safety, or AGH compatibility. |
| **P:** | Pattern or community signal: the source demonstrates a role, recurring job, or integration category, but does not verify this proposed implementation. |
| **I:** | Product inference: AGH fit, outcome, priority, surface, or packaging is an inference made for this catalog. |

Source codes are defined in the appendix. Every candidate carries at least one inference marker so verified upstream presence cannot be mistaken for a shipped AGH integration.

## Existing production in-tree bridge baseline

These are production implementations present in the source tree. Released agh artifacts do **not** include their provider executables or install them automatically. An operator currently builds and installs them explicitly from a trusted source checkout. They are baseline, not new opportunity rows.

| ID | Platform | In-tree directory | Current provider contract | Catalog treatment |
| --- | --- | --- | --- | --- |
| BL-001 | Discord | [extensions/bridges/discord](../../extensions/bridges/discord/) | Signed interactions/webhooks; message create, edit, and delete | Existing baseline; do not count as a new extension |
| BL-002 | Google Chat | [extensions/bridges/gchat](../../extensions/bridges/gchat/) | Direct, Pub/Sub, or hybrid JWT inbound; message create, edit, and delete | Existing baseline; do not count as a new extension |
| BL-003 | GitHub | [extensions/bridges/github](../../extensions/bridges/github/) | Signed issue/review-comment webhooks; comment create, edit, and delete | Existing baseline; do not count as a new extension |
| BL-004 | Linear | [extensions/bridges/linear](../../extensions/bridges/linear/) | Signed comment or Agent Session webhooks; comments or append-only activities | Existing baseline; do not count as a new extension |
| BL-005 | Slack | [extensions/bridges/slack](../../extensions/bridges/slack/) | Signed events, commands, and interactions; message create, edit, and delete | Existing baseline; do not count as a new extension |
| BL-006 | Microsoft Teams | [extensions/bridges/teams](../../extensions/bridges/teams/) | Bot Framework activities with bearer JWTs; activity create, edit, and delete | Existing baseline; do not count as a new extension |
| BL-007 | Telegram | [extensions/bridges/telegram](../../extensions/bridges/telegram/) | Secret-token Bot API webhooks; message create, edit, and delete | Existing baseline; do not count as a new extension |
| BL-008 | WhatsApp | [extensions/bridges/whatsapp](../../extensions/bridges/whatsapp/) | Meta verification and signed POST; Cloud API text create | Existing baseline; do not count as a new extension |

The baseline describes external messaging bridge transport, not AGH Network channels or every business operation exposed by the corresponding vendor. A separate tool-provider extension may still be appropriate for service-specific records, search, files, administration, or analytics.

## Platform-enabling opportunities

The following are intentionally separated from atomic service extensions. Each requires CORE work or a new public contract before the desired author or user experience is truthful.

> **Product decision — 2026-07-20 (binding). Do not implement any row marked `⛔ rejected`.**
>
> This list was reviewed and pruned. The accepted direction is: **connectors converge on MCP servers** (authentication handled by the existing MCP OAuth layer) **or subprocess extensions that ship real code and read API keys from the vault.** Under that model there is no need for no-code API import/compilation (a connector author writes code or reuses an MCP server) or for a generic cross-connector OAuth broker (OAuth stays MCP-only, matching every comparable harness). Only **credential/secret liveness health** is worth building as new core work.
>
> | Decision | Items | Rationale |
> | --- | --- | --- |
> | ✅ **Accepted** | **PE-015 — liveness/health only** | Knowing whether a stored credential is alive (ok / expired / revoked / missing) is real value; the `bind a secret to one permission grant` half of PE-015 is **rejected**. |
> | ☑️ Already implemented | PE-004, PE-007, PE-010 | Hosted MCP enrollment + OAuth, webhook ingress router, and tool risk metadata already ship today. |
> | ⛔ **Rejected — do not build** | PE-001, PE-002, PE-003, PE-005, PE-006, PE-008, PE-009, PE-011, PE-012, PE-013, PE-014, PE-016, PE-017, PE-018 | No-code API import/compile, generic OAuth broker, third-party authoring kits, sandbox/egress, signing/registry/conformance, and bundle composition (dependency resolution, profile-scoped resources, capability contracts) are out of scope for the core/engine roadmap. |

| Decision | ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ⛔ rejected | PE-001 | Integration surface catalog importer | P0 | Extension authors turn a service domain into a provenance-preserving candidate record instead of rediscovering endpoints and auth from scratch. | CORE+SK | R0/A0 | V:INT · V:AGH · I:AGH |
| ⛔ rejected | PE-002 | OpenAPI toolset compiler | P0 | Authors generate typed draft tools from a reviewed OpenAPI operation allowlist, then classify mutations before publishing. | CORE+TL+TP | R2/A1 | V:INT · V:OAI · I:AGH |
| ⛔ rejected | PE-003 | GraphQL schema adapter | P1 | Authors map selected queries and mutations into bounded tools without exposing raw arbitrary GraphQL execution. | CORE+TL+TP | R2/A1 | V:INT · I:AGH |
| ☑️ implemented | PE-004 | Hosted MCP enrollment and OAuth | P0 | Operators connect a remote MCP endpoint, complete auth, inspect tools, and approve a stable permission snapshot. | CORE | R3/A1 | V:MCP · V:OAI · V:ANTH · I:AGH |
| ⛔ rejected | PE-005 | CLI adapter and packager | P1 | Authors wrap a version-pinned CLI command set with schemas, exit-code handling, sandboxing, and deterministic output limits. | CORE+TP+TL | R3/A1 | V:INT · P:SKILL-A · I:AGH |
| ⛔ rejected | PE-006 | OAuth and credential broker | P0 | Operators bind least-privilege accounts, rotate or revoke credentials, and see connection health without exposing secrets to agents. | CORE | R3/A1 | V:INT · V:OAI · V:AGH · I:AGH |
| ☑️ implemented | PE-007 | Webhook ingress router | P0 | Services deliver authenticated, deduplicated events that can fire workspace-scoped automation triggers with replay protection. | CORE+TP | R2/A1 | P:N8N · V:AGH · I:AGH |
| ⛔ rejected | PE-008 | Watch-source author kit | P0 | Extension authors implement cursoring, backoff, checkpointing, and bounded polling against the existing loop.watch_source contract. | CORE+WS+SK | R1/A1 | V:AGH · I:AGH |
| ⛔ rejected | PE-009 | Connector sandbox and egress allowlist | P0 | Operators constrain subprocess, CLI, and contributed-code network access to declared service domains and runtime budgets. | CORE+TP | R3/A0 | V:AGH · P:INT · I:AGH |
| ☑️ implemented | PE-010 | Tool risk metadata normalizer | P0 | Users receive consistent read-only, open-world, destructive, interaction, and approval labels across generated and hand-authored tools. | CORE+TL | R3/A0 | V:OAI · V:AGH · I:AGH |
| ⛔ rejected | PE-011 | Signed releases and publisher identity | P0 | Operators verify namespace ownership, immutable artifacts, checksums, publisher identity, and provenance before enablement. | CORE | R3/A0 | V:MCP · V:OAI · V:ANTH · V:AGH · I:AGH |
| ⛔ rejected | PE-012 | Registry federation and namespace ownership | P1 | Teams discover official, verified, community, workspace, and local packages without collapsing those trust lanes. | CORE | R2/A0 | V:MCP · V:ANTH · V:OAI · I:AGH |
| ⛔ rejected | PE-013 | Extension conformance harness | P0 | Publishers prove clean install, auth failure, scope denial, pagination, rate limit, idempotency, partial failure, cleanup, and removal behavior. | CORE+TP+TL | R2/A0 | V:OAI · V:AGH · I:AGH |
| ⛔ rejected | PE-014 | Third-party bridge SDK and marketplace grants | P0 | Messaging bridge authors build and test adapters through a supported SDK, conformance suite, and explicit marketplace permission lane. | CORE+BA | R3/A2 | V:AGH · I:AGH |
| ✅ accepted (health only) | PE-015 | Permission-scoped secret bindings and health | P0 | Operators bind a secret to one extension permission grant, validate it without disclosure, and distinguish missing, expired, denied, and unhealthy states. **Accepted scope: liveness/health only; the secret→permission-grant binding is rejected.** | CORE+TP | R3/A1 | V:INT · V:AGH · I:AGH |
| ⛔ rejected | PE-016 | Transactional dependency resolution and activation | P1 | A bundle resolves required, optional, and one-of extensions; previews permissions; activates atomically; and rolls back owned resources on failure. | CORE+BD | R3/A0 | V:AGH · P:ANTH · P:OAI · I:AGH |
| ⛔ rejected | PE-017 | Profile-scoped static resource activation | P0 | Bundle authors select only the skills, Loops, hooks, tools, and MCP servers required by one profile without exposing every static resource in the owning extension. | CORE+BD | R3/A0 | V:AGH · I:AGH |
| ⛔ rejected | PE-018 | Provider and capability contracts | P0 | Bundle authors require a conforming calendar, CRM, work-management, media, or peer capability interface and let operators select a compatible implementation. | CORE+BD | R3/A0 | V:AGH · P:OAI · P:ANTH · I:AGH |

## Atomic service and provider catalog

### Personal productivity and work management

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| PW-001 | Gmail | P0 | Individuals and teams search threads, classify mail, extract commitments, and prepare reply drafts. | TP+SK+WS | R2/A1 | V:OAI · V:INT · I:AGH |
| PW-002 | Google Calendar | P0 | Users inspect availability, detect conflicts, prepare agendas, and propose or create events under approval. | TP+SK+WS | R2/A1 | V:OAI · V:INT · I:AGH |
| PW-003 | Google Drive | P0 | Teams search, read, organize, and place governed files in shared drives with source links. | TP+SK | R1/A1 | V:OAI · V:INT · I:AGH |
| PW-004 | Google Docs | P1 | Users read documents, create structured drafts, and apply explicitly reviewed edits. | TP+SK | R1/A1 | V:INT · I:AGH |
| PW-005 | Google Sheets | P1 | Operators read tables, append validated rows, and produce formula-safe updates with cell provenance. | TP+SK | R1/A1 | V:INT · I:AGH |
| PW-006 | Outlook Email | P0 | Microsoft 365 users search mail, triage threads, extract actions, and prepare replies. | TP+SK+WS | R2/A1 | V:OAI · I:AGH |
| PW-007 | Outlook Calendar | P0 | Microsoft 365 users inspect schedules, detect conflicts, and propose or create meetings. | TP+SK+WS | R2/A1 | V:OAI · I:AGH |
| PW-008 | OneDrive | P1 | Users retrieve and organize personal or team files while retaining tenant and source identity. | TP+SK | R1/A1 | V:INT · I:AGH |
| PW-009 | SharePoint | P0 | Enterprise teams search governed sites, lists, and documents and create reviewed internal updates. | TP+SK | R1/A2 | V:OAI · V:INT · I:AGH |
| PW-010 | Notion | P0 | Teams search workspace knowledge, create database records, and maintain reviewed pages. | TP+SK+WS | R1/A1 | V:OAI · V:INT · I:AGH |
| PW-011 | Airtable | P0 | Operators query bases, create validated records, and monitor operational views. | TP+SK+WS | R1/A1 | V:ANTH · V:INT · I:AGH |
| PW-012 | Asana | P1 | Teams inspect portfolios and create or update approved tasks, owners, and due dates. | TP+SK+WS | R1/A1 | V:ANTH · V:INT · I:AGH |
| PW-013 | ClickUp | P1 | Teams search workspaces and maintain approved tasks, comments, and custom-field state. | TP+SK+WS | R1/A1 | V:OAI · I:AGH |
| PW-014 | monday.com | P1 | Operations teams read boards and apply schema-aware item and status updates. | TP+SK+WS | R1/A1 | V:INT · I:AGH |
| PW-015 | Todoist | P1 | Individuals capture, prioritize, schedule, and close personal tasks through a small, reversible surface. | TP+SK+WS | R1/A1 | V:INT · I:AGH |
| PW-016 | Trello | P1 | Teams inspect boards and create or move cards with explicit board and list identity. | TP+SK+WS | R1/A1 | V:INT · I:AGH |
| PW-017 | Home Assistant | P0 | Households and local operators inspect entity state, receive bounded state-change events, and invoke explicitly approved device or scene actions through one self-hosted control plane. | TP+SK+WS | R3/A0 | P:HER · I:AGH |

### Knowledge, documents, and content systems

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| KN-001 | Confluence | P0 | Teams search governed spaces, cite pages, and prepare reviewed knowledge updates. | TP+SK+WS | R1/A1 | V:INT · V:ANTH · I:AGH |
| KN-002 | Coda | P1 | Operators query docs and tables and create schema-aware rows or page drafts. | TP+SK | R1/A1 | P:N8N · I:AGH |
| KN-003 | Dropbox | P0 | Users search, retrieve, classify, and place files while preserving path and revision provenance. | TP+SK | R1/A1 | V:INT · I:AGH |
| KN-004 | Box | P0 | Enterprise teams search governed content, inspect metadata, and create reviewed file or task updates. | TP+SK | R1/A2 | V:OAI · V:ANTH · V:INT · I:AGH |
| KN-005 | Egnyte | P1 | Regulated teams retrieve governed files and metadata without bypassing repository permissions. | TP+SK | R1/A2 | V:INT · I:AGH |
| KN-006 | Guru | P1 | Support and operations teams retrieve verified cards and propose evidence-backed knowledge revisions. | TP+SK+WS | R1/A2 | V:INT · P:ANTH · I:AGH |
| KN-007 | Slite | P2 | Small teams search internal notes and prepare structured, reviewed knowledge pages. | TP+SK | R1/A3 | P:N8N · I:AGH |
| KN-008 | Obsidian | P1 | Privacy-minded users search and update a local Markdown vault with link-aware, reversible edits. | TP+SK | R1/A0 | P:SKILL-A · I:AGH |
| KN-009 | Readwise | P1 | Researchers retrieve highlights, annotations, and reading state for cited synthesis and review. | TP+SK+WS | R1/A1 | V:INT · P:SKILL-A · I:AGH |
| KN-010 | Evernote | P2 | Existing users retrieve notebooks and prepare conservative note updates or migration exports. | TP+SK | R1/A3 | P:N8N · I:AGH |
| KN-011 | WordPress | P0 | Publishers retrieve content and prepare or publish reviewed posts, pages, metadata, and media references. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| KN-012 | Webflow | P0 | Marketing teams read CMS collections and stage approved content and site-data updates. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| KN-013 | Contentful | P1 | Content teams query structured entries and create validated drafts before editorial publication. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| KN-014 | Sanity | P1 | Teams query content datasets and create schema-valid drafts with explicit dataset scope. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| KN-015 | GitBook | P1 | Documentation teams search spaces and prepare reviewed pages or change requests. | TP+SK | R1/A1 | V:INT · P:SKILL-A · I:AGH |
| KN-016 | Outline | P2 | Self-hosted teams search collections and create governed internal documentation updates. | TP+SK | R1/A0 | P:MCP-A · I:AGH |

### Meetings, scheduling, voice, and communications

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| MT-001 | Zoom | P0 | Meeting-heavy teams retrieve recordings and transcripts, inspect meetings, and prepare follow-up artifacts. | TP+SK+WS | R1/A1 | V:OAI · V:ANTH · V:INT · I:AGH |
| MT-002 | Cisco Webex | P1 | Enterprise teams retrieve meeting artifacts and coordinate approved meeting and messaging actions. | TP+SK+WS | R2/A2 | V:INT · I:AGH |
| MT-003 | RingCentral | P1 | Service teams inspect calls and messages and prepare or send approved communications. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| MT-004 | Twilio | P0 | Businesses send and monitor approved SMS or voice automations through tightly scoped numbers and templates. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| MT-005 | Vonage | P1 | Businesses implement approved SMS, voice, and verification actions with delivery evidence. | TP+SK+WS | R3/A1 | V:INT · I:AGH |
| MT-006 | Dialpad | P2 | Revenue and support teams retrieve call artifacts and draft follow-up from governed conversations. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| MT-007 | Aircall | P1 | Support and sales teams retrieve call metadata and recordings and prepare disposition updates. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| MT-008 | Quo / OpenPhone | P2 | Small businesses inspect calls and messages and prepare approved customer responses. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| MT-009 | Calendly | P0 | Users inspect event types, availability, invitees, and scheduled events for preparation and follow-up. | TP+SK+WS | R0/A1 | V:INT · P:N8N · I:AGH |
| MT-010 | Cal.com | P1 | Teams using an open scheduling stack inspect and manage bookings with explicit event-type scope. | TP+SK+WS | R2/A0 | V:INT · P:MCP-A · I:AGH |
| MT-011 | Loom | P1 | Teams retrieve video metadata and transcripts and turn asynchronous updates into reviewed actions. | TP+SK+WS | R1/A2 | P:SKILL-A · I:AGH |
| MT-012 | Fireflies.ai | P0 | Teams retrieve meeting transcripts, speakers, summaries, and action candidates with source timestamps. | TP+SK+WS | R1/A2 | V:OAI · P:N8N · I:AGH |
| MT-013 | Otter.ai | P1 | Users retrieve governed transcripts and convert them into cited decisions and task drafts. | TP+SK+WS | R1/A2 | V:INT · P:N8N · I:AGH |
| MT-014 | Fathom | P1 | Revenue teams retrieve meeting artifacts and prepare CRM and follow-up drafts without auto-sending. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| MT-015 | Read AI | P2 | Teams retrieve meeting summaries and engagement artifacts subject to tenant consent and retention policy. | TP+SK+WS | R1/A3 | V:INT · I:AGH |
| MT-016 | Granola | P0 | Individuals retrieve consented meeting notes and turn them into reviewed commitments and follow-up drafts. | TP+SK+WS | R1/A2 | V:OAI · I:AGH |

### Sales and revenue operations

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| SR-001 | Salesforce | P0 | Revenue teams inspect accounts and opportunities and prepare governed CRM updates with record provenance. | TP+SK+WS | R2/A2 | V:INT · V:ANTH · I:AGH |
| SR-002 | HubSpot | P0 | Small and mid-market teams retrieve CRM context and create approved contacts, activities, deals, and notes. | TP+SK+WS | R2/A1 | V:OAI · V:INT · I:AGH |
| SR-003 | Pipedrive | P0 | Sales teams inspect pipelines and update approved deals, activities, and notes. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| SR-004 | Attio | P0 | Modern revenue teams query flexible records and create schema-aware relationship and pipeline updates. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| SR-005 | Close | P1 | Sales teams retrieve lead history and prepare approved email, call, task, and pipeline actions. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| SR-006 | Zoho CRM | P1 | Businesses inspect modules and apply validated lead, account, deal, and activity updates. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| SR-007 | HighLevel | P1 | Agencies manage scoped contacts, opportunities, appointments, and draft communications per client subaccount. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| SR-008 | Apollo | P1 | Sellers research prospects and prepare bounded enrichment and outreach inputs without autonomous bulk contact. | TP+SK | R2/A2 | V:ANTH · V:INT · I:AGH |
| SR-009 | ZoomInfo | P2 | Enterprise teams retrieve licensed company and contact intelligence under strict contractual use limits. | TP+SK | R2/A2 | V:OAI · V:ANTH · I:AGH |
| SR-010 | Clay | P1 | Growth teams orchestrate reviewed enrichment table operations with explicit provider and cost budgets. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| SR-011 | Outreach | P1 | Sales teams inspect sequences and tasks and prepare approved prospect actions without silent enrollment. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| SR-012 | Salesloft | P1 | Revenue teams inspect cadences and activities and stage approved updates or next steps. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| SR-013 | Gong | P1 | Revenue leaders retrieve licensed call and deal intelligence for evidence-linked coaching and summaries. | TP+SK+WS | R1/A2 | P:ANTH · P:N8N · I:AGH |
| SR-014 | Common Room | P1 | Go-to-market teams retrieve community and account signals and create reviewed CRM or task actions. | TP+SK+WS | R2/A2 | V:INT · I:AGH |
| SR-015 | 6sense | P2 | Enterprise teams retrieve licensed intent and account signals for bounded prioritization, not autonomous targeting. | TP+SK+WS | R2/A3 | P:N8N · I:AGH |
| SR-016 | Demandbase | P2 | Account-based teams retrieve licensed account intelligence and prepare reviewable campaign or CRM actions. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |

### Marketing and growth

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| MG-001 | Mailchimp | P0 | Marketing teams inspect audiences and campaigns and prepare approved content, segments, and sends. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| MG-002 | Klaviyo | P0 | Commerce teams retrieve customer and campaign state and stage approved segments, flows, and messages. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| MG-003 | Customer.io | P0 | Lifecycle teams inspect people and campaigns and prepare approved event, segment, and message operations. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| MG-004 | Braze | P1 | Enterprise lifecycle teams retrieve campaign state and stage governed audience and message actions. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| MG-005 | Iterable | P1 | Lifecycle teams inspect journeys and prepare approved campaign, list, and event operations. | TP+SK+WS | R2/A2 | V:INT · P:N8N · I:AGH |
| MG-006 | Brevo | P1 | Small businesses inspect contacts and campaigns and prepare approved email or SMS sends. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| MG-007 | SendGrid | P0 | Products send approved transactional email and inspect delivery events through template and recipient allowlists. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| MG-008 | Resend | P1 | Developers and small teams send approved transactional email and inspect delivery state with narrow API keys. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| MG-009 | Postmark | P1 | Products send approved transactional messages and trace bounces, delivery, and inbound events. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| MG-010 | Google Ads | P1 | Advertisers retrieve campaign performance and prepare budget, bid, or creative changes for explicit approval. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| MG-011 | Meta Ads | P2 | Advertisers retrieve licensed performance data and stage reviewed campaign changes after app and account approval. | TP+SK+WS | R3/A3 | V:INT · P:N8N · I:AGH |
| MG-012 | LinkedIn Ads | P2 | B2B teams retrieve approved account analytics and stage reviewed campaign or audience changes. | TP+SK+WS | R3/A3 | V:INT · P:N8N · I:AGH |
| MG-013 | TikTok Ads | P2 | Advertisers retrieve approved performance data and stage campaign changes only after platform access review. | TP+SK+WS | R3/A3 | V:INT · P:N8N · I:AGH |
| MG-014 | Semrush | P1 | Marketing teams retrieve licensed keyword and domain research for cited briefs and monitoring. | TP+SK+WS | R0/A2 | V:INT · P:N8N · I:AGH |
| MG-015 | Ahrefs | P2 | Marketing teams retrieve licensed search intelligence if an approved API plan and terms permit the use case. | TP+SK+WS | R0/A2 | P:N8N · I:AGH |
| MG-016 | Buffer | P1 | Social teams inspect queues and prepare or publish explicitly approved posts across connected social services. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |

### Customer support and success

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| CS-001 | Zendesk | P0 | Support teams retrieve tickets and knowledge, classify queues, and prepare or send approved replies. | TP+SK+WS | R2/A1 | V:INT · V:ANTH · P:N8N · I:AGH |
| CS-002 | Intercom | P0 | Teams inspect conversations and customers and stage approved replies, notes, assignments, and knowledge updates. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| CS-003 | Help Scout | P1 | Small support teams retrieve mailbox context and prepare approved replies, notes, and assignments. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| CS-004 | Gorgias | P0 | Ecommerce support teams retrieve customer and order context and stage approved ticket actions. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| CS-005 | Freshdesk | P0 | Support teams inspect tickets and contacts and prepare approved responses, routing, and status updates. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| CS-006 | Front | P1 | Shared-inbox teams retrieve conversations and stage approved replies, assignments, tags, and comments. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| CS-007 | Kustomer | P1 | Enterprise support teams retrieve customer timelines and stage governed conversation or case actions. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| CS-008 | Pylon | P1 | B2B support teams retrieve customer conversations and prepare approved ticket, account, and escalation updates. | TP+SK+WS | R2/A2 | V:INT · I:AGH |
| CS-009 | Gladly | P2 | Enterprise teams retrieve person-centered service history and stage approved responses under contractual access. | TP+SK+WS | R2/A3 | P:N8N · I:AGH |
| CS-010 | Dixa | P2 | Contact centers retrieve conversations and queue state and prepare governed routing or reply actions. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| CS-011 | Crisp | P1 | Small businesses retrieve conversations and prepare approved inbox replies and contact updates. | TP+SK+WS | R2/A1 | P:N8N · I:AGH |
| CS-012 | Tidio | P2 | Small businesses retrieve chat and contact state and stage approved customer messages. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| CS-013 | Gainsight | P1 | Customer-success teams retrieve account health and prepare evidence-backed risks, tasks, and prescribed actions. | TP+SK+WS | R1/A2 | V:INT · P:ANTH · I:AGH |
| CS-014 | ChurnZero | P2 | Success teams retrieve customer health signals and stage reviewed tasks or outreach recommendations. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |
| CS-015 | Vitally | P1 | B2B success teams retrieve account and project data and prepare governed notes, tasks, and risk updates. | TP+SK+WS | R1/A2 | P:N8N · I:AGH |
| CS-016 | Planhat | P2 | Customer-success teams retrieve licensed health and lifecycle data and stage governed account actions. | TP+SK+WS | R1/A3 | P:N8N · I:AGH |

### Commerce, billing, and payments

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| CP-001 | Shopify | P0 | Merchants retrieve products, customers, orders, and inventory and stage approved operational changes. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-002 | WooCommerce | P1 | WordPress merchants inspect store state and apply approved order, product, and inventory updates. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-003 | BigCommerce | P1 | Merchants retrieve catalog and order state and stage governed commerce updates. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-004 | Adobe Commerce | P1 | Enterprise merchants retrieve catalog and order data and stage approved operational mutations. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| CP-005 | Wix Commerce | P1 | Small businesses inspect stores and prepare approved product, order, and customer actions. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-006 | Squarespace Commerce | P1 | Creators and small businesses inspect products and orders and stage approved store changes. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| CP-007 | Stripe | P0 | Businesses retrieve customers, payments, invoices, and subscriptions; money movement remains explicitly approved. | TP+SK+WS | R3/A1 | V:OAI · V:INT · P:N8N · I:AGH |
| CP-008 | PayPal | P0 | Businesses inspect transactions and disputes and prepare approved invoice, capture, or refund actions. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-009 | Square | P1 | Local businesses inspect payments, catalog, customers, and appointments and stage approved changes. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-010 | Paddle | P1 | Software businesses inspect subscriptions and transactions and prepare governed billing actions. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-011 | Chargebee | P1 | Subscription teams retrieve billing state and stage approved subscription, invoice, and customer changes. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| CP-012 | Recurly | P1 | Subscription businesses inspect accounts and invoices and prepare approved lifecycle or billing changes. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| CP-013 | Razorpay | P1 | Businesses in supported markets inspect payments and subscriptions and stage approved money actions. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| CP-014 | Adyen | P2 | Enterprise merchants inspect payment and dispute state; captures, refunds, and account changes require strict policy. | TP+SK+WS | R3/A2 | V:INT · I:AGH |
| CP-015 | Mercado Pago | P1 | Latin American businesses inspect regional payment state and stage approved refunds or collections where access permits. | TP+SK+WS | R3/A2 | P:N8N · I:AGH |
| CP-016 | Shippo | P1 | Merchants compare rates, create approved labels, and monitor shipments without duplicating purchases. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |

### Finance, legal operations, and people systems

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| FL-001 | QuickBooks Online | P0 | Small businesses retrieve accounting state and prepare categorized transactions, invoices, or reconciliation drafts. | TP+SK+WS | R3/A1 | V:OAI · P:N8N · I:AGH |
| FL-002 | Xero | P0 | Businesses retrieve ledgers, invoices, bills, and contacts and stage approved accounting updates. | TP+SK+WS | R3/A1 | V:INT · P:N8N · I:AGH |
| FL-003 | NetSuite | P1 | Enterprises retrieve ERP records and stage tightly governed finance and operations changes. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-004 | Sage Intacct | P1 | Finance teams retrieve accounting dimensions and prepare reviewed entries, invoices, and close tasks. | TP+SK+WS | R3/A2 | P:N8N · I:AGH |
| FL-005 | Ramp | P0 | Finance teams inspect cards, transactions, receipts, and expenses and prepare approved coding or policy actions. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-006 | Brex | P1 | Finance teams inspect spend, cards, travel, and expenses and stage governed administrative actions. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-007 | Airwallex | P1 | Global businesses inspect balances and transactions; payments and beneficiary changes require explicit approval. | TP+SK+WS | R3/A2 | V:ANTH · V:INT · I:AGH |
| FL-008 | Wise Business | P1 | Businesses inspect accounts and transfers and prepare, but never silently execute, cross-border payments. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-009 | Plaid | P1 | Products retrieve consented financial-account data for reconciliation and analysis without initiating movement by default. | TP+SK+WS | R3/A2 | V:INT · I:AGH |
| FL-010 | Deel | P1 | Global teams retrieve contractor and payroll operations and stage approved administrative updates. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-011 | Rippling | P1 | Organizations retrieve HR and device administration data and stage reviewed employee lifecycle actions. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-012 | Workday | P2 | Enterprises retrieve licensed HR and finance data and stage only policy-approved administrative changes. | TP+SK+WS | R3/A3 | V:INT · P:N8N · I:AGH |
| FL-013 | BambooHR | P1 | People teams retrieve employee records and prepare approved onboarding, time-off, or data-quality actions. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-014 | Greenhouse | P1 | Recruiting teams retrieve jobs and candidates and stage approved notes, interviews, and status changes. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| FL-015 | Ironclad | P1 | Legal operations teams retrieve contracts and approval or routing state and prepare reviewed metadata or routing updates. | TP+SK+WS | R3/A2 | V:INT · P:ANTH · I:AGH |
| FL-016 | DocuSign | P1 | Teams prepare and inspect envelopes and templates; sending, signing, and voiding remain explicit human actions. | TP+SK+WS | R3/A2 | V:OAI · P:N8N · I:AGH |

### Creative, media, and education

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| CM-001 | Figma | P0 | Product and design teams retrieve files and comments and prepare governed design-system or review artifacts. | TP+SK+WS | R1/A1 | V:OAI · V:ANTH · V:INT · I:AGH |
| CM-002 | Canva | P0 | Non-designers create draft assets from approved templates and export them for review before publication. | TP+SK | R2/A1 | V:OAI · V:INT · I:AGH |
| CM-003 | Adobe Creative Cloud | P1 | Creative teams retrieve governed assets and invoke approved document or media operations where product APIs permit. | TP+SK | R2/A2 | V:ANTH · V:INT · I:AGH |
| CM-004 | Miro | P1 | Teams retrieve boards and create reviewed workshop, research, or planning artifacts. | TP+SK | R1/A1 | V:ANTH · I:AGH |
| CM-005 | Lucidchart | P1 | Teams retrieve diagram metadata and prepare structured diagram drafts or reviewed updates. | TP+SK | R1/A2 | V:INT · P:N8N · I:AGH |
| CM-006 | Cloudinary | P0 | Products transform, tag, search, and deliver approved media assets with explicit transformation budgets. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| CM-007 | Shutterstock | P1 | Creative teams search licensed assets and prepare shortlists without implying purchase or usage rights. | TP+SK | R0/A2 | V:INT · I:AGH |
| CM-008 | Unsplash | P1 | Teams search image assets and preserve attribution and provider terms in downstream drafts. | TP+SK | R1/A1 | P:N8N · I:AGH |
| CM-009 | Runway | P1 | Creative teams generate or transform draft video assets under explicit cost, source, and publication controls. | TP+SK | R2/A2 | V:ANTH · I:AGH |
| CM-010 | ElevenLabs | P1 | Teams synthesize approved speech or transcribe audio with consent, voice-identity, and cost controls. | TP+SK | R3/A1 | V:INT · P:N8N · I:AGH |
| CM-011 | Spotify | P1 | Users retrieve catalog and playback context and prepare governed playlist operations. | TP+SK+WS | R1/A1 | V:INT · I:AGH |
| CM-012 | YouTube | P0 | Creators retrieve YouTube channel and video data and prepare metadata, comments, uploads, or analytics actions for approval. | TP+SK+WS | R2/A1 | V:INT · P:N8N · I:AGH |
| CM-013 | Vimeo | P1 | Teams retrieve video libraries and prepare governed upload, metadata, privacy, and review actions. | TP+SK+WS | R2/A1 | V:INT · I:AGH |
| CM-014 | Moodle | P1 | Educators retrieve courses and learner activity and prepare reviewed content, feedback, or administrative updates. | TP+SK+WS | R3/A1 | P:N8N · I:AGH |
| CM-015 | Canvas LMS | P1 | Educators retrieve courses, assignments, and submissions and stage reviewed learning and grading actions. | TP+SK+WS | R3/A2 | P:N8N · I:AGH |
| CM-016 | Teachable | P2 | Creators retrieve course and enrollment state and prepare governed content or learner communications. | TP+SK+WS | R2/A2 | P:N8N · I:AGH |

### Research, science, and public data

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| RP-001 | Crossref | P0 | Researchers resolve DOIs and retrieve authoritative publication metadata for cited literature work. | TP+SK | R0/A0 | V:PUB · I:AGH |
| RP-002 | OpenAlex | P0 | Researchers query works, authors, institutions, topics, and citation relationships for reproducible evidence maps. | TP+SK | R0/A1 | V:PUB · I:AGH |
| RP-003 | Semantic Scholar | P1 | Researchers retrieve papers, authors, references, and recommendations with stable source identifiers. | TP+SK | R0/A1 | V:PUB · I:AGH |
| RP-004 | PubMed | P0 | Health and life-science researchers search biomedical citations and retrieve records without clinical inference. | TP+SK | R0/A0 | V:PUB · P:ANTH · I:AGH |
| RP-005 | Europe PMC | P1 | Researchers retrieve literature, grants, citations, and open-access links for evidence-backed synthesis. | TP+SK | R0/A0 | V:PUB · I:AGH |
| RP-006 | arXiv | P1 | Technical researchers search and retrieve preprint metadata and versions with explicit non-peer-reviewed status. | TP+SK+WS | R0/A0 | V:PUB · P:SKILL-A · I:AGH |
| RP-007 | bioRxiv | P1 | Life-science researchers monitor preprints and retrieve metadata while preserving preprint status and version provenance. | TP+SK+WS | R0/A0 | V:PUB · P:ANTH · I:AGH |
| RP-008 | ClinicalTrials.gov | P1 | Researchers retrieve registered trial records and changes without turning registry data into medical advice. | TP+SK+WS | R0/A0 | V:PUB · P:ANTH · I:AGH |
| RP-009 | ORCID | P1 | Researchers resolve contributor identities and, with explicit consent, prepare profile record updates. | TP+SK | R1/A1 | V:PUB · I:AGH |
| RP-010 | Zotero | P0 | Researchers retrieve and organize libraries, citations, notes, and attachments with collection-level scope. | TP+SK+WS | R1/A1 | V:OAI · V:PUB · I:AGH |
| RP-011 | OpenStreetMap | P0 | Users query open geospatial features for routing, place context, and local operations with attribution. | TP+SK | R0/A0 | V:PUB · V:INT · I:AGH |
| RP-012 | NASA Open Data | P1 | Researchers retrieve public NASA datasets and API results for sourced analysis and monitoring. | TP+SK+WS | R0/A1 | V:PUB · I:AGH |
| RP-013 | U.S. Census Data | P1 | Analysts retrieve public demographic and economic tables with geography, vintage, and variable provenance. | TP+SK | R0/A0 | V:PUB · I:AGH |
| RP-014 | World Bank Data | P1 | Analysts retrieve public development indicators with country, series, and revision provenance. | TP+SK+WS | R0/A0 | V:PUB · I:AGH |
| RP-015 | OECD Data Explorer | P2 | Policy researchers retrieve public statistical series while preserving dataset, edition, and jurisdiction context. | TP+SK+WS | R0/A0 | V:PUB · I:AGH |
| RP-016 | SEC EDGAR | P0 | Investors and researchers retrieve public filings and company facts for cited analysis, not investment execution. | TP+SK+WS | R0/A0 | V:PUB · I:AGH |

### Data platforms and analytics

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| DA-001 | Snowflake | P0 | Data teams run bounded read-only queries and retrieve governed metadata before enabling reviewed writes. | TP+SK | R3/A2 | V:INT · V:ANTH · I:AGH |
| DA-002 | Google BigQuery | P0 | Analysts run cost-bounded read-only queries and retrieve dataset and job metadata with project scope. | TP+SK | R3/A1 | V:ANTH · V:INT · I:AGH |
| DA-003 | Databricks | P0 | Data and AI teams retrieve governed catalog, job, and query results and stage approved workload actions. | TP+SK+WS | R3/A2 | V:INT · V:ANTH · I:AGH |
| DA-004 | Amazon Redshift | P1 | Data teams run bounded read-only queries and inspect clusters and workloads before any administrative action. | TP+SK | R3/A2 | V:INT · V:ANTH · I:AGH |
| DA-005 | PostgreSQL | P0 | Applications expose schema-constrained, parameterized database operations with read-only credentials by default. | TP+SK | R3/A0 | P:MCP-A · P:SKILL-A · I:AGH |
| DA-006 | MySQL | P1 | Applications expose allowlisted, parameterized queries with explicit database identity and read-only defaults. | TP+SK | R3/A0 | P:MCP-A · P:SKILL-A · I:AGH |
| DA-007 | MongoDB | P0 | Teams query scoped collections and prepare validated document changes without arbitrary database access. | TP+SK+WS | R3/A1 | V:INT · V:ANTH · I:AGH |
| DA-008 | ClickHouse | P1 | Analytics teams run cost-bounded read queries and inspect schemas, queries, and cluster health. | TP+SK | R3/A1 | V:INT · V:ANTH · I:AGH |
| DA-009 | Supabase | P0 | Builders query project data and inspect auth, storage, and edge-function state under project-scoped credentials. | TP+SK+WS | R3/A1 | V:INT · V:ANTH · I:AGH |
| DA-010 | Elasticsearch | P1 | Search teams query indices and inspect mappings and cluster state, with mutations separately gated. | TP+SK+WS | R3/A1 | V:INT · P:MCP-A · I:AGH |
| DA-011 | dbt Cloud | P1 | Analytics engineers inspect models and jobs and trigger approved runs with run and environment provenance. | TP+SK+WS | R3/A2 | P:N8N · P:SKILL-A · I:AGH |
| DA-012 | Fivetran | P1 | Data teams inspect connectors and syncs and trigger or pause approved replication operations. | TP+SK+WS | R3/A2 | V:INT · I:AGH |
| DA-013 | Airbyte | P1 | Teams inspect open-source or cloud connections and launch approved syncs with source and destination identity. | TP+SK+WS | R3/A1 | V:INT · P:MCP-A · I:AGH |
| DA-014 | Twilio Segment | P1 | Data teams inspect sources, destinations, schemas, and delivery health and stage governed tracking changes. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| DA-015 | Amplitude | P1 | Product teams query behavioral analytics and prepare reviewed cohorts, annotations, or experiment inputs. | TP+SK+WS | R1/A1 | V:OAI · V:ANTH · V:INT · I:AGH |
| DA-016 | PostHog | P0 | Product teams query analytics, feature flags, experiments, and session metadata with project-scoped access. | TP+SK+WS | R3/A1 | V:INT · P:MCP-A · I:AGH |

### Engineering, cloud, observability, and security

| ID | Candidate | P | User and outcome | AGH surface | Risk / access | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| EC-001 | GitLab | P0 | Engineering teams inspect repositories, issues, merge requests, pipelines, and deployments and stage reviewed changes. | TP+SK+WS | R3/A1 | V:ANTH · V:INT · I:AGH |
| EC-002 | Bitbucket | P1 | Teams inspect repositories and pull requests and prepare governed review, branch, and pipeline actions. | TP+SK+WS | R3/A1 | V:INT · P:MCP-A · I:AGH |
| EC-003 | Jira | P0 | Teams search work items and prepare approved issue, comment, transition, and planning updates. | TP+SK+WS | R2/A1 | V:OAI · V:ANTH · V:INT · I:AGH |
| EC-004 | Azure DevOps | P1 | Enterprises inspect boards, repositories, pipelines, and artifacts and stage governed delivery actions. | TP+SK+WS | R3/A2 | V:INT · P:MCP-A · I:AGH |
| EC-005 | Sentry | P0 | Teams retrieve errors and traces, correlate regressions, and prepare reviewed issue and release actions. | TP+SK+WS | R2/A1 | V:OAI · V:INT · I:AGH |
| EC-006 | Datadog | P0 | Operators retrieve metrics, logs, traces, incidents, and monitors and stage reviewed operational changes. | TP+SK+WS | R3/A2 | V:INT · V:ANTH · I:AGH |
| EC-007 | PagerDuty | P1 | Incident teams retrieve on-call and incident state and execute approved acknowledgements, notes, or escalations. | TP+SK+WS | R3/A2 | V:INT · P:N8N · I:AGH |
| EC-008 | Grafana | P1 | Teams query dashboards, metrics, alerts, and incidents and stage governed observability updates. | TP+SK+WS | R3/A1 | V:INT · V:ANTH · P:MCP-A · I:AGH |
| EC-009 | Cloudflare | P0 | Operators inspect zones, DNS, traffic, security, and Workers and stage tightly approved infrastructure changes. | TP+SK+WS | R3/A1 | V:OAI · V:INT · I:AGH |
| EC-010 | Vercel | P0 | Teams inspect projects and deployments and trigger or promote approved builds with environment provenance. | TP+SK+WS | R3/A1 | V:OAI · V:INT · I:AGH |
| EC-011 | Amazon Web Services | P0 | Operators expose allowlisted, account- and region-scoped cloud operations with read-only roles by default. | TP+SK+WS | R3/A2 | V:ANTH · V:INT · I:AGH |
| EC-012 | Microsoft Azure | P1 | Operators expose allowlisted, subscription-scoped cloud queries and separately approved mutations. | TP+SK+WS | R3/A2 | V:ANTH · V:INT · I:AGH |
| EC-013 | Google Cloud | P1 | Operators expose allowlisted, project-scoped cloud queries and separately approved mutations. | TP+SK+WS | R3/A2 | V:INT · P:MCP-A · I:AGH |
| EC-014 | Okta | P1 | Identity teams inspect users, groups, apps, and events; access changes require mandatory explicit approval. | TP+SK+WS | R3/A2 | V:INT · V:ANTH · I:AGH |
| EC-015 | 1Password | P1 | Teams retrieve permitted secret references and item metadata without disclosing values outside approved tool calls. | TP+SK+WS | R3/A2 | V:INT · V:ANTH · I:AGH |
| EC-016 | HashiCorp Vault | P1 | Operators retrieve scoped dynamic credentials or secret references and inspect lease health without broad secret export. | TP+SK+WS | R3/A1 | P:MCP-A · P:SKILL-A · I:AGH |

## Portfolio decisions

P0 is a portfolio tier, not a direction to build all 75 P0 rows at once. The first implementation tranche should prove the safety and authoring substrate, then select a small provider set that unlocks several outcome bundles.

### Recommended sequence

1. **Platform-enabling work (superseded by the 2026-07-20 decision above):** the only accepted new core investment is **PE-015 liveness/health only** (credential/secret liveness). PE-004, PE-007, and PE-010 already ship. All other PE rows are **rejected — do not build**. The original sequencing text below is retained only as historical context and is no longer a directive.

   > ~~Trust and permission foundation: PE-006, PE-009, PE-010, PE-011, PE-013, PE-015. Authoring and event foundation: PE-001, PE-002, PE-004, PE-007, PE-008, PE-017, PE-018.~~ Superseded.
3. **Personal, household, and team core:** PW-001, PW-002, PW-003, PW-006, PW-007, PW-009, PW-010, PW-011, and PW-017. Together they cover email, calendar, files, enterprise knowledge, lightweight databases, both Google and Microsoft audiences, and a self-hosted home-automation gateway.
4. **Business system anchors:** choose one initial CRM from SR-001 through SR-004, one support desk from CS-001, CS-002, CS-004, or CS-005, Shopify (CP-001), Stripe (CP-007), and one accounting provider from FL-001 or FL-002. Provider choice should follow design-partner access, not catalog popularity alone.
5. **Visible-output providers:** Zoom or one meeting-record provider, Figma or Canva, Cloudinary, and YouTube make meeting, campaign, creator, and launch bundles produce inspectable results.
6. **Technical operator set:** DA-001, DA-002, DA-003, EC-001, EC-003, EC-005, EC-006, EC-009, EC-010, and EC-011 provide a coherent read-first data and delivery collection without duplicating the existing GitHub bridge.
7. **Long-tail expansion:** add P1 and P2 providers only when a maintained bundle, design partner, or verified demand signal supplies an owner, access path, conformance fixtures, and first-success outcome.

### Selection gates

A candidate is ready to move from catalog to TechSpec only when all of the following are answered:

1. **Upstream authority:** Which provider-owned API, MCP server, GraphQL endpoint, webhook, CLI, or supported export is used? What terms, plans, regions, app reviews, and partner agreements apply?
2. **Atomic boundary:** What single service or provider responsibility does the extension own? Which persona, schedule, and multi-service outcome remain in a bundle?
3. **Operation allowlist:** Which reads and writes ship first? Which R2 or R3 operations are omitted, draft-only, or approval-gated?
4. **Credential lifecycle:** How are scopes acquired, tenant/account identity selected, secrets bound, health checked, rotated, and revoked?
5. **Workspace isolation:** Is each credential and datum global-, workspace-, session-, or agent-scoped? How are workspace identity, pagination, caches, events, and delivery kept isolated?
6. **Operational contract:** What are the provider's pagination, rate-limit, retry, idempotency, webhook verification, cursor, retention, and deletion semantics?
7. **Agent manageability:** How can an agent install, inspect provenance, connect, verify health, list tools, disable, update, and remove the extension through structured CLI/HTTP/UDS/native-tool paths?
8. **Conformance evidence:** Do positive, negative, expired-credential, denied-scope, malformed-response, partial-failure, cleanup, and removal cases pass against a representative provider contract?
9. **First success:** What fixture-backed demo and real-data read-only dry run prove useful value before any external mutation?
10. **Ownership:** Who maintains provider drift, security advisories, schema changes, support, and deprecation?

An A3 row should remain discovery-only until upstream access is demonstrated. A catalog listing, community template, or closed SaaS UI is not evidence that a maintainable integration can be shipped.

## Evidence appendix

Evidence was recorded at catalog or documentation level, not as a full technical evaluation of every provider. A verified marker confirms only the narrow statement described below. Product prioritization and AGH packaging remain inference.

### Local AGH sources

| Code | Source | Verified fact used here |
| --- | --- | --- |
| V:AGH | [Extension manifest](../../internal/extension/manifest.go), [bundle specification](../../internal/extension/bundle.go), [extension grants](../../internal/extension/capability.go), and [dev-cycle reference extension](../../extensions/dev-cycle/extension.json) | Current resource fields, subprocess lifecycle, provider contracts, Host API grants, and tool/watch-source precedent. |
| V:AGH | [Extension development guide](../../packages/site/content/runtime/core/extensions/develop.mdx), [install and trust guide](../../packages/site/content/runtime/core/extensions/install.mdx), and [capabilities and bundles reference](../../skills/agh/references/capabilities-and-bundles.md) | Publicly documented extension lifecycle, stricter marketplace grants, agent manageability, bundle projection, and current bridge authoring limitation. |
| V:AGH | [Bridge provider baseline](../../extensions/bridges/README.md) | Eight production in-tree providers and the explicit build/install process; released artifacts do not include or auto-install their executables. |

### Primary ecosystem sources

| Code | Source | Verified fact used here |
| --- | --- | --- |
| V:INT | [integrations.sh](https://integrations.sh/), [`api.json`](https://integrations.sh/api.json), [registry revision `7ae23b7c`](https://github.com/UsefulSoftwareCo/integrations/commit/7ae23b7cb75c0b62fd32a08bb1e241f85b829d8e), and [publishing guide](https://integrations.sh/publishing/) | A named service or directly related official integration surface appeared in the time-bound snapshot; the registry models MCP, API/OpenAPI, GraphQL, CLI, authentication, and detected/discovered provenance. On 2026-07-17, the API payload (`generatedAt` 2026-07-08T01:44:23.703Z; SHA-256 `887d050c487bacae08c1bc708daa3ea06e86780d513725671ab72fd85e72109e`) contained 5,758 surfaces. The homepage published 3,230 domains and had SHA-256 `c5705d3346b4f60c3f9937bbc83e67016bb628b67455e89281f104ec0a76acc4`; that domain count is an observed publication value, not derivable from the Git checkout. See the [reproduction commands](README.md#reproducing-volatile-catalog-counts). |
| P:INT | The same integrations.sh sources | Pattern evidence for credential-first discovery, provenance, and declared service dependencies; not evidence that AGH should execute contributed code unchanged. |
| V:OAI | [OpenAI plugin guidance](https://help.openai.com/en/articles/20001256-plugins-in-codex/), [build guide](https://learn.chatgpt.com/docs/build-plugins), [submission guide](https://learn.chatgpt.com/docs/submit-plugins), [official plugin repository](https://github.com/openai/plugins), and [manifest at revision `11c74d6b`](https://github.com/openai/plugins/blob/11c74d6ba24d3a6d48f54a194cd00ef3beea18f9/.agents/plugins/marketplace.json) | A named plugin or marketplace pattern appeared in the official repository snapshot. The pinned manifest contains 180 entries across 11 categories and has SHA-256 `0b19caddb65a6125b7af3138634ba78da1c0fb204290e0599fa9a6fdc4c5258f`; see the [reproduction commands](README.md#reproducing-volatile-catalog-counts). |
| P:OAI | [OpenAI role-specific plugins](https://github.com/openai/role-specific-plugins) | Role packaging and recurring-job precedent, not a claim about provider access or AGH demand. |
| V:ANTH | [Claude plugin documentation](https://code.claude.com/docs/en/plugins), [plugin reference](https://code.claude.com/docs/en/plugins-reference), [official plugin directory](https://github.com/anthropics/claude-plugins-official), and [manifest at revision `ded0c09c`](https://github.com/anthropics/claude-plugins-official/blob/ded0c09c1ff6003a16d52fc28eae33ed55e4eb87/.claude-plugin/marketplace.json) | A named plugin or marketplace pattern appeared in the official directory snapshot. The pinned manifest contains 256 entries and has SHA-256 `0fdcf8b09c97b13b1a79bb77fa7c752ca038bcbbab6bd1ed4292f8841fd81bfc`; see the [reproduction commands](README.md#reproducing-volatile-catalog-counts). |
| P:ANTH | [Anthropic knowledge-work plugins](https://github.com/anthropics/knowledge-work-plugins) | Official role-pack precedent for productivity, sales, support, product, marketing, legal, finance, data, enterprise search, and life-science jobs; not provider-access validation. |
| V:MCP | [Official MCP Registry](https://github.com/modelcontextprotocol/registry) | Registry-scale server discovery and publisher namespace ownership through GitHub/OIDC, DNS, or HTTP verification. |
| V:PUB | Provider-maintained API documentation for [Crossref](https://www.crossref.org/documentation/retrieve-metadata/rest-api/), [OpenAlex](https://developers.openalex.org/), [Semantic Scholar](https://www.semanticscholar.org/product/api), [NCBI/PubMed](https://www.ncbi.nlm.nih.gov/books/NBK25501/), [Europe PMC](https://europepmc.org/RestfulWebService), [arXiv](https://info.arxiv.org/help/api/), [bioRxiv](https://api.biorxiv.org/), [ClinicalTrials.gov](https://clinicaltrials.gov/data-api/about-api), [ORCID](https://info.orcid.org/documentation/integration-guide/), [Zotero](https://www.zotero.org/support/dev/web_api/v3/start), [OpenStreetMap Overpass](https://wiki.openstreetmap.org/wiki/Overpass_API), [NASA](https://api.nasa.gov/), [U.S. Census](https://www.census.gov/data/developers.html), [World Bank](https://datahelpdesk.worldbank.org/knowledgebase/articles/889392), [OECD](https://www.oecd.org/en/data/insights/data-explainers/2024/09/api.html), and [SEC EDGAR](https://www.sec.gov/search-filings/edgar-application-programming-interfaces) | A provider-maintained public data or API surface exists. Public availability does not remove authentication, attribution, rate-limit, privacy, or permitted-use obligations. |

### Community and breadth signals

| Code | Source | Interpretation |
| --- | --- | --- |
| P:N8N | [awesome-n8n-templates](https://github.com/ScraperNode/awesome-n8n-templates) | Community automation breadth across business and personal domains. Templates are not security, quality, API-access, or maintenance endorsements. |
| P:MCP-A | [punkpeye/awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers) and [wong2/awesome-mcp-servers](https://github.com/wong2/awesome-mcp-servers) | Community breadth for MCP-accessible systems. Every server still needs origin, permission, dependency, and maintenance review. |
| P:SKILL-A | [awesome-claude-skills](https://github.com/ComposioHQ/awesome-claude-skills) and [awesome-claude-code](https://github.com/subinium/awesome-claude-code) | Community breadth for reusable operational skills and developer tooling, not proof of a production connector. |
| P:HER | [Hermes Home Assistant plugin](../../.resources/hermes/plugins/platforms/homeassistant/), [tool registration](../../.resources/hermes/tools/homeassistant_tool.py), and [fake integration server](../../.resources/hermes/tests/fakes/fake_ha_server.py) | Concrete local precedent for credential-gated Home Assistant state retrieval, events, and device actions; not evidence of an AGH implementation. |
| I:AGH | This catalog, informed by the current AGH model and [ecosystem opportunity map](README.md) | Product inference covering candidate selection, atomic boundary, priority, outcome, implementation surface, risk, access class, and bundle leverage. |

## Catalog audit

The catalog was mechanically checked after authoring:

| Audit | Result |
| --- | --- |
| Existing baseline rows | 8, identifiers BL-001 through BL-008 |
| New platform-enabling rows | 18, identifiers PE-001 through PE-018 |
| New atomic service/provider rows | 193 across twelve domain prefixes |
| Total new opportunity rows | 211 |
| Priority distribution | P0: 75 · P1: 114 · P2: 22 |
| Domain distribution | PW has 17 rows; KN, MT, SR, MG, CS, CP, FL, CM, RP, DA, and EC have 16 each |
| Candidate-name duplicates | 0 exact case-insensitive duplicates |
| Baseline collisions | 0 candidate names equal Discord, Google Chat, GitHub, Linear, Slack, Microsoft Teams, Telegram, or WhatsApp |
| Evidence typing | Every new row includes I:AGH; verified and pattern codes remain explicitly distinguishable |
| Identifier continuity | Every declared prefix is gap-free and unique |

This is a catalog-shape audit, not provider conformance or market validation. No upstream account was connected and no candidate integration was executed.

## AGH Impact Audit

- **Native tools:** no runtime impact. This document adds no agh__ tool IDs, descriptors, schemas, digests, risk flags, availability diagnostics, capability gates, or fallbacks.
- **Extensibility and hooks:** no runtime impact. The current manifest, provider contracts, typed hooks, registries, bridge authoring limit, MCP packaging, bundle projection, trust model, and config lifecycle were reviewed; no extension or registry behavior changes.
- **Workspace data isolation:** no runtime datum is added. Every future connector must classify credential and data scope and prove workspace identity through CLI, HTTP/UDS, core, store, cache, events, and delivery paths.
- **Official AGH skill:** no impact. The canonical capabilities-and-bundles reference was checked; no public behavior or agent guidance changed.

## Web, docs, config, and QA impact

- web/: no route, component, hook, query cache, or user-visible behavior changes.
- packages/site/: no public documentation or shipped-support claim changes.
- config.toml: no keys, defaults, lifecycle entries, or examples change.
- docs/qa/scenarios/: no user-visible runtime behavior changes, so no scenario is added or reset.
