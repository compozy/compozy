# AGH Bundle Opportunities

- **Language:** English · [Português (Brasil)](ptbr/bundle-opportunities.md)

> Status: opportunity catalog and product-planning reference. This document does not claim that the integrations or bundles described below are currently available in AGH.

## Purpose

AGH already supports extension-shipped bundle profiles that project a coordinated runtime shape. No first-party extension in this repository currently ships a bundle profile. The next product question covers both the integrations to add and the complete outcomes those integrations should support for technical and nontechnical users.

This catalog separates two layers:

- **Extensions** provide reusable resources and service surfaces such as agents, providers, tools, hooks, skills, Loops, MCP servers, bridge adapters, and external-system connectors. An extension may also ship bundle definitions.
- **Bundles** package an outcome for a persona by selecting and configuring agents, authored context, jobs, triggers, AGH Network channels, and bridge preset templates around already installed dependencies.

The catalog is intentionally broader than a roadmap. It is a source of candidates to validate, combine, reject, or sequence. Evidence codes point to the implementation boundary in AGH or to patterns observed in Hermes, Pi, OpenClaw, and Paperclip. External evidence supports the opportunity pattern; it does not prove that the corresponding AGH integration exists.

## Runtime truth and fit model

### What a bundle profile can project today

The current profile schema in AGH can project only:

- declared AGH Network channels, including one primary channel;
- packaged agents and their Soul and Heartbeat sidecars;
- package-managed automation jobs;
- package-managed automation triggers;
- disabled bridge instances materialized from package-managed presets. Preset secret slots remain catalog and setup metadata; activation does not bind them.

These resources are declared by an already installed owning extension. A bundle profile does **not** install another extension, resolve dependencies, bind an account or secret, run a dependency health check, or enable bridge delivery. Static skills, Loops, hooks, tools, and MCP server resources remain extension-scoped; activating one profile does not install or selectively scope those resources to that profile. See [AGH-1], [AGH-2], and [AGH-3].

This distinction matters in the catalog. A blueprint can be valuable without fitting the current activation schema end to end.

### Fit labels

- **Current** — the blueprint can be represented by one installed owning extension, a configured ACP runtime/provider for each packaged agent, and the current profile projection. The owning extension may package static skills, Loops, hooks, tools, or MCP servers, but those resources remain extension-scoped rather than profile-scoped.
- **Current with preinstalled dependencies** — the profile can project agents, AGH Network channels, jobs, triggers, and disabled bridge instances today, but every required connector and every static skill, Loop, hook, tool, or MCP server must already be installed, configured, authorized, and healthy. Those static resources remain extension-scoped, and bridge delivery still requires separate secret binding and enablement.
- **Platform evolution** — a trustworthy, portable version requires one or more unshipped platform features: dependency installation, dependency locking, profile-scoped static resources, provider service contracts, guided setup, OAuth/account selection, health checks, dry runs, update ownership, direct Loop targets from bundle automation, or transactional rollback.

The second label is intentionally literal: **Current with installed, configured, authorized, and healthy dependencies (static skill/Loop/hook/tool/MCP resources remain extension-scoped; bridge delivery remains disabled until setup completes)**.

Current bundle jobs target either a packaged agent or an explicit task configuration; current bundle triggers target a packaged agent. Neither profile schema can target a Loop directly. When a current-fit blueprint names a Loop, its automation targets an agent that has access to `agh__loop_run`; that agent starts the extension-scoped Loop. A direct job-to-Loop or trigger-to-Loop binding is platform evolution.

## Design rules for a useful bundle

An outcome bundle should answer the following questions before it is considered installable:

1. **Persona:** who owns the outcome and who can approve consequential actions?
2. **First proof:** what small, inspectable result demonstrates value after setup?
3. **Dependencies:** which extensions, provider accounts, local binaries, and secrets are required or optional?
4. **Agents and authored context:** which roles exist, which persona, principles, and constraints each Soul expresses, and what bounded reentry guidance each Heartbeat supplies after a runtime wake? Operational authority belongs to `AGENT.md`, approvals, grants, and the runtime.
5. **Skills and peer delegation:** which local procedural skills are needed, and which interpretive AGH Network Capabilities may be delegated to a peer?
6. **Loop:** what deterministic goal, verification gate, terminal states, and budget define completion?
7. **Automation:** which jobs and triggers observe state or start work, which agent or task each one targets, and how are retries, catch-up, and idempotency bounded?
8. **AGH Network channels and external bridges:** where does the user coordinate inside AGH, where are results delivered externally, and how is identity mapped?
9. **Approvals:** what may be read automatically and what must be confirmed before mutation, sending, publishing, spending, or deletion?
10. **Data boundary:** is each datum global-, workspace-, session-, or agent-scoped?
11. **Failure behavior:** what pauses, escalates, or opens a circuit when a dependency is unavailable?
12. **Ownership:** what can an update replace, and what user-authored data must remain untouched?

## Priority lenses

Candidates should be compared on more than market size:

| Lens | Strong signal |
| --- | --- |
| First-proof speed | A user can inspect a useful result in less than ten minutes |
| Outcome clarity | The user describes a job, not a collection of protocols |
| Reuse | Several bundles can share the same extension contracts |
| Safe autonomy | Read-only value arrives before broad mutation access |
| Recurrence | The outcome benefits from jobs, triggers, or durable Loops; Heartbeat policy supplies bounded reentry guidance after a wake |
| Delivery fit | The result arrives through the local AGH surface or external bridge where the persona already works |
| Evidence | The bundle links every conclusion or action to source data |
| Isolation | Client, family, tenant, and workspace data cannot cross boundaries |
| Recovery | A failed run can resume, no-op, or escalate without duplicating side effects |
| Community potential | A domain expert can improve the bundle without maintaining every connector |

# Flagship blueprints

Each flagship is detailed enough to support product discovery or a future TechSpec. Identifiers are proposals, not reserved public IDs.

## Personal and family

### 1. personal-chief-of-staff

- **Persona:** an independent professional, founder, caregiver, or busy household organizer.
- **Outcome and first proof:** produce a cited morning brief with the day's calendar, urgent inbox items, outstanding commitments, and three suggested priorities. The first proof is a read-only brief delivered locally or through a chosen messaging bridge.
- **Extensions:** calendar, email, tasks/reminders, contacts, documents, weather, and an optional messaging bridge.
- **Agents and authored context:** a chief-of-staff agent whose Soul expresses a prioritization-and-drafting posture, plus an administrative researcher agent; runtime grants and approvals enforce authority. Heartbeat reentry guidance covers overdue commitments and connection diagnostics after a runtime wake without sending messages.
- **Skills and task behaviors:** inbox triage, calendar preparation, commitment extraction, source citation, and concise briefing skills; trusted peers may optionally advertise calendar-read and task-read AGH Network Capabilities.
- **Loop:** collect current state, deduplicate commitments, rank by urgency and importance, verify every item against a source, render the brief, and stop with done, no-op, blocked, or failed.
- **Jobs and triggers:** weekday morning job, evening preparation job, and optional calendar-change or high-priority-message triggers with deduplication.
- **Channel and bridge:** local AGH Network channel by default; optional Telegram, WhatsApp, Slack, or email bridge preset.
- **Approvals:** no approval for read-only briefing; approval required before sending a reply, moving a meeting, creating an external task, or sharing personal data.
- **Safety defaults:** read-only connections, one brief per period, no automatic catch-up flood, redaction of sensitive message bodies, workspace-scoped memory, and a cost ceiling.
- **Current AGH fit:** **Platform evolution** — a portable version needs dependency installation, account setup, provider contracts, connection health checks, and a dry-run path.
- **Evidence:** [H-4], [OC-2], [OC-3], [PI-3].

### 2. family-command-center

- **Persona:** a household coordinating school, health appointments, chores, groceries, and shared events.
- **Outcome and first proof:** create one conflict-checked weekly family plan. The first proof is a read-only calendar and task summary with clearly marked unknowns.
- **Extensions:** shared calendar, reminders/tasks, email, school-message ingestion, grocery list, weather, and family messaging.
- **Agents and authored context:** a family coordinator whose Soul expresses authored constraints against unilateral purchases or schedule changes, plus optional school and household agents; runtime approvals enforce those boundaries. Heartbeat reentry guidance covers tomorrow's conflicts and unacknowledged obligations after a runtime wake.
- **Skills and task behaviors:** schedule reconciliation, age-appropriate summaries, grocery consolidation, document extraction, and privacy-aware notification.
- **Loop:** gather household inputs, normalize people and times, detect conflicts, request missing information, verify the plan, and publish only after the organizer confirms.
- **Jobs and triggers:** Sunday planning job, nightly next-day job, school-email trigger, and severe-weather trigger.
- **Channel and bridge:** one private family AGH Network channel plus optional per-person direct-delivery bridges.
- **Approvals:** organizer approval before adding or changing events, sharing children's information, ordering items, or contacting third parties.
- **Safety defaults:** private workspace, least-data summaries, no location tracking by default, quiet hours, per-recipient visibility, and no medical inference.
- **Current AGH fit:** **Platform evolution** — setup, household identity mapping, per-recipient policy, connector installation, and profile-scoped resources are required.
- **Evidence:** [H-4], [OC-3], [OC-7].

### 3. inbox-zero

- **Persona:** a professional or small-team operator with a high-volume inbox.
- **Outcome and first proof:** classify a bounded sample of messages into action, waiting, reference, and ignore, with suggested drafts for the action set. The first proof does not mutate the mailbox.
- **Extensions:** Gmail, Outlook, or IMAP; tasks; calendar; contacts; optional Slack or Teams bridge.
- **Agents and authored context:** a triage agent, a reply drafter, and an escalation reviewer; their Souls express the distinction between factual classification and representation of the user, while runtime approvals control external actions. Heartbeat reentry guidance covers backlog age and connection failures after a runtime wake.
- **Skills and task behaviors:** thread summarization, sender/context resolution, commitment extraction, draft writing, and duplicate detection.
- **Loop:** fetch a bounded batch, classify, verify high-risk classifications, create draft suggestions, wait for approval, apply approved actions, and record evidence.
- **Jobs and triggers:** scheduled triage windows, VIP sender trigger, and end-of-day unresolved-item job.
- **Channel and bridge:** local review queue with optional Slack, Teams, Telegram, or email delivery bridge.
- **Approvals:** required before send, delete, archive outside an approved rule, unsubscribe, or create a third-party task.
- **Safety defaults:** start with a sample folder or label, retain original messages, cap batch size, never infer consent from silence, and stop on mailbox identity mismatch.
- **Current AGH fit:** **Current with preinstalled dependencies** — the profile can project agents, jobs, and triggers; email tools, the triage skill, and the deterministic Loop remain extension-scoped. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; any delivery bridge remains disabled until secrets are bound and setup explicitly enables it.
- **Evidence:** [H-4], [OC-2], [OC-6].

### 4. travel-concierge

- **Persona:** an individual, family, or executive assistant planning and monitoring a trip.
- **Outcome and first proof:** produce an itinerary draft that reconciles reservation documents, calendar constraints, transit time, and explicit preferences.
- **Extensions:** email, calendar, maps, weather, document extraction, travel search, notes, and messaging.
- **Agents and authored context:** a travel planner, a reservation verifier, and a disruption monitor; their Souls express a no-purchase and no-cancellation posture, while runtime approvals enforce authority. Heartbeat reentry guidance applies only during active-trip windows after a runtime wake.
- **Skills and task behaviors:** itinerary planning, reservation extraction, timezone reconciliation, accessible-travel checks, and source citation.
- **Loop:** collect constraints, build an itinerary, verify every booking fact, surface conflicts, obtain approval, and monitor confirmed plans.
- **Jobs and triggers:** pre-trip checklist jobs, day-before brief, reservation-change trigger, and severe-disruption trigger.
- **Channel and bridge:** private AGH Network travel channel with a mobile-messaging bridge.
- **Approvals:** mandatory before booking, cancellation, payment, sharing passport data, or messaging a host or provider.
- **Safety defaults:** no autonomous purchase, minimize identity-document retention, no background location access, and expire trip-specific state after the retention window.
- **Current AGH fit:** **Platform evolution** — travel provider contracts, guided setup, account linking, purchase policy, and data-retention controls are required.
- **Evidence:** [H-4], [OC-3], [PI-3].

### 5. home-automation-concierge

- **Persona:** a homeowner, renter, or caregiver using a smart-home controller.
- **Outcome and first proof:** generate a read-only home status digest and propose one bounded automation, such as a lighting or energy schedule.
- **Extensions:** Home Assistant, optional Hue or media providers, weather, notifications, and sensor-event ingress.
- **Agents and authored context:** a home coordinator whose Soul expresses the distinction between advisory, reversible, and safety-critical actions, plus a device specialist; runtime policy and approvals enforce the action boundary. Heartbeat reentry guidance covers unavailable-device diagnostics and repeated-event evidence after a runtime wake.
- **Skills and task behaviors:** state interpretation, scene planning, energy summaries, device naming, and incident escalation.
- **Loop:** inspect current state, validate device identity, simulate the proposed action, obtain approval, apply, verify observed state, and roll back when supported.
- **Jobs and triggers:** daily status job, occupancy-independent energy job, device-offline trigger, and user-defined event triggers with cooldowns.
- **Channel and bridge:** private local AGH Network channel plus an optional mobile-notification bridge.
- **Approvals:** required for locks, alarms, doors, cameras, heating extremes, purchases, and any automation involving another person.
- **Safety defaults:** read-only initial mode, explicit device allowlist, cooldown and idempotency keys, no occupancy inference in outbound messages, and a circuit breaker for repeated commands.
- **Current AGH fit:** **Current with preinstalled dependencies** — a profile can project the agent, jobs, triggers, and a disabled bridge instance after the smart-home extension and its static resources are installed, configured, authorized, and healthy. Jobs and triggers target the agent, which starts the extension-scoped Loop through `agh__loop_run`; bridge delivery still requires separate secret binding and enablement.
- **Evidence:** [H-6], [OC-7].

## Local business and regulated administration

### 6. local-business-front-desk

- **Persona:** the owner or receptionist of a clinic, salon, workshop, restaurant, studio, or home-services business.
- **Outcome and first proof:** answer a test inquiry from an approved FAQ, collect contact and scheduling needs, and create a draft booking without confirming it.
- **Extensions:** WhatsApp, SMS, voice, email, calendar/booking, contacts/CRM, payments, and an approved knowledge source.
- **Agents and authored context:** a receptionist, a qualification agent, and an owner-escalation agent; the receptionist Soul expresses represented business identity and authored constraints against prohibited claims, while runtime policy controls representation. Heartbeat reentry guidance covers unhandled conversations and booking conflicts after a runtime wake.
- **Skills and task behaviors:** FAQ retrieval, intake, schedule lookup, quote drafting, language detection, and escalation.
- **Loop:** authenticate bridge and customer identity, understand the request, answer from evidence, collect missing fields, propose a slot or next step, wait for approval where required, and confirm delivery.
- **Jobs and triggers:** inbound-message and missed-call triggers, appointment reminder jobs, and end-of-day unresolved-lead digest.
- **Channel and bridge:** one or more customer-facing bridges plus a private owner AGH Network channel.
- **Approvals:** required for nonstandard pricing, refunds, commitments outside policy, sensitive-data disclosure, or outbound marketing.
- **Safety defaults:** no invented availability or prices, no automatic payment capture, quiet hours, per-customer session isolation, human fallback, and delivery idempotency.
- **Current AGH fit:** **Platform evolution** — messaging-bridge setup, business-system dependencies, identity mapping, knowledge-source setup, and reusable provider contracts are required.
- **Evidence:** [OC-3], [OC-5], [PC-4].

### 7. real-estate-agent-desk

- **Persona:** an independent real-estate agent or small brokerage team.
- **Outcome and first proof:** produce a sourced daily lead and showing brief with stale follow-ups and document gaps.
- **Extensions:** CRM, listings feed, email, SMS/WhatsApp, calendar, maps, documents, and e-signature.
- **Agents and authored context:** a lead coordinator, listing researcher, and transaction checklist agent; their Souls express fair-housing and source-grounding constraints, while policy and approvals enforce represented actions. Heartbeat reentry guidance covers expiring contingencies and unanswered leads after a runtime wake.
- **Skills and task behaviors:** lead qualification, listing comparison, showing preparation, document checklist, and follow-up drafting.
- **Loop:** ingest a lead or transaction change, resolve identity, gather facts, propose the next action, obtain approval, update the system of record, and verify delivery.
- **Jobs and triggers:** new-lead trigger, showing reminder, daily pipeline job, and deadline-risk trigger.
- **Channel and bridge:** CRM-linked workspace with a private AGH Network coordination channel and approved customer-messaging bridges.
- **Approvals:** required before client communication, offer language, document submission, pricing representation, or disclosure of protected information.
- **Safety defaults:** evidence links for property facts, fair-housing policy checks, tenant/client isolation, no legal advice, and no signature or offer submission without a human.
- **Current AGH fit:** **Platform evolution** — listings and CRM contracts, tenant identity, compliance gates, setup, and dependency installation are needed.
- **Evidence:** [OC-2], [OC-3], [PC-2].

