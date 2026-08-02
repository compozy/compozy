# Compozy Ecosystem Opportunity Map

- **Status:** research reference and product opportunity map, not a committed roadmap
- **Research snapshot:** 2026-07-17
- **Companion catalog:** [Atomic extensions and integrations](extension-opportunities.md)

## Purpose

This document asks which reusable services, providers, and complete operating kits Compozy should
make available through extensions. Every candidate is an opportunity, not a claim that an
integration exists or that an upstream API, commercial model, or regional deployment is suitable.

> Extensions make services reachable and ship complete kits. Compozy keeps their resources running.

## Product direction

The Marketplace has three artifact kinds: extensions, skills, and MCP servers. An extension is the
single installable, versioned, trust-bearing unit for a complete kit. It can ship static resources,
run as a capability-gated subprocess, or do both.

| User question | Marketplace artifact | Typical author |
| --- | --- | --- |
| Can Compozy reach the service I use? | Atomic extension or MCP server | Integration developer or vendor |
| Can Compozy run this recurring job? | Extension with an outcome-focused kit | Domain expert with an extension developer |
| What should someone like me install? | Editorial collection of extensions, skills, and MCP servers | Maintainer or curator |

Collections remain editorial. They do not create a hidden composition or activation product.

## Extension responsibility

An extension owns one lifecycle. Installation verifies and records the package but leaves static
resources inert. Enable publishes the instance-owned kit; disable removes those resources; update
checks trust, secret bindings, and Network consent before replacing the active version.

| Concern | Current Compozy shape |
| --- | --- |
| Lifecycle | Search, install, inspect, enable, disable, update, remove |
| Provenance | Source, trust tier, checksums, Marketplace metadata |
| Static kit | Skills, Loops, agents and sidecars, automation, layouts, MCP sidecars |
| Runtime services | Tool, memory, model, and Loop watch-source provide surfaces |
| Dynamic resources | Host API publication through declared permissions |
| Secrets | Instance-scoped bindings from declared environment keys to Vault references |
| Operability | Inventory, preview, structured status, logs, and deterministic errors |
| Management | CLI, HTTP/UDS, web, and read-only native tools where appropriate |

An atomic integration should own one reusable responsibility: a service connector, provider,
event source, policy hook, document/media processor, or infrastructure adapter. An outcome-focused
extension can combine its own agents, automation, layouts, skills, and sidecars, but must not claim
to install or coordinate separately owned extensions.

## Opportunity taxonomy

### Atomic extension

One versioned integration or runtime provider, such as Google Workspace, HubSpot, document
extraction, browser automation, a memory backend, or a webhook source.

### Extension family

Separate implementations for the same user need, such as Asana, ClickUp, Jira, Linear, and Monday
for work management. Each package keeps its own lifecycle and trust boundary.

### Outcome extension

An extension kit for a bounded job with a visible completion condition, such as converting a
meeting into assigned actions or fixing a Linear issue through a reviewed pull request.

### Persona or vertical extension

A broader kit for a role or domain, with explicit terminology, approvals, schedules, data policy,
and escalation paths. These packages are still ordinary extensions; the label is editorial.

## Prioritization

Evaluate candidates on audience reach, frequency, visible first value, reuse across kits, safe
starting posture, integration viability, setup burden, maintenance cost, and differentiation.

| Risk | Examples | Default posture |
| --- | --- | --- |
| **R0 — read-only** | Search, summarize, report, monitor | Allow after explicit connection and scope selection |
| **R1 — reversible private write** | Draft, create internal task, add private note | Preview or undo path; configurable approval |
| **R2 — external or public write** | Send email, publish content, contact lead | Human approval by default |
| **R3 — high impact** | Payment, deletion, legal filing, access change | Mandatory narrow approval; some actions remain unsupported |

## Recommended first portfolio

High-leverage service foundations include Google Workspace, Microsoft 365, Notion, HubSpot,
Intercom or Zendesk, meeting intelligence, document intelligence, web research, browser automation,
webhook ingress/egress, Home Assistant, Shopify, Stripe, and a design-asset provider.

Outcome-focused validation candidates include Meeting to Action, Personal Chief of Staff, Local
Business Front Desk, Sales Call Prep and Follow-up, Support Triage to Knowledge, Campaign in a Box,
Content Repurposing Engine, Ecommerce Daily Operator, Executive Business Review, Product Launch
Command Center, Community Manager, and Fix a Linear Issue.

Start every candidate read-only or draft-first where possible. External writes, money movement,
deletion, regulated decisions, and access changes require explicit operator control.

## Authoring and lifecycle bar

Before publishing an extension opportunity, answer:

1. What single service, provider, or outcome does the extension own?
2. Which resources ship in its static kit, and what becomes live on enable?
3. Which environment keys are required, and how are Vault bindings checked without exposing refs?
4. Which Network requirement digest needs confirmation on enable or update?
5. What does inventory show, and what does preview predict before mutation?
6. Which CLI, HTTP/UDS, web, and native-tool surfaces let an agent inspect or manage it?
7. What are the global/workspace/session/agent ownership boundaries?
8. What proves first value, safe failure, cleanup, and disable behavior?

## Platform boundaries

Cross-extension dependency resolution, profile-scoped composition, generic provider contracts, a
no-code API compiler, a generic OAuth broker, and a second package/activation abstraction are not
part of the core roadmap. Connectors should converge on MCP servers or subprocess extensions that
ship real code and use the existing Vault and lifecycle contracts.

## Source-of-truth paths

- `internal/extension/`
- `internal/resources/`
- `internal/api/contract/`
- `internal/tools/`
- `packages/site/content/docs/extensions/`
- `skills/compozy/references/extensions.md`

## Compozy Impact Audit

- **Native tools:** editorial only; checked extension inventory/preview and resource discovery guidance. No descriptor or ToolID changes originate here.
- **Extensibility and hooks:** editorial only; describes the current extension resource, hook, registry, MCP-sidecar, and config lifecycle without changing runtime behavior.
- **Workspace data isolation:** editorial only; adds no runtime datum and requires each candidate to document its ownership scope.
- **Official Compozy skill:** aligned with `skills/compozy/references/extensions.md`; no additional skill change originates here.
