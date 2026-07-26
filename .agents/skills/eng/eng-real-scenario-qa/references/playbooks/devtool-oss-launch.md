# Helix CLI — v1.0 OSS Release Week

## Company

**Helix CLI** is a developer-tools startup releasing v1.0 of its open-source build orchestrator after eight months of beta. The release is the first stable cut: changelog, release notes, docs landing page, performance benchmarks, demo script, launch tweet thread, and the binary release pipeline must all land in one week. The team has been running performance regressions silently — one is sitting in the bench harness right now.

Stress profile: heavy CLI/release artifacts (Go service stub, Python benchmark, shell release script), a docs surface that must clear copy review, and a real benchmark regression that the team must discover and absorb mid-release.

## Operator persona

**Mateo Rivera — Founder & CEO of Helix CLI**. Voice: technical but not theatrical; references the bench harness, the docs PR queue, and the OSS contributor expectations.

## Workspaces (5)

| id | name | purpose |
|---|---|---|
| ws_release_eng | release-eng | v1.0 binary release, changelog, version table |
| ws_docs_site | docs-site | Docs landing page, API reference, getting-started guide |
| ws_devrel | devrel | Launch announcement, tweet thread, blog post |
| ws_community | community | OSS contributors, GitHub issue triage, support tone |
| ws_bench_ops | bench-ops | Benchmark harness, regression analysis, SLA dashboard |

## Agents (8)

| id | role | workspace |
|---|---|---|
| eng-lead-agent | Engineering Lead | ws_release_eng |
| release-pipeline-agent | Release Pipeline Engineer | ws_release_eng |
| docs-engineer-agent | Docs Engineer | ws_docs_site |
| docs-reviewer-agent | Docs Reviewer | ws_docs_site |
| devrel-lead-agent | DevRel Lead | ws_devrel |
| community-lead-agent | Community Lead | ws_community |
| bench-eng-agent | Benchmark Engineer | ws_bench_ops |
| perf-reviewer-agent | Performance Reviewer | ws_bench_ops |

## Channels (5)

| id | purpose | primary workspace |
|---|---|---|
| release-room | v1.0 cutover decisions, blockers, sign-offs | ws_release_eng |
| docs-review | Docs PR queue, copy review, examples | ws_docs_site |
| launch-comms | Tweet thread, blog post, press timing | ws_devrel |
| community-pulse | OSS issue triage, contributor support | ws_community |
| bench-watch | Bench results, regressions, SLA dashboards | ws_bench_ops |

## Disruption probe seeds

- **bench_regression** at minute 8 — overwrite the bench artifact with a 22% slowdown in cold-start.
- **breaking_change_undocumented** at minute 14 — channel message from a "user" reporting a breaking flag rename not in the changelog.
- **release_pipeline_signing_fail** at minute 22 — task event recording a signing-key failure on the release pipeline dry-run.

---

## Canonical machine-readable spec