### 8. hospitality-guest-desk

- **Persona:** a hotel, short-term rental, hostel, or small hospitality operator.
- **Outcome and first proof:** answer a sandboxed guest question from property policy and generate an arrival brief for one test reservation.
- **Extensions:** property-management system, booking providers, email and messaging bridge adapters, maps, payments, maintenance tasks, and approved property knowledge.
- **Agents and authored context:** a guest concierge, reservation verifier, and maintenance dispatcher; their Souls express property-specific persona and constraints, while runtime grants and approvals define authority. Heartbeat reentry guidance covers arrivals, unresolved incidents, and integration diagnostics after a runtime wake.
- **Skills and task behaviors:** reservation lookup, multilingual FAQ, local recommendations, maintenance intake, and handoff.
- **Loop:** verify guest and reservation, answer from policy, record the interaction, create a bounded request when needed, and confirm resolution or escalation.
- **Jobs and triggers:** booking-created trigger, pre-arrival and checkout jobs, inbound-message trigger, and urgent-maintenance trigger.
- **Channel and bridge:** guest-messaging bridges with a separate private operations AGH Network channel.
- **Approvals:** required for refunds, compensation, access-code changes, emergency actions outside policy, and sharing guest data.
- **Safety defaults:** reservation verification before disclosure, redacted access information, no autonomous refund, quiet-hour routing, and incident escalation.
- **Current AGH fit:** **Platform evolution** — PMS/booking contracts, identity verification, multi-tenant setup, and provider-specific onboarding are required.
- **Evidence:** [OC-3], [H-4], [PC-2].

### 9. clinic-admin-assistant

- **Persona:** a healthcare practice administrator handling scheduling and documents, not clinical decisions.
- **Outcome and first proof:** produce a read-only list of incomplete administrative intake items and upcoming scheduling conflicts.
- **Extensions:** scheduling, approved patient communication, forms/documents, billing administration, and task management.
- **Agents and authored context:** an administrative coordinator and document checker; their Souls express authored constraints against diagnosis, treatment advice, and clinical prioritization, while runtime policy and approvals enforce scope. Heartbeat reentry guidance covers failed reminders and missing administrative fields after a runtime wake.
- **Skills and task behaviors:** form completeness, scheduling, policy-grounded FAQ, reminder drafting, and secure escalation.
- **Loop:** verify patient identity and scope, inspect administrative records, flag missing items, draft an approved next step, and stop before any clinical interpretation.
- **Jobs and triggers:** appointment reminder jobs, form-received trigger, cancellation trigger, and daily administrative exception report.
- **Channel and bridge:** approved patient-communication bridge and a private staff AGH Network channel.
- **Approvals:** required before external messages beyond approved templates, record changes, billing commitments, or disclosure to another party.
- **Safety defaults:** minimum necessary data, workspace isolation, no clinical inference, no emergency triage, immutable audit references, and immediate human escalation for medical content.
- **Current AGH fit:** **Platform evolution** — regulated-data controls, identity, provider connectors, policy enforcement, setup, and compliance validation are prerequisites.
- **Evidence:** [OC-5], [PC-2].

### 10. legal-intake-coordinator

- **Persona:** a law office administrator or attorney receiving prospective-client inquiries.
- **Outcome and first proof:** turn a test intake into a complete, structured summary with missing facts and source documents, without giving legal advice.
- **Extensions:** secure forms, email, calendar, document extraction, CRM/case intake, e-signature, and approved messaging.
- **Agents and authored context:** an intake coordinator and document organizer; their Souls express authored constraints against legal conclusions, promises, and representation, while runtime policy and approvals enforce scope. Heartbeat reentry guidance is limited to unsigned forms and stalled intake after a runtime wake.
- **Skills and task behaviors:** structured intake, conflict-check input preparation, document classification, scheduling, and attorney handoff.
- **Loop:** verify identity and consent, collect required fields, classify documents, identify missing information, prepare a factual summary, and route to a human decision.
- **Jobs and triggers:** new-intake trigger, missing-document reminder, appointment preparation job, and retention-expiry job.
- **Channel and bridge:** secure client-intake bridge plus a private firm AGH Network channel.
- **Approvals:** mandatory before any legal characterization, engagement communication, external filing, document signature, or third-party disclosure.
- **Safety defaults:** no legal advice, privilege warning, tenant isolation, sensitive-data redaction, configurable retention, and no automated conflict decision.
- **Current AGH fit:** **Platform evolution** — secure intake, regulated retention, identity, conflict-system integration, and policy controls are required.
- **Evidence:** [OC-5], [PC-2].

### 11. nonprofit-grant-desk

- **Persona:** a nonprofit program lead, fundraiser, or volunteer grant coordinator.
- **Outcome and first proof:** produce a sourced eligibility matrix for a small set of grants, including deadlines, missing evidence, and a recommended next step.
- **Extensions:** web research, email, calendar, cloud documents, spreadsheets, donor/CRM, forms, and task management.
- **Agents and authored context:** a grant researcher, program-evidence curator, and application editor; their Souls express source-grounding constraints against invented impact claims or financial figures, while runtime approvals control submission. Heartbeat reentry guidance covers approaching deadlines and missing approvals after a runtime wake.
- **Skills and task behaviors:** eligibility extraction, source citation, narrative drafting, budget-table preparation, and requirement traceability.
- **Loop:** discover or ingest an opportunity, extract requirements, verify eligibility against organization data, build a compliance matrix, draft only supported sections, request review, and stop before submission.
- **Jobs and triggers:** weekly opportunity scan, new-opportunity trigger, deadline reminders, and post-award reporting reminders.
- **Channel and bridge:** private grants AGH Network channel with an optional email or Slack digest bridge.
- **Approvals:** required before submitting, contacting a funder, committing match funds, changing a budget, or asserting outcomes.
- **Safety defaults:** citation for every eligibility claim, no fabricated metrics, funder-domain allowlist, duplicate-application detection, and strict organization/workspace isolation.
- **Current AGH fit:** **Platform evolution** — portable research, CRM/document dependencies, guided setup, deadline import, and source-verification policy are needed.
- **Evidence:** [H-4], [OC-2], [PI-3].

## Marketing, brand, and creators

### 12. content-marketing-engine

- **Persona:** a small marketing team, founder-led brand, or agency content lead.
- **Outcome and first proof:** deliver one source-backed content brief and draft, matched to an approved audience and publication destination, without publishing.
- **Extensions:** web search, browser, analytics, documents, CMS, social bridge adapters, image generation, and project tracking.
- **Agents and authored context:** strategist, researcher, writer, editor, and brand reviewer; each Soul expresses a stage-specific persona and source-grounding constraints, while runtime tasks and approvals own stages and authority. Heartbeat reentry guidance covers stalled approvals and performance-review windows after a runtime wake.
- **Skills and task behaviors:** audience research, claim verification, editorial structure, brand voice, SEO checks, accessibility, and asset prompting.
- **Loop:** select a validated topic, research, create a brief, draft, review claims and brand fit, obtain approval, optionally publish, and measure after a defined interval.
- **Jobs and triggers:** weekly planning job, source-change trigger, editorial calendar jobs, and post-publication measurement job.
- **Channel and bridge:** editorial AGH Network channel with optional Slack/Teams approval bridge and CMS publishing bridge.
- **Approvals:** required before publishing, changing a live page, using customer data, generating paid media, or making performance claims.
- **Safety defaults:** source citations, no fabricated quotes or statistics, license-aware asset handling, destination-specific rate limits, and a hard publish gate.
- **Current AGH fit:** **Platform evolution** — it needs dependency resolution, provider contracts, account setup, profile-scoped skills/Loops, sample-data mode, and publication health checks.
- **Evidence:** [H-4], [OC-2], [OC-4], [PI-2].

### 13. creator-repurpose-studio

- **Persona:** a podcaster, video creator, educator, or community host.
- **Outcome and first proof:** transform one approved source recording into a transcript summary, chapter list, short-post drafts, and clip candidates.
- **Extensions:** transcription, document/media extraction, video tools, image generation, cloud storage, social bridge adapters, and publishing tools.
- **Agents and authored context:** transcript editor, clip researcher, social writer, and rights reviewer; their Souls express the creator's position and source-grounding constraints against invented quotes, while runtime grants restrict source access. Heartbeat reentry guidance applies only to approved source folders and release windows after a runtime wake.
- **Skills and task behaviors:** transcription cleanup, chaptering, quote verification, clip selection, platform adaptation, captions, and accessibility.
- **Loop:** ingest approved media, transcribe, align timestamps, identify self-contained segments, verify exact attribution, generate derivatives, request approval, and export.
- **Jobs and triggers:** new-media trigger, scheduled repurpose job, publication reminders, and performance-review job.
- **Channel and bridge:** creator AGH Network workspace channel with optional Discord, Telegram, or Slack review bridges and separate publishing bridges.
- **Approvals:** required before public posting, voice or likeness synthesis, clipping a guest, monetization changes, or rights-sensitive reuse.
- **Safety defaults:** source-only quotation, rights metadata, no voice cloning by default, private drafts, export watermarking when requested, and deletion controls for raw media.
- **Current AGH fit:** **Platform evolution** — media provider contracts, provider setup, dependency installation, asset ownership, and profile-scoped processing resources are required.
- **Evidence:** [OC-1], [OC-2], [PI-2].

### 14. newsletter-studio

- **Persona:** an analyst, founder, association, or niche publisher producing a recurring newsletter.
- **Outcome and first proof:** produce one cited issue draft from an approved source list with duplicate stories removed and links checked.
- **Extensions:** RSS/web research, browser, documents, email publishing, analytics, image generation, and optional CRM segmentation.
- **Agents and authored context:** source curator, analyst, editor, and deliverability reviewer; their Souls express editorial stance and source-grounding constraints against invented reporting, while runtime approvals control sending. Heartbeat reentry guidance covers source-freshness and send-readiness evidence after a runtime wake.
- **Skills and task behaviors:** source ranking, deduplication, summarization, link checking, editorial sequencing, subject-line drafting, and accessibility.
- **Loop:** collect from approved sources, cluster duplicates, verify claims, draft sections, edit, check links and consent segments, obtain approval, send, and review outcomes.
- **Jobs and triggers:** source collection job, issue deadline jobs, breaking-topic trigger with manual confirmation, and post-send analytics job.
- **Channel and bridge:** editorial AGH Network channel plus an email-platform bridge and optional team-approval bridge.
- **Approvals:** required before adding a source outside the allowlist, sending, changing subscriber segments, or using sponsored claims.
- **Safety defaults:** unsubscribe and consent integrity, no scraped personal addresses, bounded quotation, source links, send-test first, and duplicate-send protection.
- **Current AGH fit:** **Current with preinstalled dependencies** — scheduling, agents, and triggers fit today; research, email, analytics, skills, and the Loop must be installed, configured, authorized, and healthy extension-wide. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; bridge presets project disabled instances that require separate setup and enablement.
- **Evidence:** [H-4], [OC-2], [OC-3].

### 15. seo-growth-loop

- **Persona:** an SEO lead, content marketer, or small-site owner.
- **Outcome and first proof:** identify one evidence-backed search opportunity and produce a technical/content brief tied to current site data.
- **Extensions:** Search Console, analytics, crawler/browser, CMS, documents, rank tracking, and issue tracking.
- **Agents and authored context:** search analyst, technical auditor, content strategist, and reviewer; their Souls express authored constraints against guaranteed-ranking claims and hidden link schemes, while runtime policy restricts properties and publishing. Heartbeat reentry guidance applies only to approved properties after a runtime wake.
- **Skills and task behaviors:** query clustering, page intent analysis, technical inspection, internal-link analysis, structured brief writing, and change verification.
- **Loop:** gather property data, select an opportunity, inspect current pages, propose a bounded change, verify before/after evidence, request approval, publish or open an issue, and measure later.
- **Jobs and triggers:** weekly opportunity job, crawl-error trigger, content-decay job, and post-change measurement job.
- **Channel and bridge:** marketing AGH Network channel with CMS and issue-tracker bridges.
- **Approvals:** required before editing a live page, changing metadata at scale, publishing, or altering redirects.
- **Safety defaults:** property allowlist, no ranking guarantees, no destructive bulk edits, change budget, source snapshots, and reversible patches where supported.
- **Current AGH fit:** **Platform evolution** — analytics/crawler provider contracts, guided property selection, dependency setup, profile-scoped audit resources, and measurement state are needed.
- **Evidence:** [OC-2], [OC-4], [PI-1].

### 16. community-manager

- **Persona:** a Discord, Slack, Telegram, forum, or member-community operator.
- **Outcome and first proof:** deliver a weekly community-health brief with unanswered questions, repeated topics, moderation escalations, and representative source links.
- **Extensions:** community bridge adapters, moderation tools, knowledge base, event calendar, analytics, and issue/CRM export.
- **Agents and authored context:** moderator assistant, FAQ responder, community analyst, and escalation reviewer; their Souls express community posture and constraints against impersonating staff decisions, while runtime policy and human roles retain moderation authority. Heartbeat reentry guidance covers unanswered high-signal threads and delivery failures after a runtime wake.
- **Skills and task behaviors:** conversation clustering, policy-grounded moderation, FAQ retrieval, event reminders, sentiment caution, and source-preserving summaries.
- **Loop:** ingest bounded public/community events, filter private content, identify needs, answer only approved FAQs, escalate policy decisions, compile the brief, and verify citations.
- **Jobs and triggers:** unanswered-question trigger, explicit mention trigger, event reminders, moderation-signal trigger, and weekly health job.
- **Channel and bridge:** the external community bridge plus a private staff-review AGH Network channel.
- **Approvals:** required for bans, deletions, public policy statements, direct outreach, or publishing member-derived insights externally.
- **Safety defaults:** no private-message mining by default, role-aware visibility, anti-spam limits, human moderation authority, and no emotion or demographic profiling.
- **Current AGH fit:** **Current with preinstalled dependencies** — the AGH Network channel, agents, jobs, triggers, and disabled bridge projection fit; moderation, analytics, and knowledge resources stay extension-scoped and must be installed, configured, authorized, and healthy. Jobs and triggers target a packaged agent that starts any required Loop through `agh__loop_run`.
- **Evidence:** [OC-3], [PC-4], [H-7].

## Sales, meetings, and customer success

### 17. sales-pipeline-assistant

- **Persona:** an account executive, founder-seller, or small sales team.
- **Outcome and first proof:** produce a sourced daily pipeline brief with stale opportunities, upcoming meetings, missing next steps, and draft follow-ups.
- **Extensions:** CRM, email, calendar, meeting transcripts, contacts/enrichment, documents, and messaging.
- **Agents and authored context:** account researcher, meeting-prep agent, follow-up drafter, and pipeline reviewer; their Souls express source-grounding and no-unsanctioned-outreach constraints, while runtime approvals control external representation. Heartbeat reentry guidance covers stale stages and commitments after a runtime wake.
- **Skills and task behaviors:** account research, CRM hygiene, meeting preparation, commitment extraction, follow-up drafting, and source citation.
- **Loop:** reconcile CRM and communication state, identify a bounded next action, prepare evidence, draft, request approval, apply the approved update, and verify the system of record.
- **Jobs and triggers:** daily pipeline job, meeting-soon trigger, post-meeting trigger, and stale-opportunity job.
- **Channel and bridge:** private sales AGH Network channel plus CRM, email, and optional Slack/Teams bridges.
- **Approvals:** required before sending, changing stage or forecast, creating a quote, sharing pricing, or enriching from a paid source.
- **Safety defaults:** no bulk cold outreach, contact consent and suppression respected, CRM remains authority, per-account isolation, and evidence attached to stage suggestions.
- **Current AGH fit:** **Platform evolution** — CRM and meeting provider contracts, identity resolution, account setup, dependency installation, and approval UX are required.
- **Evidence:** [H-6], [OC-2], [PC-2].

