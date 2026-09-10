# Skills catalog delivery and input-only roles

Issue: [#604](https://github.com/compozy/compozy/issues/604). Baseline:
`ed7f2d7adc2d7ec38071677d28a2d6e87a019c28`.

## Cause and correction

The skills augmenter remembered rendered catalogs during preparation. Startup used different
tags and instructions and never entered that cache. Consequently the first request repeated the
catalog, and preparing a failed request could establish an unseen catalog as current.

The augmenter now registers the complete filtered current catalog, its equivalent startup
rendering, and the existing unchanged marker with one dispatch attempt. The ACP boundary compacts
the registered section only when its equivalent startup is included by the current delivery mode
or the process has confirmed the same current catalog. A successful, uncanceled ACP response
records that context. Failure/cancellation invalidates uncertain context. Recovery retains full
input, and a fresh process owns a fresh context record. Duplicate or omitted registered content
is not treated as a confirmed section. No process-wide catalog LRU remains.

The existing input-only role predicate now also excludes startup and live skills and situation
context for memory extraction, automatic titles, and checkpoint summaries. Runtime identity,
authored instructions, ordinary workers, unknown roles, and skill authorization retain their
existing behavior.

## Measurement method

The owning harness integration scenario uses the production assembler, registry, role resolver,
session manager, real SQLite, and the existing ACP mock subprocess. It seeds 23 local fixture
skills alongside bundled skills, sends first/unchanged/changed turns, and creates the three
input-only role sessions. Receiver diagnostics measure actual UTF-8 prompt bytes. Role fixtures
use a common instruction and input to isolate harness overhead; these are not production
extractor transcript sizes, tokenizer counts, provider billing, or model-compliance measurements.

The baseline uses a Go overlay containing the three original daemon source files from the base
commit; it does not modify another checkout. The same receiver scenario detects the original
first-turn and role failures. Both runs used the same fixture and input; generated timestamps
and session metadata can vary by a few bytes.

| Received prompt | Before bytes | After bytes | Before skill entries | After skill entries |
| --- | ---: | ---: | ---: | ---: |
| Ordinary first turn | 22,751 | 19,947 | 48 | 24 |
| Ordinary unchanged turn | 1,656 | 1,655 | 0 | 0 |
| Ordinary changed turn (one addition) | 4,523 | 4,523 | 25 | 25 |
| Memory extractor | 11,192 | 1,452 | 50 | 0 |
| Auto title | 11,186 | 1,452 | 50 | 0 |
| Checkpoint summary | 11,097 | 1,450 | 50 | 0 |

The observed first-turn reduction is 2,804 bytes (12.3% of this complete prompt). Input-only
reductions are 9,647–9,740 bytes (86.9–87.0%). These differences are calculated from measured
receiver payloads, not projected token or billing savings. Ordinary situation context remains
present. All input-only payloads omit both catalogs and situation context. The unchanged marker
and changed full catalog remain observable after the correction.

The corrected measurement scenario passes with the race detector. Running the same scenario
against the baseline overlay fails specifically on first-turn duplication and all three role
context assertions, reproducing the reported defects.

## Verification ownership

- Registry visibility, provider filtering, activation, and exact-source invocation remain owned
  by the existing daemon skill suites.
- `TestPromptCompactsDeliveredSections` owns startup/native delivery, unchanged/changed turns,
  failed preparation, provider error, cancellation, removed context, and fresh-process behavior.
- Harness resolver and integration suites own role gating and persisted-role resume.
- The extension-agent E2E scenario owns startup catalog availability plus explicit invocation
  through the real daemon, HTTP, hosted MCP, and ACP subprocess.
- The test-shape checker passes the changed daemon suites. Its four ACP findings are unchanged
  baseline cases outside this change; new cases use the canonical subtest form.

## Cross-surface impact

Audit under `docs/_memory/change-impact.md`:

| Surface | Impact |
| --- | --- |
| Native tools | No IDs, schemas, permissions, approval gates, or dispatch changes. Catalog prompt compaction is not authorization. |
| Extensions, hooks, configuration | Current registry resolution and provider filtering remain live. No new configuration or extension contract. Explicit skill activation retains its existing path. |
| Workspace data | Confirmed context belongs to one ACP process; only hashes persist in its memory. Recovery requests retain their own full section copies. No database shape or stored-user-state change. |
| Official skill | Existing unchanged-marker and latest-full-context guidance remains applicable; native skill reads and resources are unchanged. |
| Web and docs | No UI or public wire changes. The bundled-skills guide and ET-049 explain the delivery and role behavior. |
| Compatibility | Internal prompt preparation changes only. No migration, compatibility alias, or user action is required. |

Executed validation:

- Focused race suites for ACP section delivery, daemon catalog resolution, and role policy: pass.
- `TestHarnessContextIntegrationScopesToolGuidanceForInternalCallers` with `-race -tags=integration`:
  pass, including persisted-role resume with skills enabled and disabled.
- `TestHarnessContextIntegrationMeasuresDeliveredSkillCatalogs` with `-race -tags=integration`:
  pass; measurements above come from its ACP receiver diagnostics.
- `TestDaemonE2EExtensionPublishedAgentSessionCommandsAndPrompt` with `-race -tags=integration`:
  pass through the real daemon, HTTP, hosted MCP, and ACP fixture subprocess.
- `make gate`: final result recorded in the PR validation evidence.

Runtime checks use isolated temporary homes and owned subprocess cleanup. The user's daemon is
not restarted. Linux race parity remains subject to current-head CI; no live inference-provider
or Windows runtime validation is claimed.