```json
{
  "schema_version": 1,
  "playbook_ref": "devtool-oss-launch",
  "company": {
    "name": "Helix CLI",
    "short_name": "Helix",
    "tagline": "Open-source build orchestrator going stable in v1.0 release week",
    "stage": "Open-source MIT, founder-funded",
    "industry": "developer-tools"
  },
  "operator_persona": {
    "role": "Founder/CEO",
    "name": "Mateo Rivera",
    "voice_guidelines": "Technical, low theatrics. Reference the bench harness, the docs PR queue, OSS contributor expectations, and the release pipeline status. Never use evaluation or testing language."
  },
  "kickoff_brief": "Mateo here. v1.0 cutover is in 60 minutes. Open work: release notes and changelog (release-eng), docs landing TSX with the upgrade guide (docs-site), benchmark regression check on the cold-start path (bench-ops), launch tweet thread component for the site (devrel), demo shell script that walks new users through helix init -> build -> run, and a community macro pack so the contributor inbox does not melt at announcement. The bench harness has been quiet for two days; treat any signal there as load-bearing. Work in your workspaces, coordinate in release-room and your domain channels, and request reviews where the OSS audience will see the artifact. We do not ship a stable v1.0 with a known cold-start regression unannounced.",
  "workspaces": [
    { "id": "ws_release_eng", "name": "release-eng", "purpose": "v1.0 binary release, changelog, version table" },
    { "id": "ws_docs_site", "name": "docs-site", "purpose": "Docs landing page, API reference, getting-started guide" },
    { "id": "ws_devrel", "name": "devrel", "purpose": "Launch announcement, tweet thread, blog post" },
    { "id": "ws_community", "name": "community", "purpose": "OSS contributors, GitHub issue triage, support tone", "knowledge_files": ["workspace/contributor-support-tone.md"] },
    { "id": "ws_bench_ops", "name": "bench-ops", "purpose": "Benchmark harness, regression analysis, SLA dashboard", "knowledge_files": ["workspace/bench-harness-status.md"] }
  ],
  "agents": [
    {
      "id": "eng-lead-agent",
      "role": "Engineering Lead",
      "persona": "Engineering Lead. Owns release sign-off and cross-team unblocking.",
      "system_prompt": "You are the Engineering Lead at Helix CLI. Hold the v1.0 cutover checklist in release-room, unblock owners, and decide whether the release ships, ships with a known issue, or holds. Do not implement work yourself — coordinate it. Read the release-policy and the current bench status before approving cutover.",
      "workspace": "ws_release_eng"
    },
    {
      "id": "release-pipeline-agent",
      "role": "Release Pipeline Engineer",
      "persona": "Release Pipeline Engineer. Owns the binary release shell script and signing pipeline.",
      "system_prompt": "You are the Release Pipeline Engineer at Helix CLI. Author the release shell script (build, sign, publish) and the canary-promotion control service stub for the release rollout endpoint. Surface signing failures to release-room immediately; never silently retry.",
      "workspace": "ws_release_eng"
    },
    {
      "id": "docs-engineer-agent",
      "role": "Docs Engineer",
      "persona": "Docs Engineer. Owns the docs landing page and the upgrade guide.",
      "system_prompt": "You are the Docs Engineer at Helix CLI. Ship the docs landing TSX page, the v1.0 upgrade guide runbook, and the changelog spec. Route every docs change through docs-reviewer on docs-review channel before it lands.",
      "workspace": "ws_docs_site"
    },
    {
      "id": "docs-reviewer-agent",
      "role": "Docs Reviewer",
      "persona": "Docs Reviewer. Reviews docs for accuracy, examples, and tone.",
      "system_prompt": "You are the Docs Reviewer at Helix CLI. Reject docs PRs that introduce undocumented breaking changes, silently rename flags, or claim performance numbers the bench harness does not currently support. Approve only when examples run.",
      "workspace": "ws_docs_site"
    },
    {
      "id": "devrel-lead-agent",
      "role": "DevRel Lead",
      "persona": "DevRel Lead. Owns the launch tweet thread component and the announcement timing.",
      "system_prompt": "You are the DevRel Lead at Helix CLI. Build the launch tweet thread as a TSX component for the site (one component, one thread). Hold publication until the docs landing is live and the engineering lead clears the bench status. Coordinate with the community lead on contributor messaging tone.",
      "workspace": "ws_devrel"
    },
    {
      "id": "community-lead-agent",
      "role": "Community Lead",
      "persona": "Community Lead. Owns the contributor inbox and the launch macro pack.",
      "system_prompt": "You are the Community Lead at Helix CLI. Author the contributor support macro pack runbook. Triage new GitHub issues in community-pulse with the approved support tone (warm, technical, never defensive).",
      "workspace": "ws_community"
    },
    {
      "id": "bench-eng-agent",
      "role": "Benchmark Engineer",
      "persona": "Benchmark Engineer. Owns the bench harness and the cold-start benchmark.",
      "system_prompt": "You are the Benchmark Engineer at Helix CLI. Run and analyze the cold-start benchmark. If you detect a regression, post the delta with evidence to bench-watch and block release-room until the perf reviewer signs off. Author the benchmark Python script and the demo shell script.",
      "workspace": "ws_bench_ops"
    },
    {
      "id": "perf-reviewer-agent",
      "role": "Performance Reviewer",
      "persona": "Performance Reviewer. Reviews bench deltas and decides ship/hold.",
      "system_prompt": "You are the Performance Reviewer at Helix CLI. Read every bench delta the bench-eng-agent posts, classify (noise / regression / optimization), and write the verdict to bench-watch. Hold the v1.0 release if cold-start regression exceeds 15%.",
      "workspace": "ws_bench_ops"
    }
  ],
  "channels": [
    { "id": "release-room", "purpose": "v1.0 cutover decisions, blockers, sign-offs", "primary_workspace": "ws_release_eng" },
    { "id": "docs-review", "purpose": "Docs PR queue, copy review, examples", "primary_workspace": "ws_docs_site" },
    { "id": "launch-comms", "purpose": "Tweet thread, blog post, press timing", "primary_workspace": "ws_devrel" },
    { "id": "community-pulse", "purpose": "OSS issue triage, contributor support", "primary_workspace": "ws_community" },
    { "id": "bench-watch", "purpose": "Bench results, regressions, SLA dashboards", "primary_workspace": "ws_bench_ops" }
  ],
  "knowledge_files": [
    {
      "path": "global/operator-style.md",
      "content": "# Operator narrative style\n\nTechnical, low theatrics. State what shipped, what is blocked, what is next. Reference the bench harness, the docs PR queue, OSS contributor expectations, and the release pipeline. Never use evaluation or testing framing.\n"
    },
    {
      "path": "global/release-policy.md",
      "content": "# v1.0 release policy\n\n- Ship only with: signed binary, accurate changelog, docs landing live, bench cold-start within 15% of v0.9.x, contributor macro pack live.\n- Ship with known issue: documented regression, runbook for rollback, public note in announcement.\n- Hold: bench regression > 15%, signing failure, undocumented breaking change.\n"
    },
    {
      "path": "global/changelog-style.md",
      "content": "# Changelog style\n\n- Group by Added / Changed / Deprecated / Removed / Fixed / Performance.\n- Include flag renames in BOTH Changed and Deprecated.\n- Cite the issue or PR id for every entry.\n- v1.0 entry must include explicit \"Stable API\" note and version-table compatibility section.\n"
    },
    {
      "path": "workspace/contributor-support-tone.md",
      "content": "# Contributor support tone\n\nWarm, technical, never defensive. Acknowledge first, restate the issue, propose the next concrete step or known workaround. Do not over-promise on roadmap timing.\n"
    },
    {
      "path": "workspace/bench-harness-status.md",
      "content": "# Bench harness status\n\nLast clean run: two days ago. Cold-start path under observation; no regressions reported. Awaiting fresh run before release.\n"
    }
  ],
  "open_tasks": [
    {
      "title": "Release shell script (build, sign, publish)",
      "description": "POSIX shell script driving the v1.0 binary release: build, sign, publish to GitHub releases. Surface signing failures hard.",
      "owner_agent": "release-pipeline-agent",
      "deliverable_type": "shell_script",
      "deliverable_path_hint": "ws_release_eng/scripts/release-v1.sh",
      "review_required_by": "eng-lead-agent",
      "channel": "release-room"
    },
    {
      "title": "Canary promotion control stub",
      "description": "Go service stub exposing /promote and /pause for the release rollout; gate behind bench status.",
      "owner_agent": "release-pipeline-agent",
      "deliverable_type": "go_service_stub",
      "deliverable_path_hint": "ws_release_eng/services/release-control/main.go",
      "review_required_by": "eng-lead-agent",
      "channel": "release-room"
    },
    {
      "title": "Docs landing TSX page",
      "description": "Docs landing for v1.0; references upgrade guide and changelog; routes to /v1.",
      "owner_agent": "docs-engineer-agent",
      "deliverable_type": "tsx_page",
      "deliverable_path_hint": "ws_docs_site/landing/v1-release.tsx",
      "review_required_by": "docs-reviewer-agent",
      "channel": "docs-review"
    },
    {
      "title": "Upgrade guide runbook",
      "description": "Step-by-step v0.9 -> v1.0 upgrade runbook; covers flag renames and config changes.",
      "owner_agent": "docs-engineer-agent",
      "deliverable_type": "runbook_md",
      "deliverable_path_hint": "ws_docs_site/runbooks/upgrade-v1.md",
      "review_required_by": "docs-reviewer-agent",
      "channel": "docs-review"
    },
    {
      "title": "v1.0 changelog spec",
      "description": "Full changelog conforming to changelog-style.md; cite all PRs.",
      "owner_agent": "docs-engineer-agent",
      "deliverable_type": "spec_md",
      "deliverable_path_hint": "ws_release_eng/changelog/v1.md",
      "review_required_by": "docs-reviewer-agent",
      "channel": "release-room"
    },
    {
      "title": "Cold-start benchmark Python script",
      "description": "Python script that runs the helix cold-start benchmark and writes JSON results.",
      "owner_agent": "bench-eng-agent",
      "deliverable_type": "python_script",
      "deliverable_path_hint": "ws_bench_ops/bench/cold-start.py",
      "review_required_by": "perf-reviewer-agent",
      "channel": "bench-watch"
    },
    {
      "title": "Bench result analyzer",
      "description": "Python script comparing two cold-start runs and emitting the percentage delta.",
      "owner_agent": "bench-eng-agent",
      "deliverable_type": "python_script",
      "deliverable_path_hint": "ws_bench_ops/bench/analyze-delta.py",
      "review_required_by": "perf-reviewer-agent",
      "channel": "bench-watch"
    },
    {
      "title": "Bench harness smoke tests",
      "description": "TS test suite covering the bench harness JSON contract.",
      "owner_agent": "bench-eng-agent",
      "deliverable_type": "ts_test",
      "deliverable_path_hint": "ws_bench_ops/bench/contract.test.ts",
      "review_required_by": "perf-reviewer-agent",
      "channel": "bench-watch"
    },
    {
      "title": "Launch tweet thread component",
      "description": "TSX component rendering the v1.0 launch thread on the site; one component, multiple cards.",
      "owner_agent": "devrel-lead-agent",
      "deliverable_type": "tsx_component",
      "deliverable_path_hint": "ws_devrel/launch/tweet-thread.tsx",
      "review_required_by": "eng-lead-agent",
      "channel": "launch-comms"
    },
    {
      "title": "Contributor support macro pack",
      "description": "Macro pack for the v1.0 launch wave; consumed by community-lead in community-pulse.",
      "owner_agent": "community-lead-agent",
      "deliverable_type": "runbook_md",
      "deliverable_path_hint": "ws_community/runbooks/launch-macro-pack.md",
      "review_required_by": "devrel-lead-agent",
      "channel": "community-pulse"
    },
    {
      "title": "Bench regression decision",
      "description": "Live decision recorded in release-room when the bench regression disruption fires; ship / ship-with-known-issue / hold.",
      "owner_agent": "eng-lead-agent",
      "deliverable_type": "spec_md",
      "deliverable_path_hint": "ws_release_eng/decisions/bench-regression-decision.md",
      "review_required_by": "perf-reviewer-agent",
      "channel": "release-room"
    }
  ],
  "required_deliverables": {
    "go_service_stub": 1,
    "python_script": 2,
    "shell_script": 1,
    "tsx_page": 1,
    "tsx_component": 1,
    "ts_test": 1,
    "runbook_md": 1,
    "spec_md": 1
  },
  "required_collaboration": {
    "peer_messages_min": 12,
    "review_cycles_min": 3,
    "disagreements_resolved_min": 1,
    "channels_active_min": 4
  },
  "disruption_probe_seeds": [
    {
      "type": "bench_regression",
      "seed_at_minute": 8,
      "delivery": "knowledge_file",
      "expected_recovery": "Bench engineer detects the 22% delta and posts to bench-watch within 5 minutes; perf reviewer issues a verdict; engineering lead writes the bench-regression-decision before announcing release."
    },
    {
      "type": "breaking_change_undocumented",
      "seed_at_minute": 14,
      "delivery": "channel_message",
      "expected_recovery": "Docs engineer adds the flag rename to changelog and upgrade guide within 8 minutes; docs reviewer approves the diff."
    },
    {
      "type": "release_pipeline_signing_fail",
      "seed_at_minute": 22,
      "delivery": "task_event",
      "expected_recovery": "Release pipeline engineer surfaces failure to release-room immediately; engineering lead decides hold or ship-with-known-issue and records the decision."
    }
  ]
}
```