### 18. meeting-to-action

- **Persona:** a manager, salesperson, recruiter, project lead, or executive assistant.
- **Outcome and first proof:** convert one approved transcript into a factual meeting summary, decision log, proposed owners, and draft tasks.
- **Extensions:** meeting transcript provider, calendar, task system, documents/knowledge base, CRM, and email or messaging.
- **Agents and authored context:** facilitator, decision recorder, action editor, and follow-up drafter; their Souls express the distinction between spoken facts and inferred ownership, while approvals control task mutation and external follow-up. Heartbeat reentry guidance covers unresolved approved actions after a runtime wake.
- **Skills and task behaviors:** transcript alignment, decision extraction, action normalization, owner resolution, task drafting, and follow-up writing.
- **Loop:** validate meeting and attendee scope, extract facts, separate decisions from proposals, draft actions, request owner confirmation, write approved records, and verify links.
- **Jobs and triggers:** transcript-ready trigger, pre-meeting context job, post-meeting follow-up deadline, and overdue-action check.
- **Channel and bridge:** meeting-specific AGH Network channel with task-system, CRM, Slack/Teams, and email bridges.
- **Approvals:** required before assigning another person, creating or changing external tasks, updating CRM, or sending follow-up.
- **Safety defaults:** transcript consent, no recording activation, source timestamps, uncertain-owner markers, participant visibility controls, and no sensitive-summary broadcast.
- **Current AGH fit:** **Current with preinstalled dependencies** — profile resources fit after meeting, task, CRM, and static processing resources are installed, configured, authorized, and healthy extension-wide. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; bridge presets remain disabled until separately configured and enabled.
- **Evidence:** [H-6], [OC-3], [PC-2].

### 19. customer-onboarding-pod

- **Persona:** a customer-success manager onboarding a new business customer.
- **Outcome and first proof:** generate a customer-specific onboarding plan from the signed scope, known stakeholders, product requirements, and agreed milestones.
- **Extensions:** CRM, contract/documents, project tracking, email, calendar, product telemetry, knowledge base, and messaging.
- **Agents and authored context:** onboarding lead, technical coordinator, training curator, and risk reviewer; their Souls express signed-scope constraints, while runtime approvals control commitments. Heartbeat reentry guidance covers milestones, unanswered questions, and adoption evidence after a runtime wake.
- **Skills and task behaviors:** scope extraction, stakeholder mapping, project setup, training selection, risk detection, and handoff summaries.
- **Loop:** validate scope, build plan, obtain internal and customer approval, create milestones, monitor evidence, escalate blockers, verify completion, and hand off to success.
- **Jobs and triggers:** contract-signed trigger, kickoff preparation, milestone reminders, telemetry review, and handoff job.
- **Channel and bridge:** isolated customer workspace with an internal AGH Network channel and customer-approved messaging bridges.
- **Approvals:** required before customer communication, plan commitment, account configuration, deadline change, or disclosure across customer workspaces.
- **Safety defaults:** contract is authority, tenant isolation, no unsupported delivery promise, scoped product access, explicit handoff, and capped reminder frequency.
- **Current AGH fit:** **Platform evolution** — cross-system dependency setup, customer workspace provisioning, identity mapping, and reusable lifecycle templates are needed.
- **Evidence:** [PC-1], [PC-2], [OC-2].

### 20. churn-rescue-desk

- **Persona:** a customer-success or account-management team.
- **Outcome and first proof:** produce a cited at-risk-account brief that separates observed signals from hypotheses and proposes a bounded human-owned intervention.
- **Extensions:** product analytics, CRM, support desk, billing, email/calendar, meeting notes, and task tracking.
- **Agents and authored context:** account analyst, support-pattern researcher, and intervention drafter; their Souls express constraints against manipulative outreach and unsupported health scores, while runtime approvals control external contact. Heartbeat reentry guidance covers approved risk-threshold evidence after a runtime wake.
- **Skills and task behaviors:** account timeline reconstruction, usage comparison, ticket clustering, billing-status interpretation, and intervention drafting.
- **Loop:** gather permitted signals, verify identity and time windows, explain evidence, propose options, obtain account-owner approval, create an action, and measure the result.
- **Jobs and triggers:** periodic health review, usage-drop trigger, repeated-ticket trigger, failed-payment trigger, and renewal-window job.
- **Channel and bridge:** private success AGH Network channel with CRM and approved outreach bridges.
- **Approvals:** mandatory before contacting a customer, applying a discount, changing terms, or labeling an account at risk in the CRM.
- **Safety defaults:** no demographic inference, no dark patterns, transparent evidence, configurable thresholds, tenant isolation, and human ownership of relationship decisions.
- **Current AGH fit:** **Platform evolution** — provider contracts, identity reconciliation, risk-policy setup, dependency installation, and measurement state are required.
- **Evidence:** [PC-2], [OC-5].

### 21. customer-support-desk

- **Persona:** a support lead or small service team handling email, chat, or community tickets.
- **Outcome and first proof:** triage a bounded test queue, link each issue to approved knowledge, and prepare reply drafts and escalation reasons without sending.
- **Extensions:** support platform, email/chat bridge adapters, knowledge base, product status, issue tracker, CRM, and optional telemetry.
- **Agents and authored context:** triage agent, knowledge researcher, reply drafter, and escalation reviewer; their Souls express support posture and representation constraints, while runtime policy and approvals enforce authority. Heartbeat reentry guidance covers SLA risk and integration failures after a runtime wake.
- **Skills and task behaviors:** intent classification, duplicate detection, evidence retrieval, troubleshooting guidance, sentiment caution, and issue handoff.
- **Loop:** authenticate ticket scope, reconstruct context, search approved evidence, draft or escalate, obtain approval where needed, send, verify delivery, and record outcome.
- **Jobs and triggers:** new-ticket trigger, SLA threshold trigger, incident-cluster trigger, and daily unresolved-ticket digest.
- **Channel and bridge:** support-system bridge plus a private team AGH Network channel; customer-facing delivery remains in the system of record.
- **Approvals:** required for refunds, account changes, security disclosures, policy exceptions, or responses unsupported by approved knowledge.
- **Safety defaults:** start in draft-only mode, preserve ticket authority, redact secrets, isolate customer data, cap automated replies, and stop during suspected incidents.
- **Current AGH fit:** **Platform evolution** — connector installation, account/tenant setup, knowledge indexing, approval UX, and reusable support-system contracts are required.
- **Evidence:** [OC-3], [PC-2], [PC-4].

## Commerce, finance, and administration

### 22. ecommerce-operator

- **Persona:** a small ecommerce owner or operations manager.
- **Outcome and first proof:** produce a daily operating brief with sales, fulfillment exceptions, low-stock risks, refunds, and support hotspots, each linked to source records.
- **Extensions:** Shopify or WooCommerce, payments, inventory/fulfillment, support, reviews, analytics, email, and messaging.
- **Agents and authored context:** commerce analyst, fulfillment coordinator, customer-issue researcher, and merchandising advisor; their Souls express constraints against unapproved price or refund changes, while runtime approvals enforce mutation authority. Heartbeat reentry guidance covers exceptions and connector diagnostics after a runtime wake.
- **Skills and task behaviors:** order reconciliation, inventory risk, refund triage, review clustering, KPI explanation, and action drafting.
- **Loop:** collect bounded operational data, reconcile IDs, identify exceptions, verify against source systems, propose actions, obtain approval, apply selected mutations, and confirm.
- **Jobs and triggers:** daily brief, low-stock trigger, high-value refund trigger, fulfillment-delay trigger, and weekly trend review.
- **Channel and bridge:** private operations AGH Network channel with commerce, support, and owner-messaging bridges.
- **Approvals:** required for refunds, discounts, price changes, supplier orders, customer outreach, or product publication.
- **Safety defaults:** store remains authority, money and inventory mutations default off, per-order idempotency, fraud decisions stay human, and no customer-data reuse for unrelated marketing.
- **Current AGH fit:** **Platform evolution** — commerce/provider contracts, secret setup, dependency resolution, transaction policy, and health checks are needed.
- **Evidence:** [H-5], [OC-5], [PC-3].

### 23. finance-close-lite

- **Persona:** a freelancer, nonprofit treasurer, or small-business finance operator working with a qualified accountant.
- **Outcome and first proof:** prepare a read-only month-end exception list covering missing receipts, unreconciled items, duplicate charges, and source-document gaps.
- **Extensions:** accounting system, payments, bank feed, invoices, document extraction, cloud storage, spreadsheets, and task management.
- **Agents and authored context:** reconciliation assistant, document collector, and close reviewer; their Souls express constraints against accounting representations, filing, or autonomous journal entries, while runtime policy and approvals enforce scope. Heartbeat reentry guidance applies only to approved close periods after a runtime wake.
- **Skills and task behaviors:** transaction matching, document extraction, duplicate detection, variance explanation, and checklist management.
- **Loop:** import a bounded period, normalize records, match evidence, flag exceptions, request missing documents, obtain human disposition, and export an audit-linked close packet.
- **Jobs and triggers:** month-end preparation, new-document trigger, overdue-receipt reminders, and close-deadline job.
- **Channel and bridge:** private finance AGH Network channel with accounting and secure-document bridges.
- **Approvals:** mandatory before ledger mutation, payment, filing, classification override, or external communication.
- **Safety defaults:** read-only by default, accountant remains authority, source-linked output, no tax or investment advice, immutable period snapshots, and strict workspace access.
- **Current AGH fit:** **Platform evolution** — financial provider contracts, secure identity, policy, reconciliation state, guided setup, and compliance validation are required.
- **Evidence:** [H-4], [OC-2], [OC-5].

### 24. invoice-chaser

- **Persona:** a freelancer, agency operations lead, or accounts-receivable clerk.
- **Outcome and first proof:** create a verified overdue-invoice queue and a staged set of draft reminders that reflect contract terms and previous communication.
- **Extensions:** invoicing/accounting, payments, CRM, email, calendar, documents, and messaging.
- **Agents and authored context:** receivables coordinator and tone reviewer; their Souls express constraints against threats, invented fees, or payment commitments, while runtime approvals control external sends. Heartbeat reentry guidance covers due dates and recorded promises to pay after a runtime wake.
- **Skills and task behaviors:** invoice-state reconciliation, contract-term extraction, communication timeline, reminder drafting, and escalation.
- **Loop:** verify invoice and contact identity, inspect terms and conversation history, choose the allowed stage, draft, request approval, send, confirm delivery, and schedule the next check.
- **Jobs and triggers:** due-date jobs, failed-payment trigger, promise-to-pay reminder, and weekly receivables summary.
- **Channel and bridge:** private finance AGH Network channel plus an approved email/SMS bridge.
- **Approvals:** required before every initial external send, escalation, fee mention, account hold, or collections handoff.
- **Safety defaults:** no duplicate reminders, quiet hours and jurisdiction-aware cadence, dispute detection, stop on payment or reply, and full communication evidence.
- **Current AGH fit:** **Current with preinstalled dependencies** — jobs, trigger templates, and agents fit; financial/email resources and the Loop must be installed, configured, authorized, and healthy and remain extension-scoped. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; bridge presets project disabled instances that require separate secret binding and enablement.
- **Evidence:** [H-4], [OC-2], [OC-5].

### 25. cash-runway-watch

- **Persona:** a founder or finance lead who needs operating visibility, not automated financial advice.
- **Outcome and first proof:** produce a reproducible cash runway snapshot using explicitly selected accounts, assumptions, and committed expenses.
- **Extensions:** accounting, bank feeds, payments, payroll summary, spreadsheets, contracts, and notification bridge adapters.
- **Agents and authored context:** finance analyst and assumption reviewer; their Souls express assumption-disclosure and no-advice constraints, while runtime policy controls data access and publication. Heartbeat reentry guidance covers data-freshness evidence and threshold events after a runtime wake.
- **Skills and task behaviors:** cash categorization, commitment extraction, scenario modeling, variance analysis, and provenance.
- **Loop:** gather approved balances and commitments, validate freshness, compute scenarios, verify arithmetic, label uncertainty, publish the snapshot, and stop before taking financial action.
- **Jobs and triggers:** weekly snapshot, material-balance change trigger, major-contract trigger, and stale-data alert.
- **Channel and bridge:** private leadership AGH Network channel with spreadsheet and accounting bridges.
- **Approvals:** required before changing assumptions used by others, sharing externally, moving funds, or initiating cost reductions.
- **Safety defaults:** no prescriptive financial advice, explicit currency/time horizon, source links, stale-data block, scenario rather than certainty language, and no money movement.
- **Current AGH fit:** **Platform evolution** — provider setup, typed financial data contracts, calculation verification, policy, and secure dependency management are needed.
- **Evidence:** [OC-5], [PC-1].

## People operations and education

### 26. recruiting-coordinator

- **Persona:** a recruiter, hiring manager, or small-company people lead.
- **Outcome and first proof:** produce a candidate-process brief with missing materials, upcoming interviews, and draft scheduling messages for a test requisition.
- **Extensions:** applicant tracking system, email, calendar, documents, forms, video meeting, and messaging.
- **Agents and authored context:** scheduling coordinator, packet preparer, and process auditor; their Souls express constraints against protected-trait ranking or hiring decisions, while runtime policy and human approvals retain decision authority. Heartbeat reentry guidance covers scheduling conflicts and stalled stages after a runtime wake.
- **Skills and task behaviors:** requisition-grounded intake, schedule coordination, document completeness, interview packet assembly, and respectful communication drafting.
- **Loop:** verify requisition and candidate scope, collect required process data, propose schedule or next step, obtain approval, update the ATS, and confirm delivery.
- **Jobs and triggers:** application-received trigger, interview reminder, feedback reminder, stage-stale job, and candidate-withdrawal trigger.
- **Channel and bridge:** private hiring AGH Network channel plus ATS, calendar, email, and approved interview-team messaging bridges.
- **Approvals:** required before candidate rejection, advancement, compensation discussion, external contact, or sharing interview feedback.
- **Safety defaults:** no protected-trait inference, no automated hiring verdict, role-based visibility, candidate retention policy, consistent approved templates, and audit links.
- **Current AGH fit:** **Platform evolution** — ATS/provider contracts, identity mapping, policy enforcement, dependency setup, and regulated retention controls are needed.
- **Evidence:** [PC-2], [OC-5].

### 27. employee-onboarding-coordinator

- **Persona:** a people-operations lead or manager onboarding a new employee or contractor.
- **Outcome and first proof:** generate a role-specific onboarding plan from approved policy, manager goals, required access, and scheduled introductions.
- **Extensions:** HRIS, identity/access requests, email, calendar, documents, learning system, ticketing, and messaging.
- **Agents and authored context:** onboarding coordinator, access checklist agent, and learning curator; their Souls express constraints against granting access or representing policy beyond approved sources, while runtime grants and approvals enforce authority. Heartbeat reentry guidance covers incomplete tasks and first-week milestones after a runtime wake.
- **Skills and task behaviors:** checklist generation, policy retrieval, meeting scheduling, training selection, and manager handoff.
- **Loop:** validate role and start date, build plan, obtain manager/HR approval, create allowed tasks, monitor evidence, escalate blockers, and close with an acknowledged handoff.
- **Jobs and triggers:** hire-approved trigger, pre-start jobs, first-day and first-week jobs, access-delay trigger, and 30-day check.
- **Channel and bridge:** isolated employee-onboarding AGH Network channel with HR, ticketing, calendar, and team-messaging bridges.
- **Approvals:** required before granting access, sharing personnel data, changing employment terms, or sending policy statements.
- **Safety defaults:** least privilege, no access grant by default, role-based visibility, no sensitive-data broadcast, deletion/retention policy, and manager ownership.
- **Current AGH fit:** **Platform evolution** — HRIS and identity provider contracts, setup, access-policy integration, and workspace provisioning are required.
- **Evidence:** [PC-1], [PC-2], [OC-5].

### 28. study-coach

- **Persona:** a student, adult learner, or parent supporting a learner.
- **Outcome and first proof:** produce a realistic seven-day study plan from syllabus deadlines, available time, and a short diagnostic, with source-linked topics.
- **Extensions:** learning-management system, calendar, notes, documents, flashcards, reminders, and optional messaging.
- **Agents and authored context:** study planner, tutor, and reflection coach; their Souls express constraints against completing graded work as the learner or making unsupported educational assessments, while runtime policy and approvals enforce scope. Heartbeat reentry guidance covers agreed sessions and deadline risk after a runtime wake.
- **Skills and task behaviors:** syllabus extraction, spaced repetition, question generation, formative feedback, plan adjustment, and citation.
- **Loop:** collect goals and constraints, assess knowledge with consent, plan, conduct one study unit, verify understanding, record reflection, and adjust without hiding uncertainty.
- **Jobs and triggers:** daily study reminder, deadline trigger, spaced-repetition jobs, and weekly review.
- **Channel and bridge:** private learner AGH Network channel with calendar, LMS, and flashcard bridges.
- **Approvals:** required before submitting work, contacting an instructor, sharing learner data, or changing external deadlines.
- **Safety defaults:** no cheating, age-appropriate privacy, workload caps, accessible format, no diagnosis of learning conditions, and explicit source attribution.
- **Current AGH fit:** **Platform evolution** — LMS/provider contracts, learner setup, dependency resolution, progress state, and age/guardian policy are needed.
- **Evidence:** [H-2], [H-4], [OC-2].

### 29. research-desk

- **Persona:** an analyst, researcher, journalist, consultant, or product strategist.
- **Outcome and first proof:** answer one bounded research question with a source map, claim-evidence matrix, contradictions, and clearly labeled inferences.
- **Extensions:** web search, browser/scraping, documents, academic search, local knowledge base, citation store, and optional team messaging.
- **Agents and authored context:** research lead, source scout, skeptic, and synthesis editor; their Souls express the distinction between evidence, inference, and recommendation, while runtime grants restrict source access. Heartbeat reentry guidance applies only to active research deadlines and source-availability diagnostics after a runtime wake.
- **Skills and task behaviors:** query decomposition, source evaluation, citation capture, contradiction analysis, bounded quotation, and synthesis.
- **Loop:** define the question, plan evidence needs, collect sources, assess quality, test contradictory explanations, synthesize, verify citations, and stop at budget or sufficiency.
- **Jobs and triggers:** scheduled topic watch, source-change trigger, deadline reminders, and optional periodic evidence refresh.
- **Channel and bridge:** research AGH Network channel with document and knowledge bridges plus an optional reviewer AGH Network channel.
- **Approvals:** required before contacting subjects, purchasing data, publishing, or ingesting restricted/confidential sources.
- **Safety defaults:** primary sources preferred, copyright limits, provenance for each claim, no citation fabrication, source-domain policy, and budgeted exploration.
- **Current AGH fit:** **Current with preinstalled dependencies** — the agent roster, AGH Network channel, jobs, and triggers fit; search, research skills, citation tools, and the Loop must be installed, configured, authorized, and healthy and remain extension-scoped. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`.
- **Evidence:** [H-2], [PI-1], [PI-3], [OC-2].

## Engineering and product

### 30. fix-linear-issue

- **Persona:** a software engineer or product engineering team that uses Linear and GitHub.
- **Outcome and first proof:** take one explicitly selected Linear issue, reproduce or validate it in the bound workspace, and publish a plan plus evidence back to the issue before any code mutation.
- **Extensions:** Linear, GitHub, coding-agent bridge, workspace/filesystem, terminal and sandbox tools, CI status, and optional team messaging.
- **Agents and authored context:** issue investigator, implementer, test reviewer, and PR reviewer; their Souls express issue-bound, repository-grounded working constraints, while task claims, grants, and approvals enforce authority. Heartbeat reentry guidance covers claimed-work context, CI evidence, review requests, and retry limits after a runtime wake.
- **Skills and task behaviors:** issue reading/updating, repository exploration, implementation discipline, test placement, review, PR writing, and handoff; an optional peer may advertise a code-review AGH Network Capability.
- **Loop:** claim the issue, inspect dependencies, reproduce, plan, obtain any required approval, implement, run scoped verification, open or update a PR, request review, react to verdicts, update Linear, and stop only at a named terminal outcome.
- **Jobs and triggers:** explicit issue-assignment or label trigger, PR-review trigger, CI-completion trigger, continuation run after rejection, and stale-claim task-event trigger.
- **Channel and bridge:** issue-specific AGH Network channel with Linear and GitHub bridges; optional Slack/Teams notification bridge.
- **Approvals:** required before destructive workspace operations, external PR creation where policy demands it, merging, release, changing issue scope, or exposing secrets.
- **Safety defaults:** one issue and one workspace per activation, no auto-merge, no weakening tests to obtain green status, bounded retries and spend, sandbox policy, source-linked status updates, and idempotent issue comments.
- **Current AGH fit:** **Current with preinstalled dependencies** — current profiles can project the agents, AGH Network channels, jobs, triggers, and disabled bridge instances; Linear/GitHub extensions and static skills, tools, MCP servers, hooks, and the Loop must be installed, configured, authorized, and healthy and remain extension-scoped. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`.
- **Evidence:** [AGH-1], [AGH-2], [H-5], [OC-4], [PI-3], [PC-2].

### 31. pr-review-factory

- **Persona:** a software team that wants consistent, reviewable pull-request analysis.
- **Outcome and first proof:** inspect one selected PR and return deduplicated findings grouped by correctness, security, tests, performance, architecture, and documentation, each tied to a line or runtime fact.
- **Extensions:** GitHub or GitLab, coding-agent bridge, workspace checkout, CI status, static analysis, and optional issue tracking.
- **Agents and authored context:** review coordinator plus focused reviewers; their Souls express evidence-first and low-noise review constraints, while runtime grants and approvals control publication. Heartbeat reentry guidance covers review requests, updated commits, and budget state after a runtime wake.
- **Skills and task behaviors:** diff triage, code review, security review, test-quality review, documentation impact, and finding deduplication.
- **Loop:** resolve the immutable revision, assign review scopes, collect findings, verify and deduplicate, rank by user impact, publish a review draft, obtain approval if required, and re-review only changed evidence.
- **Jobs and triggers:** review-request trigger, new-commit trigger, CI-completion trigger, and stale-review reminder.
- **Channel and bridge:** PR-specific AGH Network channel with a source-control review bridge and optional team-notification bridge.
- **Approvals:** required before submitting blocking reviews, requesting changes as a represented human, modifying code, or merging.
- **Safety defaults:** pin commit SHA, no duplicate comments, severity evidence, bounded reviewer fan-out, no auto-merge, and ignore unrelated workspace changes.
- **Current AGH fit:** **Current with preinstalled dependencies** — agents, triggers, and jobs fit; source-control tools and all review skills and Loops must be installed, configured, authorized, and healthy extension-wide. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; bridge presets remain disabled until separately configured and enabled.
- **Evidence:** [PI-3], [OC-2], [PC-2].

### 32. incident-response-room

- **Persona:** an on-call engineer, incident commander, or small operations team.
- **Outcome and first proof:** create a time-bounded incident brief from one alert, showing current evidence, affected services, unknowns, and the next safest diagnostic action.
- **Extensions:** alerting, logs/traces, deployment system, status page, issue tracker, runbooks, source control, and Slack/Teams/PagerDuty bridge adapters.
- **Agents and authored context:** incident commander, diagnostics investigator, communications drafter, and timeline recorder; their Souls express command-role and evidence-labeling posture, while task claims, grants, and approvals enforce authority. Heartbeat reentry guidance covers alert evidence, task-lease context, and communication cadence after a runtime wake.
- **Skills and task behaviors:** alert triage, log analysis, deployment comparison, runbook retrieval, stakeholder update drafting, and postmortem timeline.
- **Loop:** validate the incident, establish scope, gather evidence, propose a reversible diagnostic or mitigation, obtain approval, execute, verify impact, communicate, and terminate or hand off explicitly.
- **Jobs and triggers:** alert trigger, status-update cadence job, mitigation verification trigger, and post-incident review job.
- **Channel and bridge:** dedicated incident AGH Network channel with observability, source-control, deployment, and communication bridges.
- **Approvals:** mandatory before production mutation, rollback, traffic shift, status-page publication, customer communication, or secret access.
- **Safety defaults:** read-only investigation first, one incident correlation key, mitigation budget, hard circuit breaker, no credential copying, explicit command hierarchy, and preserved audit timeline.
- **Current AGH fit:** **Platform evolution** — safe provider contracts, incident-scoped setup, approval delivery, dependency resolution, and transactional action semantics are required.
- **Evidence:** [OC-2], [OC-5], [PC-2].

### 33. release-manager

- **Persona:** a maintainer or product engineering team preparing a release.
- **Outcome and first proof:** generate a release-readiness report for one immutable revision with test, documentation, migration, compatibility, and artifact gaps.
- **Extensions:** source control, CI, package registry, changelog/docs, issue tracker, artifact signing, deployment system, and announcement bridge adapters.
- **Agents and authored context:** release coordinator, verification agent, documentation reviewer, and communications drafter; their Souls express the distinction between preparation and release authority, while runtime approvals enforce publishing and deployment rights. Heartbeat reentry guidance covers gate and artifact-publication evidence after a runtime wake.
- **Skills and task behaviors:** change classification, release notes, dependency impact, artifact verification, docs impact, and rollback planning.
- **Loop:** select revision, compute release scope, run required gates, resolve blockers, prepare artifacts and notes, obtain approval, publish in order, verify availability, and record evidence.
- **Jobs and triggers:** release-candidate trigger, gate-completion triggers, scheduled maintenance-window job, and post-release verification.
- **Channel and bridge:** release AGH Network channel with source-control, CI, registry, deployment, and announcement bridges.
- **Approvals:** required before tagging, publishing packages, deploying, changing release channels, or making public announcements.
- **Safety defaults:** immutable revision, single-release lock, signed/checksummed artifacts where supported, no bypassed gate, rollback plan, and idempotent publication checks.
- **Current AGH fit:** **Current with preinstalled dependencies** — the coordinator profile fits after source-control, CI, registry, deployment, and static release resources are installed, configured, authorized, and healthy. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; any bridge preset remains disabled until separately configured and enabled.
- **Evidence:** [OC-2], [PI-1], [PC-2].

### 34. docs-drift-guardian

- **Persona:** a maintainer responsible for keeping documentation aligned with runtime behavior.
- **Outcome and first proof:** report one evidence-backed mismatch between a shipped contract and its documentation, or produce a verified no-op result.
- **Extensions:** workspace/filesystem, source control, code search, OpenAPI/CLI reference access, docs build/link checker, and issue tracker.
- **Agents and authored context:** contract scout, docs reviewer, and patch author; their Souls express a runtime-truth-first posture and constraints against aspirational support claims, while runtime grants and approvals control edits. Heartbeat reentry guidance covers approved source-change evidence after a runtime wake.
- **Skills and task behaviors:** contract discovery, generated/reference-doc distinction, copy consistency, link checking, and documentation patching.
- **Loop:** identify changed behavior, locate owned documentation, compare claims to runtime evidence, draft the minimal correction, run docs gates, request review, and stop at done or no-op.
- **Jobs and triggers:** contract-change trigger, release-candidate trigger, weekly drift scan, and failed-docs-build trigger.
- **Channel and bridge:** repository AGH Network channel with source-control and issue-tracker bridges.
- **Approvals:** required before public publishing, broad copy repositioning, or changing generated artifacts outside their generator.
- **Safety defaults:** no prose-only source-of-truth tests, generated regions stay generator-owned, exact evidence links, narrow patches, and no unsupported present-tense claims.
- **Current AGH fit:** **Current with preinstalled dependencies** — agents, jobs, triggers, and the AGH Network channel fit; documentation skills, code tools, hooks, and the Loop must be installed, configured, authorized, and healthy and remain extension-scoped. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`.
- **Evidence:** [AGH-1], [AGH-2], [PI-1], [OC-2].

### 35. security-audit-pipeline

- **Persona:** a security engineer or maintainer performing an authorized review.
- **Outcome and first proof:** produce a scoped threat and finding inventory for one repository revision, with exploitability evidence and false-positive notes.
- **Extensions:** source control, workspace/sandbox, dependency and secret scanners, SAST, issue tracker, and optional runtime telemetry.
- **Agents and authored context:** threat modeler, code auditor, dependency reviewer, and finding verifier; their Souls express scoped-review and responsible-disclosure posture, while runtime grants and approvals enforce authority. Heartbeat reentry guidance covers scanner-completion evidence and review budget after a runtime wake.
- **Skills and task behaviors:** threat modeling, vulnerability review, dependency risk, secret handling, remediation guidance, and evidence verification.
- **Loop:** confirm authorization and scope, establish attack surfaces, run bounded analysis, verify findings, rank by impact and likelihood, obtain disclosure approval, and create remediation records.
- **Jobs and triggers:** explicit audit start, dependency-change trigger, high-risk-code trigger, and periodic authorized scan.
- **Channel and bridge:** restricted security AGH Network channel with source-control and issue-tracker bridges.
- **Approvals:** required before active exploitation, network scanning, secret access, issue publication, external disclosure, or code mutation.
- **Safety defaults:** read-only first, target allowlist, no production exploitation, secret redaction, isolated artifacts, bounded tools, and human disclosure authority.
- **Current AGH fit:** **Current with preinstalled dependencies** — current profiles can coordinate the team and schedule; security tools, hooks, skills, MCP servers, and Loops must be installed, configured, authorized, and healthy and remain extension-scoped. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`.
- **Evidence:** [H-6], [OC-5], [PI-1].

### 36. design-parity-board

- **Persona:** a product designer and frontend engineer comparing implementation with an approved visual contract.
- **Outcome and first proof:** produce a reference-versus-implementation comparison for one route or story, with screenshots and structural mismatches.
- **Extensions:** Figma or reference assets, Storybook/browser automation, screenshot capture, accessibility tooling, source control, and issue tracking.
- **Agents and authored context:** visual reviewer, interaction reviewer, accessibility reviewer, and remediation planner; their Souls express the approved design-system posture and the distinction between defects and preferences, while runtime grants and approvals control changes. Heartbeat reentry guidance applies only to active review targets after a runtime wake.
- **Skills and task behaviors:** reference extraction, deterministic capture, visual comparison, responsive review, accessibility inspection, and issue drafting.
- **Loop:** pin reference and implementation revisions, capture matching states, compare structure and behavior, verify findings, request design adjudication for ambiguity, and publish evidence.
- **Jobs and triggers:** story/route review trigger, design-reference update trigger, release-candidate job, and resolved-finding recheck.
- **Channel and bridge:** design-review AGH Network channel with Figma, Storybook/browser, source-control, and issue bridges.
- **Approvals:** required before changing design tokens, accepting intentional divergence, editing source, or closing design findings.
- **Safety defaults:** deterministic viewport and data, no implementation-only parity claim, accessibility as a separate evidence lane, no invented reference behavior, and immutable capture metadata.
- **Current AGH fit:** **Platform evolution** — design/provider contracts, visual evidence packaging, dependency setup, and a profile-scoped comparison Loop are required.
- **Evidence:** [PI-1], [OC-4].

### 37. data-quality-watch

- **Persona:** a data engineer, analyst, or operations owner responsible for trusted reporting.
- **Outcome and first proof:** explain one detected data anomaly with affected datasets, first bad interval, likely upstream changes, and uncertainty.
- **Extensions:** warehouse/query engine, transformation metadata, orchestration, data catalog, observability, issue tracker, and notifications.
- **Agents and authored context:** anomaly investigator, lineage researcher, and remediation reviewer; their Souls express constraints against unapproved data mutation and unsupported causal claims, while runtime grants and approvals enforce authority. Heartbeat reentry guidance covers approved-check and stale-data evidence after a runtime wake.
- **Skills and task behaviors:** query validation, schema/change comparison, lineage analysis, anomaly explanation, and remediation planning.
- **Loop:** validate alert, reproduce query, bound affected data, inspect lineage and recent changes, test hypotheses, propose repair, obtain approval, verify downstream recovery, and close.
- **Jobs and triggers:** freshness and quality triggers, schema-change trigger, scheduled reconciliation, and post-repair verification.
- **Channel and bridge:** restricted data-operations AGH Network channel with warehouse, orchestration, catalog, and issue bridges.
- **Approvals:** required before running costly queries beyond budget, mutating data, backfilling, changing checks, or notifying external consumers.
- **Safety defaults:** read-only queries first, query cost cap, sampled investigation, no PII export, lineage evidence, idempotent backfills, and rollback plan.
- **Current AGH fit:** **Platform evolution** — typed data-provider contracts, cost controls, dependency setup, identity, and durable repair semantics are required.
- **Evidence:** [OC-2], [PC-2].

### 38. open-source-maintainer

- **Persona:** a maintainer or small open-source team handling issues, PRs, releases, and contributor support.
- **Outcome and first proof:** deliver a weekly project-health brief with unanswered issues, review bottlenecks, flaky checks, release readiness, and contributor follow-ups.
- **Extensions:** GitHub/GitLab, CI, package registries, discussion/community bridge adapters, docs, and optional funding platform.
- **Agents and authored context:** issue triager, PR coordinator, release assistant, and contributor guide; their Souls express constraints against representing maintainer decisions without approval, while runtime policy and approvals enforce authority. Heartbeat reentry guidance covers stale contributions and failing default-branch checks after a runtime wake.
- **Skills and task behaviors:** issue deduplication, reproduction guidance, contribution policy, review routing, release notes, and community response drafting.
- **Loop:** ingest bounded project changes, classify, gather evidence, propose maintainer actions, obtain approval, apply allowed labels/comments, and verify state.
- **Jobs and triggers:** new-issue/PR triggers, CI-failure trigger, weekly health job, and release milestone job.
- **Channel and bridge:** project AGH Network channel with source-control, CI, registry, and community bridges.
- **Approvals:** required before closing, rejecting, labeling sensitive reports, merging, releasing, or making governance statements.
- **Safety defaults:** public-data boundary, no contributor profiling, rate limits, no auto-close by inactivity alone, security-report isolation, and transparent bot identity.
- **Current AGH fit:** **Current with preinstalled dependencies** — the profile fits with source-control/community extensions and extension-scoped skills and Loops installed, configured, authorized, and healthy. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; bridge presets remain disabled until separately configured and enabled.
- **Evidence:** [OC-2], [OC-4], [PI-2].

### 39. n8n-workflow-doctor

- **Persona:** an operations engineer or no-code automation owner maintaining n8n workflows.
- **Outcome and first proof:** inspect one selected workflow and produce a read-only diagnostic of missing credentials, failing nodes, retry behavior, unsafe mutations, and recent execution errors.
- **Extensions:** n8n MCP/API connector, logs, webhook test client, secrets health, documentation search, and issue tracking.
- **Agents and authored context:** workflow diagnostician, credential-state checker, and remediation reviewer; their Souls express constraints against enabling or editing production workflows without approval, while runtime grants and approvals enforce authority. Heartbeat reentry guidance applies only to selected workflows after a runtime wake.
- **Skills and task behaviors:** graph inspection, node-contract lookup, execution analysis, idempotency review, and repair planning.
- **Loop:** fetch workflow and bounded history, validate credentials without revealing values, reproduce safely where possible, isolate failure, propose repair, obtain approval, apply, and verify.
- **Jobs and triggers:** failed-execution trigger, credential-health job, version-change trigger, and post-repair observation.
- **Channel and bridge:** private automation AGH Network channel with n8n and issue-tracker bridges.
- **Approvals:** required before activation, mutation, credential rebinding, replaying side effects, or changing webhook exposure.
- **Safety defaults:** read-only tools enabled first, secret values never returned, replay against test data, side-effect inventory, idempotency check, and production circuit breaker.
- **Current AGH fit:** **Current with preinstalled dependencies** — the n8n MCP and its Loop must be installed, configured, authorized, and healthy separately, and its tools may be pruned. Jobs and triggers target a packaged agent that starts the Loop through `agh__loop_run`; profile-owned static resources and automatic dependency setup are not current.
- **Evidence:** [H-5], [OC-5].

### 40. paperclip-company-operator

- **Persona:** an operator supervising a Paperclip company from an AGH workspace.
- **Outcome and first proof:** produce a company-health brief covering goals, blocked issues, stale heartbeats, approvals, budget use, and agents needing attention.
- **Extensions:** Paperclip HTTP/API adapter, optional messaging bridge, documents, and cost/telemetry export.
- **Agents and authored context:** company observer, issue coordinator, and governance reviewer; their Souls express a Paperclip-as-authority posture and constraints against autonomous hiring or budget mutation, while runtime grants and approvals enforce authority. Heartbeat reentry guidance covers company-health evidence and adapter diagnostics after a runtime wake.
- **Skills and task behaviors:** company inspection, issue lifecycle, budget interpretation, approval routing, run-state diagnosis, and handoff.
- **Loop:** inspect one company, identify stalled or risky state, verify against Paperclip records, propose operator actions, obtain approval, apply allowed actions, and confirm state.
- **Jobs and triggers:** periodic health job, stalled-issue trigger, approval-request trigger, budget-threshold trigger, and adapter failure alert.
- **Channel and bridge:** company-specific AGH Network channel with Paperclip and optional Slack/Teams bridges.
- **Approvals:** required before agent creation or deletion, hiring, budget changes, issue reassignment, company import, or external messaging.
- **Safety defaults:** one company per workspace binding, read-only first, Paperclip remains system of record, budget limits, no task ownership duplication, and idempotent updates.
- **Current AGH fit:** **Platform evolution** — an official provider/adapter contract, company binding, setup, typed events, and authority mapping would be required.
- **Evidence:** [PC-1], [PC-2], [PC-3], [PC-4].

### 41. codebase-research-cell

- **Persona:** an engineer or architect asking a bounded question about a local repository.
- **Outcome and first proof:** return a source-linked architecture map or answer, including unknowns and competing interpretations, without modifying files.
- **Extensions:** none beyond the owning bundle extension and the selected ACP coding-agent runtime; optional source-control metadata.
- **Agents and authored context:** research coordinator and scoped explorer agents; their Souls express read-only and evidence-path constraints, while agent tool grants and workspace policy enforce access. Heartbeat reentry guidance covers child limits and research-budget context after a runtime wake.
- **Skills and task behaviors:** repository search and structured synthesis may be expressed in agent authored context rather than a separately activated skill.
- **Loop:** the coordinator decomposes the question, dispatches bounded explorers, verifies evidence, resolves contradictions, synthesizes, and stops at done, blocked, or exhausted. This can be prompt-coordinated today; a deterministic packaged Loop would remain extension-scoped.
- **Jobs and triggers:** optional explicit research trigger; no recurring job is required.
- **Channel and bridge:** one workspace-scoped AGH Network research channel; no external bridge required.
- **Approvals:** required before any write, network access outside policy, or expansion beyond the selected workspace.
- **Safety defaults:** read-only tools, bounded child count and depth, no unrelated file access, evidence links, and no unsupported certainty.
- **Current AGH fit:** **Current** — one owning extension can package the agents, authored context, declared AGH Network channel, optional agent-targeted trigger, and any static research skill or Loop needed. The configured ACP coding-agent runtime is a baseline prerequisite under the fit definition; static resources remain extension-scoped, and an agent starts any packaged Loop through `agh__loop_run`.
- **Evidence:** [AGH-1], [PI-3].

### 42. workspace-standup-coordinator

- **Persona:** a small AGH-operated team that needs a consistent internal status ritual.
- **Outcome and first proof:** collect structured status from packaged agents and publish one workspace-scoped standup with progress, blockers, handoffs, and explicit unknowns.
- **Extensions:** none beyond the owning extension; an optional external team-messaging bridge can be omitted from the current form.
- **Agents and authored context:** coordinator plus worker agents; their Souls express status vocabulary and constraints against claiming unverified completion, while task state remains authoritative. Heartbeat reentry guidance tells an eligible session how to reorient after a runtime wake; task APIs and session-health surfaces expose blockers and health.
- **Skills and task behaviors:** status and handoff behavior can live in authored agent context; no separately activated static skill is required for the basic profile.
- **Loop:** request status, wait within a bounded window, normalize replies, verify referenced work state, publish the summary, and stop. The basic implementation can be prompt/job coordinated without a profile-owned Loop.
- **Jobs and triggers:** weekday standup job, blocker trigger, and optional end-of-day handoff job.
- **Channel and bridge:** one declared AGH Network coordination channel; an optional bridge remains disabled until its extension is installed and its account, secrets, authorization, and health are configured.
- **Approvals:** no approval for internal status collection; approval required before forwarding outside the workspace or reassigning work.
- **Safety defaults:** workspace-only visibility, no raw claim tokens, no status authority transfer to the channel, bounded reminders, and clear missing-response markers.
- **Current AGH fit:** **Current** — packaged agents, Soul/Heartbeat sidecars, AGH Network channels, jobs, and agent-targeted triggers are all current bundle-profile resources. The sidecars provide persona and wake/reentry context rather than operational authority or monitoring.
- **Evidence:** [AGH-1], [AGH-3].

### 43. agency-client-pod

- **Persona:** an agency or consultancy operating repeatable work for multiple clients.
- **Outcome and first proof:** provision one isolated client coordination profile and deliver a source-backed weekly client report from approved systems.
- **Extensions:** client-specific email/chat bridges, project tracker, documents, analytics, CRM, invoicing, and optional content or development tools.
- **Agents and authored context:** client lead, delivery coordinator, analyst, and reviewer; every Soul expresses the client boundary and commitment constraints, while runtime grants and approvals enforce authority. Heartbeat reentry guidance covers milestones and data-source diagnostics after a runtime wake.
- **Skills and task behaviors:** client briefing, status synthesis, scope traceability, risk reporting, deliverable review, and invoice preparation.
- **Loop:** bind client workspace, gather approved evidence, compare to scope and milestones, draft internal report, obtain account-owner approval, deliver, and record acknowledgment.
- **Jobs and triggers:** weekly report, milestone trigger, scope-change trigger, risk threshold, and invoice-cycle job.
- **Channel and bridge:** isolated client workspace with an internal AGH Network channel and separate customer-facing bridges.
- **Approvals:** required before client delivery, scope or deadline commitment, invoice issuance, production mutation, or sharing across workspaces.
- **Safety defaults:** one client per workspace, no shared memory or caches, contract as authority, explicit bridge identity, approval before external send, and teardown on offboarding.
- **Current AGH fit:** **Platform evolution** — transactional dependency setup, workspace provisioning, client identity, update ownership, and isolation checks are required for a reusable product.
- **Evidence:** [PC-1], [PC-2], [OC-3].

# Additional compact candidates

These candidates are deliberately shorter than the flagships. They are additional opportunities, not claims of availability. “Composition” names the likely building blocks; any static skill, Loop, hook, tool, or MCP server mentioned in a candidate remains extension-scoped under the current model.

Every compact row inherits this structural declaration unless the row says otherwise:

- **Owner:** one candidate-specific owning extension, provisionally named from the candidate slug.
- **Profile resources:** one packaged coordinator agent named `<candidate>-coordinator`, its authored `AGENT.md`, and any explicitly named agents, jobs, triggers, AGH Network channels, or bridge presets. Soul and Heartbeat sidecars may add persona and wake/reentry context, but never operational authority or monitoring.
- **Dependencies:** every provider, connector, local binary, static skill, Loop, hook, tool, or MCP server named in “Composition.” A current-with-dependencies fit assumes each dependency is installed, configured, authorized, and healthy.
- **Automation target:** every compact job or trigger targets `<candidate>-coordinator` unless the row names another packaged agent or an explicit task configuration. If the composition includes a Loop, the coordinator requires `agh__loop_run` and starts that extension-scoped Loop; direct bundle-automation targets for Loops remain platform evolution.
- **Bridge lifecycle:** a named bridge preset projects a disabled instance. Secret binding, account selection, health checks, authorization, and enablement remain separate setup steps.

## Personal and family

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 001 | personal-renewal-watch | Individual: surface subscriptions, documents, cards, and memberships before renewal. | Email/document extraction, calendar, finance read connector, reminder agent, weekly job, approval before cancellation. | Platform evolution | [H-4], [OC-2] |
| 002 | household-grocery-planner | Household: merge meal constraints, pantry notes, and shared requests into one approved list. | Notes/tasks, calendar, grocery provider, planner agent, weekly job, family AGH Network channel, purchase approval. | Platform evolution | [H-4], [OC-7] |
| 003 | school-week-ahead | Parent or student: summarize assignments, events, forms, and supplies for the next seven days. | School email/LMS, calendar, documents, student-scoped agent, Sunday job, private delivery. | Platform evolution | [H-4], [OC-3] |
| 004 | caregiver-checklist | Family caregiver: coordinate nonclinical appointments, documents, transport, and follow-ups. | Calendar, secure notes, reminders, transport info, coordinator agent, strict privacy approvals. | Platform evolution | [OC-5] |
| 005 | household-document-vault | Household: classify warranties, receipts, policies, and renewal dates with provenance. | Document extraction, local knowledge store, librarian agent, expiry jobs, no external sharing. | Current with preinstalled dependencies | [OC-1], [OC-7] |
| 006 | moving-house-coordinator | Household: maintain a verified move checklist, provider changes, inventory, and deadlines. | Email, calendar, tasks, documents, utility connectors, coordinator agent, staged Loop, messaging. | Platform evolution | [OC-2] |
| 007 | pet-care-coordinator | Pet owner: track appointments, medication reminders supplied by a professional, supplies, and sitter handoffs. | Calendar, reminders, documents, messaging, pet-scoped agent, no medical inference. | Platform evolution | [H-4] |
| 008 | shared-expense-organizer | Household or roommates: collect receipts and prepare a transparent split proposal. | Document extraction, spreadsheet, payments read connector, reconciliation skill, approval before request. | Platform evolution | [OC-5] |
| 009 | digital-declutter-review | Individual: report duplicate files, stale downloads, and storage hotspots before any deletion. | Filesystem tool, cleanup hook, reviewer agent, scheduled scan, explicit delete approval. | Current with preinstalled dependencies | [H-6], [PI-1] |
| 010 | personal-reading-digest | Reader: summarize saved articles and create a source-linked weekly reading queue. | Read-later/RSS, browser, notes, curator agent, weekly job, no automatic publication. | Platform evolution | [H-2], [OC-2] |

## Local business and field service

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 011 | salon-booking-assistant | Salon owner: qualify requests, propose open slots, and draft confirmations. | WhatsApp/SMS, booking calendar, service catalog, receptionist agent, appointment trigger, approval. | Platform evolution | [OC-3], [PC-4] |
| 012 | repair-shop-intake | Repair shop: turn messages and photos into a factual intake record and inspection appointment. | Messaging/media, CRM, calendar, document store, intake agent, no diagnostic promise. | Platform evolution | [OC-1], [OC-3] |
| 013 | contractor-estimate-prep | Home-services contractor: collect scope and prepare an estimate worksheet for human pricing. | Forms, photos, maps, calendar, documents, estimator assistant, owner approval. | Platform evolution | [PI-3] |
| 014 | mobile-technician-dispatch | Field-service team: match approved jobs to available technicians and routes. | Work orders, calendar, maps, messaging, dispatcher agent, route job, reassignment approval. | Platform evolution | [OC-2] |
| 015 | restaurant-reservation-desk | Restaurant: answer policy questions and propose reservations from verified availability. | Booking, phone/chat, calendar, knowledge, receptionist agent, customer bridge. | Platform evolution | [OC-3] |
| 016 | menu-change-coordinator | Restaurant: trace ingredient, price, allergy, print, and digital-menu updates through approval. | Documents, inventory, CMS, design assets, checklist Loop, owner and safety approvals. | Platform evolution | [OC-2] |
| 017 | local-review-follow-up | Local business: summarize reviews and draft policy-compliant responses. | Review platforms, messaging, analyst/drafter agents, weekly job, publish approval. | Platform evolution | [OC-2] |
| 018 | appointment-no-show-reducer | Appointment business: identify reminder gaps and send approved staged reminders. | Booking/calendar, SMS/WhatsApp bridge, reminder job targeting the default coordinator, delivery cursor, opt-out and quiet-hour policy. | Current with preinstalled dependencies | [H-4], [OC-3] |
| 019 | quote-to-job-coordinator | Trades business: move an approved quote through scheduling, checklist, and customer updates. | CRM, documents, calendar, tasks, messaging, lifecycle Loop, approval at commitments. | Platform evolution | [PC-2] |
| 020 | daily-till-exception-brief | Retail manager: report cash/POS discrepancies and missing supporting records. | POS/accounting read connectors, documents, exception agent, close job, no ledger mutation. | Platform evolution | [OC-5] |

## Marketing, brand, and growth

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 021 | campaign-brief-builder | Marketer: turn approved goals, audience evidence, and constraints into a reviewable campaign brief. | Analytics, CRM segments, docs, researcher/strategist agents, brief Loop, approval. | Platform evolution | [OC-2], [PI-3] |
| 022 | social-calendar-editor | Social lead: create a destination-specific draft calendar with source and asset status. | Docs, social providers, asset store, strategist/editor agents, weekly job, publish gates. | Platform evolution | [H-4] |
| 023 | brand-claim-auditor | Brand/legal reviewer: locate unsupported or stale claims across approved pages and campaigns. | Browser/crawler, CMS, documents, claim-review skill, issue bridge, no auto-edit. | Current with preinstalled dependencies | [PI-1], [OC-2] |
| 024 | competitor-change-watch | Product marketer: report material competitor pricing, positioning, release, and hiring changes. | Browser/RSS, source archive, research agents, change trigger, weekly digest. | Current with preinstalled dependencies | [H-4], [OC-2] |
| 025 | launch-readiness-room | Product marketing: coordinate asset, docs, support, analytics, and announcement readiness. | Project tracker, docs, CMS, support, analytics, agents, milestone jobs, approval gates. | Platform evolution | [PC-2] |
| 026 | webinar-production-desk | Marketing team: coordinate speakers, registration, briefing, reminders, recording, and follow-up. | Calendar, webinar, CRM, email, documents, media, coordinator agents, lifecycle Loop. | Platform evolution | [OC-3] |
| 027 | case-study-editor | Customer marketer: convert approved interviews and metrics into a sourced case-study draft. | Transcript, CRM, analytics, documents, researcher/editor agents, customer approval. | Platform evolution | [OC-1] |
| 028 | landing-page-experiment-planner | Growth team: propose a measurable experiment with evidence, variants, and stop criteria. | Analytics, CMS, design, experimentation provider, strategist/reviewer agents, approval. | Platform evolution | [PI-3] |
| 029 | event-lead-follow-up | Event marketer: reconcile consented leads and prepare segmented follow-up drafts. | Forms/scanner, CRM, email, calendar, consent policy, follow-up jobs, send approval. | Platform evolution | [OC-5] |
| 030 | localization-release-coordinator | Global marketer: track translation, review, screenshots, and publication by locale. | CMS, translation provider, assets, reviewer agents, locale jobs, per-market approval. | Platform evolution | [OC-2] |

## Sales and customer success

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 031 | account-research-brief | Seller: prepare a cited account brief before a meeting. | CRM, web research, calendar, documents, research agent, meeting trigger, private delivery. | Current with preinstalled dependencies | [H-2], [OC-2] |
| 032 | crm-hygiene-review | Revenue operations: report duplicate contacts, missing next steps, and inconsistent stages. | CRM read tools, identity matching, reviewer agent, weekly job, approval before merge/update. | Platform evolution | [OC-5] |
| 033 | proposal-assembly-desk | Seller or agency: assemble a scope-grounded proposal draft from approved components. | CRM, documents, pricing catalog, e-signature, proposal agents, commercial approval. | Platform evolution | [PC-2] |
| 034 | renewal-readiness-brief | Customer success: summarize adoption, support, billing, stakeholders, and risks before renewal. | CRM, telemetry, support, billing, meetings, analyst agent, renewal-window jobs. | Platform evolution | [PC-2] |
| 035 | expansion-signal-review | Account manager: surface evidence-backed expansion signals without automated targeting. | Product analytics, CRM, support, billing, analyst agent, human opportunity decision. | Platform evolution | [OC-5] |
| 036 | partner-onboarding-coordinator | Partnerships lead: coordinate agreements, access, training, assets, and launch milestones. | CRM, contracts, identity requests, docs, calendar, lifecycle agents, approvals. | Platform evolution | [PC-1] |
| 037 | channel-sales-digest | Channel manager: produce partner pipeline and blocker summaries from approved systems. | Partner portal/CRM, email, docs, analyst agent, weekly job, workspace isolation. | Platform evolution | [OC-2] |
| 038 | sales-territory-handoff | Sales manager: prepare an auditable account handoff between representatives. | CRM, email/calendar metadata, documents, handoff Loop, manager approval, private AGH Network channel. | Platform evolution | [PC-2] |
| 039 | customer-qbr-builder | Success manager: draft a source-backed quarterly review with outcomes, gaps, and next decisions. | CRM, telemetry, support, docs/slides, analyst/editor agents, customer approval. | Platform evolution | [OC-2] |
| 040 | lost-deal-learning-review | Revenue team: cluster closed-lost evidence and separate facts from seller hypotheses. | CRM, call notes, email metadata, research agent, monthly job, no employee scoring. | Platform evolution | [OC-5] |

## Support, service, and community

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 041 | support-kb-gap-finder | Support lead: find repeated solved questions that lack approved knowledge. | Support tickets, knowledge base, clustering agent, weekly job, draft-only article output. | Platform evolution | [PC-2] |
| 042 | incident-ticket-cluster | Support/engineering: detect a likely shared incident across new tickets and status data. | Support, observability, product status, clustering trigger, human incident declaration. | Platform evolution | [OC-2] |
| 043 | refund-review-queue | Support manager: assemble policy, payment, order, and conversation evidence for refund decisions. | Support, payments, commerce, policy docs, reviewer agent, explicit money approval. | Platform evolution | [OC-5] |
| 044 | multilingual-support-drafter | Global support: draft policy-grounded replies in the customer's language with source parity. | Support bridge, translation, knowledge, reviewer agents, send approval. | Platform evolution | [OC-3] |
| 045 | community-faq-curator | Community operator: propose FAQ updates from repeated public questions and accepted answers. | Discord/Slack/forum, knowledge store, curator agent, weekly job, moderator approval. | Current with preinstalled dependencies | [OC-3], [H-2] |
| 046 | moderation-escalation-router | Community moderator: route policy-sensitive events to the right human role. | Community-bridge events, policy skill, role mapping, trigger targeting the default coordinator, private AGH Network review channel, no auto-ban. | Current with preinstalled dependencies | [OC-3], [OC-5] |
| 047 | developer-relations-digest | Developer relations: summarize SDK questions, bugs, examples, and requests across community surfaces. | GitHub, Discord/Slack, docs, issue tracker, analyst agent, weekly job. | Platform evolution | [OC-3], [OC-4] |
| 048 | member-renewal-coordinator | Association/community: prepare consented renewal reminders and unresolved-benefit questions. | Membership CRM, billing, email, documents, staged jobs, send and billing approvals. | Platform evolution | [OC-5] |
| 049 | volunteer-community-desk | Volunteer lead: coordinate shifts, questions, training, and urgent handoffs. | Forms, calendar, messaging, docs, coordinator agent, reminder jobs, private data policy. | Platform evolution | [H-4] |
| 050 | customer-advisory-board-coordinator | Product team: manage invitations, agendas, consented notes, actions, and feedback themes. | CRM, email/calendar, meetings, docs, research agents, follow-up Loop. | Platform evolution | [PC-2] |

## Finance, procurement, and administration

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 051 | receipt-completeness-review | Finance admin: match uploaded receipts to transactions and flag gaps. | Bank/accounting read connector, document extraction, matching skill, monthly job. | Platform evolution | [OC-1] |
| 052 | subscription-spend-review | Finance or IT: inventory recurring software spend, owners, usage evidence, and renewals. | Accounting, cards, SSO/app catalog, contracts, analyst agent, renewal jobs. | Platform evolution | [OC-5] |
| 053 | purchase-request-coordinator | Operations: collect need, vendor, budget, security, and approver evidence before purchase. | Forms, procurement, budget, security docs, approval Loop, messaging. | Platform evolution | [PC-2] |
| 054 | vendor-renewal-desk | Procurement: prepare renewal packet with contract terms, usage, incidents, and alternatives. | Contracts, spend, telemetry, support, research agent, deadline jobs. | Platform evolution | [H-4] |
| 055 | expense-policy-reviewer | Finance admin: classify expense exceptions against approved policy without making final decisions. | Expense system, policy docs, document extraction, reviewer agent, human disposition. | Platform evolution | [OC-5] |
| 056 | budget-variance-explainer | Department lead: explain material budget variances with source links and explicit assumptions. | Accounting, budget spreadsheet, procurement, analyst agent, monthly job. | Platform evolution | [PC-1] |
| 057 | payment-failure-coordinator | Billing operations: route failed payments, draft notices, and track resolution. | Billing provider, CRM, email/SMS, retry events, staged Loop, customer-contact approval. | Platform evolution | [OC-2] |
| 058 | contract-obligation-calendar | Operations: extract approved dates, notice windows, and obligations into a review queue. | Document extraction, contract store, calendar, legal-review approval, reminder jobs. | Platform evolution | [OC-1] |
| 059 | inventory-reorder-review | Operations: produce evidence-backed reorder suggestions from stock, lead time, and demand. | Inventory, sales, supplier data, analyst agent, threshold triggers, purchase approval. | Platform evolution | [OC-5] |
| 060 | month-end-management-pack | Leadership: assemble a cited monthly operating pack from approved finance and KPI sources. | Accounting, analytics, spreadsheets/slides, analyst/editor agents, close job, share approval. | Platform evolution | [OC-2] |

## Legal, compliance, and governance

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 061 | policy-acknowledgment-coordinator | Compliance lead: track approved policy distribution, acknowledgment, and exceptions. | HRIS/identity, documents, forms, reminder jobs, private escalation. | Platform evolution | [PC-2] |
| 062 | privacy-request-intake | Privacy team: structure a data-subject request, verify required intake, and route it. | Secure forms, identity verification, case tracker, documents, deadline jobs, no auto-fulfillment. | Platform evolution | [OC-5] |
| 063 | retention-review-queue | Records manager: identify records reaching policy review dates without deleting them. | Document stores, policy rules, inventory agent, scheduled job, explicit deletion approval. | Platform evolution | [OC-5] |
| 064 | compliance-evidence-collector | Audit owner: gather control evidence, map it to requirements, and flag stale artifacts. | Ticketing, cloud/security systems, documents, evidence agent, periodic jobs. | Platform evolution | [PC-2] |
| 065 | contract-redline-prep | Legal team: produce a clause comparison and factual deviation matrix for attorney review. | Document extraction, clause library, comparison skill, legal reviewer, no autonomous advice. | Current with preinstalled dependencies | [OC-1] |
| 066 | accessibility-issue-coordinator | Accessibility lead: consolidate audits, user reports, evidence, owners, and remediation status. | Browser/a11y tools, support, issue tracker, reviewer agents, release jobs. | Platform evolution | [PI-1] |
| 067 | vendor-security-review | Security/procurement: assemble questionnaire evidence, gaps, and follow-up questions. | Forms, docs, security evidence store, researcher agent, approval before representations. | Platform evolution | [OC-5] |
| 068 | regulatory-change-watch | Compliance specialist: monitor approved primary sources and report material changes. | Web/RSS, source archive, research agent, change trigger, legal review. | Current with preinstalled dependencies | [H-4], [OC-2] |
| 069 | board-resolution-coordinator | Company secretary: prepare agenda, evidence packet, draft resolution, signatures, and archive. | Documents, calendar, e-signature, governance agent, strict approval chain. | Platform evolution | [PC-1] |
| 070 | audit-remediation-tracker | Audit owner: convert accepted findings into evidence-linked actions and monitor closure. | Audit docs, issue tracker, messaging, coordinator agent, milestone jobs, reviewer signoff. | Platform evolution | [PC-2] |

## People operations and workplace

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 071 | interview-packet-builder | Hiring team: assemble role-grounded interview packets and scorecard reminders. | ATS, docs, calendar, recruiter agent, interview trigger, no candidate ranking. | Platform evolution | [PC-2] |
| 072 | candidate-scheduling-desk | Recruiter: resolve interviewer constraints and draft candidate scheduling options. | ATS, calendar, email, coordinator agent, approval before send. | Platform evolution | [OC-3] |
| 073 | hiring-feedback-chaser | Recruiting operations: remind interviewers and report missing scorecards without inferring verdicts. | ATS, calendar, messaging bridge, reminder jobs targeting the default coordinator, private visibility. | Current with preinstalled dependencies | [H-4] |
| 074 | role-description-review | People lead: compare a role draft with approved leveling, accessibility, and policy guidance. | HR docs, compensation bands, reviewer agents, no automatic publication. | Current with preinstalled dependencies | [OC-5] |
| 075 | internal-mobility-coordinator | HR partner: assemble consented skills, openings, and process steps for human review. | HRIS, job catalog, learning system, coordinator agent, strict privacy. | Platform evolution | [OC-5] |
| 076 | manager-one-on-one-prep | Manager: prepare a private agenda from explicit commitments and prior agreed notes. | Calendar, private notes, tasks, preparation agent, recurring job, no employee scoring. | Platform evolution | [H-4] |
| 077 | team-capacity-brief | Team lead: summarize committed work, leave, and blockers without productivity surveillance. | Project tracker, calendar availability, coordinator agent, weekly job. | Platform evolution | [PC-2] |
| 078 | learning-path-coordinator | L&D lead: propose role-specific approved training and track completion. | LMS, HRIS, calendar, learning curator, reminder jobs, manager approval. | Platform evolution | [H-2] |
| 079 | offboarding-coordinator | People/IT: coordinate approved access removal, equipment, documents, and handoffs. | HRIS, identity/ticketing, assets, docs, checklist Loop, authorization gates. | Platform evolution | [OC-5] |
| 080 | workplace-event-organizer | Office/community lead: manage venue, invitations, dietary/access needs, agenda, and follow-up. | Forms, calendar, email, documents, coordinator agent, consent and spending approvals. | Platform evolution | [H-4] |

## Education and training

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 081 | syllabus-to-calendar | Student: extract verified dates and requirements from a syllabus into a reviewable calendar plan. | Document extraction, calendar, planner agent, date verification, event-write approval. | Platform evolution | [OC-1] |
| 082 | flashcard-curator | Learner: turn approved notes into source-linked flashcards and a spaced-review queue. | Notes/docs, flashcards, tutor agent, scheduled jobs, no graded-answer generation. | Platform evolution | [H-2], [H-4] |
| 083 | reading-seminar-prep | Student or book group: prepare a cited reading guide, questions, and disputed interpretations. | Documents, notes, research agents, scheduled discussion job, private AGH Network channel. | Current with preinstalled dependencies | [PI-3] |
| 084 | language-practice-coach | Language learner: run bounded practice, record corrections, and schedule review. | Speech/text provider, tutor agent, learning skill, daily job, privacy controls. | Platform evolution | [OC-1] |
| 085 | course-material-accessibility-review | Instructor: flag accessibility gaps in approved course materials. | Document/media extraction, accessibility tools, reviewer agent, issue export. | Current with preinstalled dependencies | [PI-1] |
| 086 | office-hours-coordinator | Instructor or teaching assistant: triage questions, schedule sessions, and surface common topics. | LMS, calendar, messaging, FAQ knowledge, coordinator agent, no grading authority. | Platform evolution | [OC-3] |
| 087 | assignment-feedback-organizer | Instructor: organize rubric-grounded draft feedback for human grading. | LMS, documents, rubric, reviewer agent, final-grade approval. | Platform evolution | [OC-5] |
| 088 | research-supervision-brief | Advisor: summarize agreed milestones, evidence, blockers, and next discussion points. | Notes, calendar, documents, research tracker, recurring job, private delivery. | Platform evolution | [PC-2] |
| 089 | certification-study-plan | Professional learner: map an official exam outline to a bounded study and practice schedule. | Official docs, calendar, flashcards, tutor agents, weekly review. | Platform evolution | [H-2] |
| 090 | workshop-facilitator | Trainer: coordinate registration, materials, exercises, timing, feedback, and follow-up. | Forms, calendar, docs, messaging, facilitator agent, event lifecycle Loop. | Platform evolution | [H-4] |

## Creator and media operations

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 091 | podcast-guest-research | Podcast host: prepare a cited guest brief, topic map, and avoid-list. | Web research, calendar, CRM/notes, researcher agent, pre-recording job. | Current with preinstalled dependencies | [H-2], [OC-2] |
| 092 | podcast-postproduction-desk | Producer: coordinate transcript, edit notes, chapters, assets, approval, and distribution. | Media tools, docs, asset store, publishing bridges, agents, lifecycle Loop. | Platform evolution | [OC-1] |
| 093 | video-chapter-editor | Video creator: derive timestamped chapters and descriptions from an approved transcript. | Transcription/video tools, editor agent, timestamp verification, export approval. | Current with preinstalled dependencies | [PI-2] |
| 094 | short-form-clip-review | Creator: surface self-contained clip candidates with exact timestamps and rights status. | Video/transcript, rights metadata, clip agent, human selection, no auto-post. | Platform evolution | [OC-1] |
| 095 | thumbnail-concept-board | Creator: generate evidence-informed thumbnail concepts and review criteria, not performance promises. | Analytics, image provider, brand assets, creative/reviewer agents, approval. | Platform evolution | [OC-1] |
| 096 | livestream-run-of-show | Host: coordinate agenda, guests, assets, moderation roles, cues, and fallback plan. | Calendar, docs, streaming-bridge tools, coordinator agents, event jobs. | Platform evolution | [OC-3] |
| 097 | sponsorship-operations-desk | Creator: track sponsor requirements, claims, assets, approvals, deadlines, and proof of delivery. | CRM, contracts, docs, media analytics, coordinator agent, commercial approvals. | Platform evolution | [PC-2] |
| 098 | rights-and-attribution-check | Media team: flag missing licenses, attribution, consent, or source metadata before publication. | Asset store, contracts, document extraction, rights reviewer, publish gate. | Platform evolution | [OC-5] |
| 099 | audio-accessibility-pack | Producer: create transcript, captions, chapters, and accessible show notes from approved audio. | STT, docs, accessibility skill, editor agent, source alignment. | Platform evolution | [OC-1] |
| 100 | creator-community-office-hours | Creator: collect member questions, cluster themes, prepare answers, and schedule a live session. | Community bridge, forms, calendar, knowledge, curator agent, event job. | Platform evolution | [OC-3] |

## Nonprofit, association, and public-good operations

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 101 | donor-thank-you-desk | Nonprofit fundraiser: prepare consented, factually grounded acknowledgment drafts. | Donor CRM, payments, documents, email, drafting agent, send approval. | Platform evolution | [OC-5] |
| 102 | volunteer-shift-coordinator | Volunteer manager: fill approved shifts, send reminders, and report gaps. | Forms/CRM, calendar, messaging, coordinator agent, reminder jobs. | Platform evolution | [H-4] |
| 103 | program-impact-evidence-organizer | Program lead: map approved outputs and stories to source records for human reporting. | CRM/case system, docs, analytics, evidence agent, no invented outcomes. | Platform evolution | [OC-5] |
| 104 | donation-reconciliation-review | Treasurer: match donations, processor records, and acknowledgments into an exception queue. | Payments, accounting, donor CRM, matching skill, finance approval. | Platform evolution | [OC-5] |
| 105 | community-resource-directory | Mutual-aid or public-service team: maintain a verified, dated directory of local resources. | Web/forms, maps, database, verifier agents, expiry jobs, public-publish approval. | Platform evolution | [H-4] |
| 106 | public-meeting-digest | Civic organization: produce a cited agenda/minutes/action digest from public records. | Public web/docs, transcript, research agent, scheduled job, publication review. | Current with preinstalled dependencies | [H-2] |
| 107 | advocacy-campaign-coordinator | Nonprofit: manage evidence, approved messaging, volunteers, events, and consented outreach. | CRM, docs, calendar, messaging, campaign agents, hard publish/send approvals. | Platform evolution | [OC-3] |
| 108 | membership-renewal-brief | Association: report renewal status, unresolved service issues, and draft next steps. | Membership CRM, billing, support, email, analyst agent, renewal jobs. | Platform evolution | [PC-2] |
| 109 | event-scholarship-review-organizer | Nonprofit: prepare eligibility evidence for human scholarship decisions. | Forms, documents, policy rubric, reviewer agent, no autonomous award verdict. | Platform evolution | [OC-5] |
| 110 | board-packet-coordinator | Nonprofit board: assemble agenda, prior actions, finance reports, decisions, and acknowledgments. | Docs, calendar, finance read connectors, coordinator agent, secure delivery. | Platform evolution | [PC-1] |

## Real estate, property, and hospitality

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 111 | listing-launch-coordinator | Real-estate team: track approved property facts, media, disclosures, publication destinations, and launch readiness. | Listings, CRM, docs, media, project tracker, agents, publication approval. | Platform evolution | [PC-2] |
| 112 | showing-follow-up-desk | Agent: prepare personalized, factual follow-up drafts and log feedback after showings. | CRM, calendar, messaging, forms, follow-up trigger, send approval. | Platform evolution | [OC-3] |
| 113 | transaction-deadline-watch | Transaction coordinator: monitor contingencies, documents, signatures, and deadlines. | Transaction system, docs, e-signature, calendar, jobs, no legal interpretation. | Platform evolution | [H-4] |
| 114 | rental-inquiry-intake | Property manager: qualify inquiries against published criteria and propose viewings. | Listings, CRM, calendar, messaging, intake agent, fair-housing policy gate. | Platform evolution | [OC-5] |
| 115 | tenant-maintenance-desk | Property manager: structure maintenance requests, gather media, route urgency, and schedule approved work. | Tenant-messaging bridge, work orders, media, calendar, dispatcher agent, emergency escalation. | Platform evolution | [OC-3] |
| 116 | property-inspection-organizer | Inspector/manager: assemble checklist, images, observations, owners, and follow-up evidence. | Mobile forms/media, docs, task tracker, inspection agent, human signoff. | Platform evolution | [PI-3] |
| 117 | short-stay-turnover-coordinator | Hospitality operator: coordinate checkout, cleaning, inspection, supplies, and next arrival. | PMS, calendar, task/field service, messaging, turnover trigger, exception alerts. | Platform evolution | [H-4] |
| 118 | guest-review-insights | Hospitality operator: cluster review evidence into property, service, and policy improvements. | Booking/review platforms, analyst agent, monthly job, no guest profiling. | Platform evolution | [OC-2] |
| 119 | venue-event-operations | Venue manager: coordinate booking, layout, vendors, access, safety, and settlement. | CRM, calendar, docs, vendors, tasks, coordinator agents, spending approvals. | Platform evolution | [PC-2] |
| 120 | hotel-night-audit-brief | Hotel manager: report reservation, payment, room-status, and handoff exceptions for human audit. | PMS, payments read connector, operations logs, analyst agent, nightly job. | Platform evolution | [OC-5] |

## Health administration and wellness

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 121 | appointment-prep-checklist | Patient or practice admin: assemble nonclinical documents, logistics, and questions before an appointment. | Calendar, secure docs, reminders, checklist agent, no medical interpretation. | Platform evolution | [H-4] |
| 122 | referral-admin-tracker | Practice admin: track referral documents, scheduling state, and administrative follow-ups. | Secure case system, documents, calendar, messaging, deadline jobs. | Platform evolution | [PC-2] |
| 123 | benefits-document-organizer | Individual: classify insurance/benefits documents and surface stated deadlines and missing forms. | Document extraction, secure local store, calendar, organizer agent, no coverage advice. | Current with preinstalled dependencies | [OC-1] |
| 124 | medication-reminder-transcriber | Individual: turn a clinician-provided schedule into confirmed reminders without recommending changes. | Approved document input, reminders, messaging bridge, confirmation Loop, authored no-advice constraint plus runtime approvals. | Platform evolution | [OC-5] |
| 125 | wellness-habit-checkin | Individual: run consented habit check-ins and summarize self-reported trends. | Calendar/reminders, private notes, coach agent, daily job, no diagnosis. | Platform evolution | [H-4] |
| 126 | fitness-plan-calendar | User and qualified coach: translate an approved plan into reminders and completion logs. | Calendar, fitness tracker, documents, coach agent, plan-change approval. | Platform evolution | [OC-7] |
| 127 | meal-plan-organizer | Household: create an allergy-aware meal plan and grocery draft from explicit preferences. | Notes, calendar, grocery provider, planner agent, no medical nutrition claims. | Platform evolution | [H-4] |
| 128 | sleep-routine-review | Individual: summarize self-reported and device data against user-defined goals. | Sleep device, calendar, weather, notes, analyst agent, no health diagnosis. | Platform evolution | [OC-7] |
| 129 | care-team-logistics-brief | Family caregiver: coordinate approved contacts, appointments, transport, and document tasks. | Secure contacts, calendar, docs, messaging, coordinator agent, minimum necessary data. | Platform evolution | [OC-5] |
| 130 | practice-cancellation-fill-desk | Practice admin: offer approved openings to consented waitlist members in order. | Scheduling, waitlist, messaging, idempotent trigger, privacy and human override. | Platform evolution | [OC-3] |

## Research, knowledge, and analysis

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 131 | literature-watch | Researcher: report new primary literature matching a bounded query and explain relevance. | Academic search/RSS, citation store, research agent, scheduled job. | Current with preinstalled dependencies | [H-2], [H-4] |
| 132 | evidence-map-builder | Analyst: map claims, supporting/contradicting sources, quality, and gaps. | Search/browser, documents, citation skill, skeptic agents, research Loop. | Current with preinstalled dependencies | [PI-3] |
| 133 | policy-comparison-desk | Policy analyst: compare official texts by jurisdiction and date with explicit source scope. | Primary-source web/docs, extraction, comparison agent, change jobs, no legal advice. | Platform evolution | [OC-1] |
| 134 | market-landscape-map | Strategist: build a sourced entity, segment, offering, and uncertainty map. | Web research, company data, documents, researcher agents, bounded budget. | Current with preinstalled dependencies | [H-2] |
| 135 | interview-synthesis-desk | Researcher: code approved interview transcripts into themes with traceable excerpts. | Transcript/docs, qualitative-analysis skill, researcher/reviewer agents, consent policy. | Current with preinstalled dependencies | [PI-3] |
| 136 | patent-scouting-brief | Authorized researcher: summarize relevant public patents and citations without legal conclusions. | Patent search, browser, documents, research agent, primary-source links. | Platform evolution | [H-2] |
| 137 | scientific-reproducibility-check | Research team: inspect methods, data availability, code, and stated limitations. | Papers/docs, repositories, sandbox tools, reviewer agents, no unsupported validity verdict. | Current with preinstalled dependencies | [PI-1] |
| 138 | due-diligence-data-room-index | Authorized deal team: inventory and classify supplied documents with gaps and provenance. | Secure documents, extraction, indexer agents, strict workspace scope, private AGH Network review channel. | Platform evolution | [OC-1] |
| 139 | expert-network-prep | Consultant: prepare source-backed expert interview questions and conflict notes. | CRM, web research, calendar, docs, researcher agent, outreach approval. | Platform evolution | [OC-5] |
| 140 | knowledge-base-freshness-watch | Knowledge owner: flag pages whose sources, owners, links, or product facts appear stale. | Knowledge system, link checker, runtime/docs sources, reviewer agent, scheduled job. | Current with preinstalled dependencies | [OC-2] |

## Engineering, product, operations, security, and data

| No. | Candidate | Persona and outcome | Likely composition | Current AGH fit | Evidence |
| ---: | --- | --- | --- | --- | --- |
| 141 | github-issue-triage | Maintainer: classify, deduplicate, request reproduction evidence, and suggest owners. | GitHub, workspace search, triage agents, issue trigger, comment approval. | Current with preinstalled dependencies | [OC-4] |
| 142 | flaky-test-investigator | Engineer: correlate test failures, revisions, timing, and environment into a reproducible hypothesis. | CI, source control, workspace/sandbox, investigator agents, bounded reruns. | Current with preinstalled dependencies | [PI-3] |
| 143 | dependency-update-coordinator | Maintainer: prepare a bounded dependency update with impact, tests, changelog, and rollback. | Package registries, source control, workspace, CI, implementation/review agents. | Current with preinstalled dependencies | [PI-1] |
| 144 | migration-readiness-audit | Engineering lead: inventory delete targets, contracts, storage, docs, and downstream consumers before a hard cut. | Workspace/code search, docs, schema/API tools, architecture agents, report Loop. | Current with preinstalled dependencies | [PI-3] |
| 145 | api-contract-change-room | API owner: coordinate DTO, schema, generated clients, mocks, docs, and compatibility evidence. | Source control, OpenAPI/codegen, workspace, CI, agents, change trigger. | Current with preinstalled dependencies | [OC-2] |
| 146 | database-migration-review | Database owner: verify append-only identity, schema equivalence, checksums, codegen, and rollback assumptions. | Database/schema tools, source control, CI, reviewer agents, approval gates. | Current with preinstalled dependencies | [PI-1] |
| 147 | performance-regression-investigator | Engineer: locate a measured regression and produce profile-backed candidate causes. | Benchmarks/profiler, CI, source control, workspace, investigator agents. | Current with preinstalled dependencies | [PI-1] |
| 148 | production-readiness-review | Service owner: assess operability, failure paths, metrics, security, capacity, and recovery evidence. | Repo, observability, deployment config, docs, reviewer agents, checklist Loop. | Current with preinstalled dependencies | [OC-2] |
| 149 | service-catalog-curator | Platform team: maintain service ownership, dependencies, runbooks, and freshness. | Catalog, repos, observability, docs, curator agent, periodic jobs. | Platform evolution | [PC-2] |
| 150 | on-call-handoff-brief | Operations team: summarize active incidents, changes, risks, and pending actions at shift change. | Alerting, incidents, deployment, tasks, coordinator agent, scheduled handoff. | Platform evolution | [OC-3] |
| 151 | deployment-verification-desk | Release/operator: verify health, logs, metrics, and key journeys after an approved deployment. | Deployment provider, observability, browser/API checks, verification Loop, rollback approval. | Platform evolution | [OC-2] |
| 152 | infrastructure-drift-review | Platform team: report approved desired-versus-observed infrastructure drift before remediation. | IaC repo, cloud inventory, policy tools, reviewer agent, scheduled scan. | Platform evolution | [OC-5] |
| 153 | secret-rotation-coordinator | Security/operations: inventory expiring secret bindings and coordinate approved rotation checks. | Secret manager, service catalog, CI/deploy, coordinator agent, strict no-secret-output policy. | Platform evolution | [OC-5] |
| 154 | vulnerability-remediation-desk | Security engineer: route verified findings, prepare fixes, test, and track disclosure-safe closure. | Scanner, source control, issue tracker, sandbox, agents, review Loop. | Current with preinstalled dependencies | [H-6], [OC-5] |
| 155 | access-review-coordinator | Security/IT: assemble role, account, owner, and usage evidence for human access certification. | Identity provider, HRIS, app catalog, reviewer agent, periodic job. | Platform evolution | [OC-5] |
| 156 | cost-anomaly-investigator | FinOps/platform: explain cloud-cost anomalies with service, usage, deployment, and owner evidence. | Cloud billing, telemetry, deployment, catalog, analyst agent, threshold trigger. | Platform evolution | [OC-2] |
| 157 | data-backfill-coordinator | Data engineer: plan, approve, execute, verify, and document an idempotent backfill. | Warehouse/orchestrator, source control, observability, repair Loop, approvals. | Platform evolution | [PC-2] |
| 158 | schema-change-impact-review | Data team: trace a schema change through producers, consumers, tests, dashboards, and docs. | Catalog/lineage, repos, BI, issue tracker, researcher agents. | Platform evolution | [OC-2] |
| 159 | product-feedback-synthesis | Product manager: consolidate support, sales, community, and interview evidence into traceable themes. | CRM, support, community, research docs, synthesis agents, weekly job. | Platform evolution | [PC-2] |
| 160 | roadmap-evidence-review | Product leadership: connect roadmap candidates to customer, usage, risk, and delivery evidence. | Product tracker, analytics, CRM, research, finance, reviewer agents, decision approval. | Platform evolution | [PC-1], [PC-2] |

# Portfolio sequence

The catalog should not be implemented from top to bottom. A smaller set of reusable foundations can support many outcomes.

## Foundation extensions

1. **Guided setup and connection health**
   - Account and tenant selection.
   - Secret binding without exposing secret values.
   - Read-only connection test.
   - Extension service-interface, resource-grant, and permission summary.
   - Demo and dry-run support.

2. **Outcome-critical work systems**
   - Google Workspace and Microsoft 365.
   - GitHub/GitLab and Linear/Jira.
   - Slack, Teams, Telegram, WhatsApp, Discord, and email.
   - Notion, common task systems, and document extraction.
   - A webhook/automation bridge for n8n, Make, Zapier, or internal services.

3. **Business systems**
   - CRM contract with at least one provider.
   - Support contract with at least one provider.
   - Commerce and payments contracts.
   - Calendar/meeting transcript contract.
   - Analytics and CMS contracts.

4. **Safety and lifecycle**
   - Resource, service-interface, and security-grant diff before activation.
   - Approval delivery through the user's chosen bridge or local operator surface.
   - Per-run and recurring cost limits.
   - Idempotency and duplicate-event handling.
   - Retry, catch-up, and circuit-breaker policy.
   - Workspace and tenant isolation checks.
   - Update ownership, deactivation, uninstall, and rollback.

## Suggested first outcome collection

| Order | Bundle | Why it earns an early validation slot | Required foundation |
| ---: | --- | --- | --- |
| 1 | personal-chief-of-staff | Read-only first proof; broad individual appeal; tests calendar, email, tasks, and delivery. | Workspace/mail/calendar, messaging, guided setup |
| 2 | meeting-to-action | One bounded input and inspectable output; useful across sales, recruiting, product, and management. | Transcript, calendar, task system, approvals |
| 3 | fix-linear-issue | Exercises the AGH premise across agent, CLI/API, bridge, deterministic Loop, review, and observable proof. | Linear, GitHub, coding runtime, CI |
| 4 | customer-support-desk | Clear draft-only mode and measurable queue outcome. | Support platform, knowledge, messaging bridge, identity |
| 5 | content-marketing-engine | Lets domain experts author bundle context while connector maintainers own integrations. | Research, docs, CMS, assets, approvals |
| 6 | sales-pipeline-assistant | Recurring value with explicit source-of-record and send gates. | CRM, email, calendar, transcript |
| 7 | ecommerce-operator | Combines read-only reporting with carefully gated money and inventory actions. | Commerce, payments, support, inventory |
| 8 | local-business-front-desk | Tests nontechnical setup and bidirectional messaging. | Customer bridge, calendar, knowledge, identity |
| 9 | family-command-center | Tests private multi-person delivery and least-data policy. | Shared calendar/tasks, messaging, identity |
| 10 | incident-response-room | Tests high-stakes approvals, event correlation, budgets, and recovery. | Observability, deployment, on-call bridge |

# Platform evolution implied by the catalog

The following items are requirements suggested by many candidates, not declarations of current behavior.

## Dependency-aware bundle installation

A portable bundle needs to declare required and optional extensions. Installation should resolve exact versions, compatibility, conflicts, and checksums before activation. The plan should be inspectable and transactional:

1. Resolve dependencies and provider choices.
2. Show requested extension service interfaces, resource grants, and security grants.
3. Install or select extensions.
4. Bind secrets and accounts.
5. Test connection health.
6. Materialize profile-scoped resources.
7. Run a dry first proof.
8. Activate jobs, triggers, and bridges only after explicit confirmation.
9. Preserve enough state for rollback and clean deactivation.

The current bundle profile does not perform these steps. It assumes its owning extension is already installed and only projects the resource classes listed in the runtime-truth section.

## Profile-scoped static resources

Many blueprints need a skill, Loop, hook, tool, or MCP server only while one profile is active. Today those resources are extension-scoped. A future design would need to decide whether to:

- allow a profile to own and project those static resources;
- keep resources extension-scoped but add explicit profile enablement and grants; or
- make bundles depend on separately versioned provider or service extensions.

The decision must preserve provenance, deterministic precedence, least privilege, workspace isolation, and clean deactivation.

## Provider service contracts

A bundle that asks for “calendar,” “customer support,” “speech,” or “image generation” should not silently assume a vendor. A provider contract could let setup select one compatible extension while preserving a stable outcome-level requirement.

Contracts should describe:

- operations and event families;
- read versus mutation operations;
- identity and tenant semantics;
- pagination, ordering, and freshness;
- delivery and idempotency guarantees;
- approval requirements;
- health and degraded-state reporting;
- portable configuration fields;
- test fixtures and compatibility evidence.

This is a platform-evolution proposal. Provider service contracts must not be confused with current AGH Network Capabilities, which are interpretive peer offers rather than local provider interfaces.

## Setup schema and first-proof contract

Each bundle should be able to declare:

- required and optional accounts;
- user-facing questions and safe defaults;
- account/tenant/workspace selectors;
- bridge recipients and quiet hours;
- job schedule in human language;
- approval policy choices;
- data retention and workspace scope;
- a sample-data demonstration;
- a read-only real-data dry run;
- a machine-checkable first successful outcome.

“Installed” should not be presented as “operational” until required connections pass health checks and the first proof succeeds.

## Permission synthesis

Users should not need to understand each tool schema to make a safety decision. The activation preview should summarize what the complete bundle may:

- read;
- write;
- send;
- publish;
- execute;
- delete;
- purchase or refund;
- schedule;
- expose over a network;
- retain;
- share across AGH Network channels or workspaces.

The summary must be derived from actual extension and resource declarations, not manually maintained marketing copy.

## Runtime budgets and recovery

Recurring bundles need policy for:

- maximum cost per run and period;
- maximum child agents and depth;
- maximum retries and catch-up executions;
- duplicate event keys;
- timeouts and stalled-state handling;
- circuit-breaker threshold;
- approval expiry;
- quiet hours;
- no-op delivery;
- human escalation;
- teardown and cleanup.

These controls are especially important for incident, finance, messaging, smart-home, and external-publication bundles.

## Distribution and community

A healthy catalog can separate:

- **Official** — maintained and compatibility-tested by the AGH project.
- **Verified** — publisher identity, provenance, security scan, compatibility evidence, and support metadata have been checked.
- **Community** — discoverable with explicit trust and maintenance caveats.
- **Local** — private or unpublished development packages.

Automated indexing can reduce publishing friction, but indexing is not verification. Listings should expose publisher, source revision, checksum/signature, requested resource, service, and security grants, dependencies, supported AGH versions, last verification, setup time, sample output, health checks, changelog, maintainer contact, and uninstall behavior.

# Blueprint readiness checklist

A flagship should not enter implementation planning until its discovery work can answer all of these:

- [ ] The persona and system of record are explicit.
- [ ] The first proof is read-only or otherwise safely reversible.
- [ ] The deterministic Loop has a definition of done, verification gate, budget, and named terminal outcomes.
- [ ] Every external dependency has an owner and a real integration path.
- [ ] Agent Souls express persona, principles, and constraints without claiming operational authority.
- [ ] Heartbeat sidecars define bounded wake/reentry guidance; jobs, triggers, task APIs, and runtime health surfaces own observation and health.
- [ ] Jobs and triggers name an agent or task target and define retry, catch-up, deduplication, and circuit-breaker behavior; a current-fit Loop starts through an agent granted `agh__loop_run`.
- [ ] AGH Network channel identity, external bridge identity, and tenant/workspace mapping are explicit.
- [ ] Every consequential action has an approval rule.
- [ ] Data scope and retention are explicit.
- [ ] Setup, health check, demo, dry run, and first proof are defined.
- [ ] Update, deactivation, uninstall, and rollback ownership are defined.
- [ ] CLI, HTTP, UDS, native tools, web, and official AGH skill impacts are analyzed.
- [ ] Documentation avoids presenting the opportunity as shipped behavior.

# Evidence appendix

## How to read the evidence codes

- AGH codes establish the current implementation boundary and therefore control fit labels.
- Hermes, Pi, OpenClaw, and Paperclip codes establish observed ecosystem, packaging, onboarding, automation, or community patterns.
- An external implementation is not evidence that AGH has the same connector.
- Paperclip's plugin specification is explicitly a proposed target layered over an early plugin runtime; [PC-3] is architecture inspiration, not a current-feature claim.
- Counts and volatile marketplace metrics are intentionally omitted from opportunity reasoning unless dated.

## AGH implementation evidence

| Code | Source | Relevant evidence |
| --- | --- | --- |
| AGH-1 | internal/extension/bundle.go | BundleProfile currently contains AGH Network channel declarations, packaged agents with Soul/Heartbeat sidecars, agent- or task-targeted jobs, agent-targeted triggers, and bridge presets. It has no direct Loop target fields. |
| AGH-2 | internal/extension/manifest.go | Skills, Loops, agents, bundles, hooks, tools, and MCP servers are static extension resources; manifests also declare extension service interfaces under `capabilities.provides`, Host API actions, subprocess, security, and bridge metadata. |
| AGH-3 | internal/bundles/service.go; internal/bundles/resource.go; internal/bundles/resource_projection.go | Bundle catalog, preview, activation, workspace/global scope, materialization, reconciliation, and owned-resource projection are current runtime responsibilities. Bridge presets materialize disabled instances; preset secret-slot declarations are setup metadata rather than bound secrets. |

## Hermes evidence

| Code | Source | Relevant evidence |
| --- | --- | --- |
| H-1 | .resources/hermes/website/docs/user-guide/features/plugins.md; .resources/hermes/website/docs/developer-guide/plugins/index.md | Plugin discovery, opt-in enablement, tools, hooks, commands, providers, platform backends, skills, environment gates, and project trust. |
| H-2 | .resources/hermes/website/docs/user-guide/features/skills.md; .resources/hermes/optional-skills/ | Skills Hub sources, provenance, trust levels, scanning, updates, and a broad domain inventory. |
| H-3 | .resources/hermes/website/docs/user-guide/profile-distributions.md | Git-distributed agent profiles can package Soul, configuration, skills, cron, and MCP configuration while preserving user-owned state and excluding secrets. |
| H-4 | .resources/hermes/cron/blueprint_catalog.py; .resources/hermes/cron/suggestion_catalog.py; .resources/hermes/cron/suggestions.py; .resources/hermes/website/docs/guides/automation-blueprints.md | Human-readable automation forms, consent-first suggestions, deduplication, reminders, briefs, reviews, monitoring, and business/technical examples. |
| H-5 | .resources/hermes/optional-mcps/linear/manifest.yaml; .resources/hermes/optional-mcps/n8n/manifest.yaml; .resources/hermes/optional-mcps/unreal-engine/manifest.yaml | OAuth/local MCP setup, tool selection, read-only defaults, mutation pruning, and explicit experimental/local warnings. |
| H-6 | .resources/hermes/plugins/google_meet/; .resources/hermes/plugins/teams_pipeline/; .resources/hermes/plugins/platforms/homeassistant/; .resources/hermes/plugins/security-guidance/; .resources/hermes/plugins/disk-cleanup/ | Meeting tools, durable meeting pipelines, event cooldowns, lifecycle cleanup, and safety hooks. |
| H-7 | .resources/hermes/apps/desktop/README.md; .resources/hermes/apps/bootstrap-installer/src/routes/welcome.tsx; .resources/hermes/plugins/hermes-achievements/README.md | No-terminal onboarding, simple installation, community links, and shareable adoption mechanics. |

## Pi evidence

| Code | Source | Relevant evidence |
| --- | --- | --- |
| PI-1 | .resources/pi/packages/coding-agent/docs/extensions.md | Extensions register tools, commands, providers, renderers, UI, and lifecycle interception; project trust and policy extensions are first-class. |
| PI-2 | .resources/pi/packages/coding-agent/docs/packages.md; .resources/pi/packages/coding-agent/docs/skills.md; .resources/pi/packages/coding-agent/docs/prompt-templates.md | Packages combine extensions, skills, prompts, and themes; skills use progressive disclosure; packages support multiple install sources and visual gallery previews. |
| PI-3 | .resources/pi/packages/coding-agent/examples/extensions/questionnaire.ts; .resources/pi/packages/coding-agent/examples/extensions/subagent/README.md; .resources/pi/packages/coding-agent/examples/extensions/preset.ts | Guided setup, multi-agent chains, role presets, and structured interactive UI patterns. |

## OpenClaw evidence

| Code | Source | Relevant evidence |
| --- | --- | --- |
| OC-1 | .resources/openclaw/docs/plugins/manifest.md; .resources/openclaw/docs/plugins/adding-capabilities.md; .resources/openclaw/docs/tools/plugin.md | Manifest-first inspection, capability contracts, channels/providers/tools/hooks, and provider-independent media/search/speech patterns. |
| OC-2 | .resources/openclaw/docs/automation/index.md; .resources/openclaw/docs/automation/standing-orders.md; .resources/openclaw/docs/automation/taskflow.md; .resources/openclaw/docs/automation/cron-jobs.md | Persistent programs, durable multi-step work, schedules, triggers, approvals, waits, verification, reporting, and delivery. |
| OC-3 | .resources/openclaw/docs/channels/index.md; .resources/openclaw/extensions/ | Broad messaging-channel support and bidirectional operational surfaces. |
| OC-4 | .resources/openclaw/docs/clawhub/cli.md; .resources/openclaw/docs/clawhub/publishing.md; .resources/openclaw/docs/plugins/manage-plugins.md | Search/install/update/publish, scans, review/trust state, owner scopes, featured connectors, and marketplace discovery. |
| OC-5 | .resources/openclaw/docs/plugins/plugin-permission-requests.md; .resources/openclaw/docs/tools/creating-skills.md; .resources/openclaw/docs/tools/skills-config.md | Per-action permission requests, dependency gates, environment/config requirements, SecretRefs, and enablement policy. |
| OC-6 | .resources/openclaw/docs/plugins/bundles.md; .resources/openclaw/extensions/migrate-claude/; .resources/openclaw/extensions/migrate-hermes/ | Cross-ecosystem bundle import and migration, including explicit support gaps and trust boundaries. |
| OC-7 | .resources/openclaw/skills/openhue/; .resources/openclaw/skills/sonoscli/; .resources/openclaw/skills/spotify-player/; .resources/openclaw/skills/eightctl/ | Smart-home, media, and personal-life integration patterns. |

## Paperclip evidence

| Code | Source | Relevant evidence |
| --- | --- | --- |
| PC-1 | [Paperclip repository](https://github.com/paperclipai/paperclip); [first company](https://docs.paperclip.ing/guides/getting-started/your-first-company/); [agents](https://docs.paperclip.ing/guides/org/agents/); [adapter overview](https://docs.paperclip.ing/reference/adapters/overview/) | Companies, goals, org charts, agents, authored instructions, Heartbeats, budgets, approvals, adapters, and portable company state. |
| PC-2 | [Issues guide](https://docs.paperclip.ing/guides/day-to-day/issues/); [companies API](https://docs.paperclip.ing/reference/api/companies/) | Status, priority, assignment, comments, documents, handoffs, recovery, watchdogs, and previewed company import/export. |
| PC-3 | [Plugin specification](https://github.com/paperclipai/paperclip/blob/master/doc/plugins/PLUGIN_SPEC.md); [plugin ideas](https://github.com/paperclipai/paperclip/blob/master/doc/plugins/ideas-from-opencode.md) | Proposed capability-gated plugins, tools, events, jobs, webhooks, managed resources, UI slots, and example integrations. |
| PC-4 | [Paperclip Community](https://paperclip.community/); [community resources](https://paperclip.community/resources); [ClipHub](https://cliphub.fyi/); [awesome-paperclip](https://github.com/gsxdsm/awesome-paperclip) | Community plugins, messaging command centers, adapters, company-support tools, curated discovery, and automated registry patterns. |

# AGH Impact Audit

- **Native tools:** no runtime impact. Checked `skills/agh/references/native-tools.md`, `internal/tools/builtin_ids.go`, and `internal/tools/builtin/bundles_resources.go`. Current native bundle management remains list/info/activate/deactivate/status; preview, update, and network settings remain structured CLI/HTTP/UDS fallbacks. This catalog adds no tool ID, descriptor, schema, digest, risk flag, availability diagnostic, capability gate, or fallback.
- **Extensibility and hooks:** no runtime impact. Checked `internal/extension/manifest.go`, `internal/extension/bundle.go`, `internal/bundles/service.go`, and `internal/bundles/resource_projection.go`. The document distinguishes extension-scoped static resources, profile-projected resources, disabled bridge instances, agent-targeted automation, and proposed future composition; it changes no extension, hook, provider, registry, bridge SDK, MCP, bundle, or config behavior.
- **Workspace data isolation:** no runtime datum is added. Checked global/workspace scope and projection in `internal/bundles/model/model.go`, `internal/bundles/service.go`, `internal/bundles/resource.go`, `internal/bundles/resource_projection.go`, and `internal/bundles/resource_store.go`. Every candidate treats global/workspace/session/agent ownership as an implementation requirement, not existing proof.
- **Official AGH skill:** no impact. Checked `skills/agh/SKILL.md`, `skills/agh/references/capabilities-and-bundles.md`, `skills/agh/references/tools-and-skills.md`, and `skills/agh/references/native-tools.md`. No public behavior, tool ID, CLI path, event, capability, bundle/resource contract, or memory/network/task semantic changed.

# Web, docs, config, and QA impact

- `web/`: no route, component, hook, query cache, or user-visible behavior changed.
- `packages/site/`: no public documentation or shipped-support claim changed.
- `config.toml`: no key, default, lifecycle behavior, or example changed.
- `docs/qa/scenarios/`: no user-visible runtime behavior changed, so no scenario is added or reset.

# Catalog totals

- **43 detailed flagship blueprints**
- **160 additional compact candidates**
- **203 total bundle opportunities**
- **15 compact-catalog domain groups**, with the detailed flagships spanning the same portfolio from personal use through engineering and operations

These counts describe this document, not shipped AGH artifacts.
