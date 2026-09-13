#!/usr/bin/env python3
"""Prepare a deep review from a small decision file; write only under --out.

First invocation creates the manifest, compact knowledge queue and decision
skeleton. Re-run with --decisions after assessing sources: references of
applied skills become pending, then complete decisions compile rules, context
and a default plan into jobs. Exit 2 means semantic decisions remain pending;
exit 1 is an invalid input, exit 3 is a drifted source; exit 0 is ready/empty.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.dont_write_bytecode = True
from _common import check_freeze, manifest_selected, read_json, rel, repo_root, write_json
from build_jobs import (
    DEFAULT_MAX_COHORT_FILES, DEFAULT_MAX_POLISH_FILES, MAX_COHORT_CHANGED_LINES,
    MAX_POLISH_CHANGED_LINES, automatic_plan, materialize_jobs, positive_int,
)
from build_knowledge import discover, knowledge_markdown


def linter_candidates(repo: Path, selected: list[str]) -> list[dict]:
    """Suggest repository-owned lanes; commands are never executed implicitly."""
    found = []
    makefile = repo / "Makefile"
    if makefile.is_file() and re.search(r"^lint\s*:", makefile.read_text(errors="replace"), re.M):
        found.append({"name": "repo-lint", "command": "make lint", "scope": ["**/*"]})
    package = repo / "package.json"
    if package.is_file() and any(Path(p).suffix in {".js", ".jsx", ".ts", ".tsx", ".vue", ".svelte"} for p in selected):
        data = read_json(package)
        manager = str(data.get("packageManager", "")).split("@")[0]
        if not manager:
            manager = next((m for lock, m in [("bun.lock", "bun"), ("bun.lockb", "bun"), ("pnpm-lock.yaml", "pnpm"), ("yarn.lock", "yarn")] if (repo / lock).exists()), "npm")
        for name in ("lint", "typecheck"):
            if name in data.get("scripts", {}) and not (name == "lint" and found):
                found.append({"name": name, "command": f"{manager} run {name}", "scope": ["**/*.js", "**/*.jsx", "**/*.ts", "**/*.tsx", "**/*.vue", "**/*.svelte"]})
    return found


def spec_artifacts(repo: Path, requested: str | None, decisions: dict) -> list[dict]:
    artifacts = []
    if requested:
        path = (repo / requested).resolve()
        if not path.exists():
            raise ValueError(f"spec path does not exist: {requested}")
        if path.is_file():
            artifacts.append({"path": rel(path, repo), "role": "requested conformance baseline"})
        else:
            # Conservative: nested Markdown contracts are cheap to inventory; reviewer routes canonical dependencies.
            artifacts.extend({"path": rel(file, repo), "role": "specification document"} for file in sorted(path.rglob("*.md")))
    for artifact in decisions.get("spec_artifacts", []):
        if not isinstance(artifact, dict) or not artifact.get("path") or not artifact.get("role"):
            raise ValueError("spec_artifacts entries need path and role")
        if not (repo / artifact["path"]).is_file():
            raise ValueError(f"missing spec artifact: {artifact['path']}")
        artifacts.append(artifact)
    if requested and not artifacts:
        raise ValueError(f"spec directory has no Markdown contracts; name canonical files in decisions.spec_artifacts: {requested}")
    return [{**item, "sha256": hashlib.sha256((repo / item["path"]).read_bytes()).hexdigest()}
            for item in {entry["path"]: entry for entry in artifacts}.values()]


def compile_context(repo: Path, out: Path, manifest: dict, registry: dict, decisions: dict, requested_spec: str | None) -> dict:
    candidates = linter_candidates(repo, list(manifest_selected(manifest)))
    linters = decisions.get("linters", [])
    if not isinstance(linters, list):
        raise ValueError("decisions.linters must be an array")
    for lane in linters:
        if not isinstance(lane, dict) or not all(lane.get(k) for k in ("name", "command", "result")):
            raise ValueError("each linter needs name, command, status and result evidence/reason")
        if lane.get("status") not in {"ran", "reused", "unavailable"}:
            raise ValueError(f"linter {lane['name']}: status must be ran/reused/unavailable")
        if lane["status"] != "unavailable" and lane.get("input_snapshot") != manifest.get("worktree_snapshot"):
            raise ValueError(f"linter {lane['name']}: input_snapshot must match the frozen review inputs")
    missing = {lane["name"] for lane in candidates} - {lane["name"] for lane in linters}
    if missing:
        raise ValueError(f"record ran/reused/unavailable results for detected linters: {sorted(missing)}")
    context = {"intent": str(decisions.get("intent") or f"Review {manifest['target']} against its stated change intent."),
               "mode": decisions.get("mode", "agent-fallback"),
               "linters": linters, "spec_artifacts": spec_artifacts(repo, requested_spec, decisions)}
    write_json(out / "review-context.json", context)
    lines = [f"# Context Pack — {manifest['target']}", "", "## Intent", context["intent"], "", "## Rubric"]
    for source in registry["sources"]:
        if source["status"] == "applied":
            count = sum(r["source"] == source["source"] for r in registry["rules"])
            lines.append(f"- `{source['source']}`: {count} rules")
    ignored = sum(s["status"] == "not-applicable" for s in registry["sources"])
    lines.extend([f"- {ignored} other sources classified not-applicable in rules.json.", "", "## Linters"])
    lines.extend(f"- {lane['name']}: {lane['status']} — `{lane['command']}` — {lane['result']}" for lane in linters)
    if not linters:
        lines.append("No repository-owned linter lanes detected; overlap suppression unavailable.")
    if context["spec_artifacts"]:
        lines.extend(["", "## Spec contract"])
        lines.extend(f"- `{a['path']}` — {a['role']}" for a in context["spec_artifacts"])
    (out / "context-pack.md").write_text("\n".join(lines) + "\n")
    return context


def render_walkthrough(repo: Path, out: Path) -> None:
    """Generate mechanical report sections and use reviewers' semantic summaries."""
    manifest, plan = read_json(out / "manifest.json"), read_json(out / "plan.json")
    context, registry = read_json(out / "review-context.json"), read_json(out / "rules.json")
    summaries, diagrams = {}, []
    if (out / "jobs.json").exists():
        for job in read_json(out / "jobs.json")["jobs"]:
            if job["kind"] != "cohort" or not (repo / job["output"]).exists():
                continue
            payload = read_json(repo / job["output"])
            if payload.get("summary"):
                summaries[job["label"]] = " ".join(str(payload["summary"]).replace("|", "\\|").split())
            if payload.get("sequence_diagram"):
                diagrams.append(str(payload["sequence_diagram"]))
    lines = ["<!-- deep-review:walkthrough -->", "<!-- deep-review:generated -->", "## Walkthrough", "", context["intent"], "", "## Changes", "", "| Cohort / File(s) | Summary |", "| --- | --- |"]
    selected = manifest_selected(manifest)
    for cohort in plan["cohorts"]:
        paths = ", ".join(f"`{path}`" for path in cohort["files"])
        summary = summaries.get(f"cohort-{cohort['id'].lower()}") or cohort.get("summary") or f"{len(cohort['files'])} selected files; {sum(sum(int(h['lines']) for h in cohort.get('hunk_scope', {}).get(p, selected[p]['hunks'])) for p in cohort['files'])} changed hunk lines."
        lines.append(f"| {cohort['name']}<br>{paths} | {summary} |")
    if diagrams:
        lines.extend(["", "## Sequence Diagram(s)"])
        lines.extend(f"\n```mermaid\n{diagram}\n```" for diagram in dict.fromkeys(diagrams))
    high = any(c["risk"] == "high" for c in plan["cohorts"])
    effort, label, minutes = (5, "Critical", 120) if high else (4, "Complex", 60) if len(plan["cohorts"]) > 2 else (3, "Moderate", 25) if len(selected) > 5 else (2, "Simple", 12)
    counts = manifest["counts"]
    lines.extend(["", "## Estimated code review effort", "", f"🎯 {effort} ({label}) | ⏱️ ~{minutes} minutes", "", "## Review details", "",
                  f"- **Scope**: {manifest['base'][:8]} → {manifest['head'][:8]} ({manifest.get('mode', 'full')} round {manifest['round']})",
                  f"- **Files**: {counts['selected']} selected · {counts['ignored']} ignored · {counts['skipped']} skipped",
                  f"- **Posture**: assertive · **Mode**: {context.get('mode', 'native')}",
                  f"- **Rubric**: {', '.join(s['source'] for s in registry['sources'] if s['status'] == 'applied')}",
                  f"- **Linters**: {', '.join(l['name'] + ': ' + l['status'] for l in context['linters']) or 'unavailable (none detected)'}"])
    (out / "walkthrough.md").write_text("\n".join(lines) + "\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--decisions")
    parser.add_argument("--spec")
    parser.add_argument("--refresh", action="store_true", help="explicitly rebuild the manifest after scope/source changes")
    parser.add_argument("--max-cohort-files", type=positive_int, help="defect files per job (default 200); no total PR cap")
    parser.add_argument("--max-cohort-lines", type=positive_int, help="defect changed lines per job (default 15000)")
    parser.add_argument("--max-polish-files", type=positive_int, help="polish files per job (default 200)")
    parser.add_argument("--max-polish-lines", type=positive_int, help="polish changed lines per job (default 15000)")
    parser.add_argument("--max-context-lines", type=positive_int, help="optional full-file context budget per cohort; no files are dropped")
    args, manifest_args = parser.parse_known_args()
    repo, out = repo_root(), Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    started = datetime.now(timezone.utc).isoformat()
    try:
        request = read_json(out / "prepare-request.json") if (out / "prepare-request.json").exists() else {}
        limits = {}
        for name, value, default in (
            ("cohort_files", args.max_cohort_files, DEFAULT_MAX_COHORT_FILES),
            ("cohort_changed_lines", args.max_cohort_lines, MAX_COHORT_CHANGED_LINES),
            ("polish_files", args.max_polish_files, DEFAULT_MAX_POLISH_FILES),
            ("polish_changed_lines", args.max_polish_lines, MAX_POLISH_CHANGED_LINES),
            ("context_lines", args.max_context_lines, None),
        ):
            limits[name] = value if value is not None else request.get("limits", {}).get(name, default)
            if limits[name] is None and name == "context_lines":
                continue
            if type(limits[name]) is not int or limits[name] < 1:
                raise ValueError(f"{name} must be a positive integer")
        if args.refresh and not manifest_args:
            manifest_args = request.get("manifest_args", [])
        requested_spec = args.spec or request.get("spec")
        if not (out / "manifest.json").exists() or args.refresh:
            refresh = ["--new-round"] if args.refresh else []
            completed = subprocess.run([sys.executable, str(Path(__file__).with_name("build_manifest.py")), "--out", str(out), *manifest_args, *refresh], cwd=repo)
            if completed.returncode:
                return completed.returncode
            write_json(out / "prepare-request.json", {"manifest_args": manifest_args, "spec": requested_spec, "limits": limits})
        request = read_json(out / "prepare-request.json") if (out / "prepare-request.json").exists() else {}
        if manifest_args and manifest_args != request.get("manifest_args"):
            raise ValueError("scope arguments differ from the existing manifest; use --refresh to start the intended round")
        write_json(out / "prepare-request.json", {**request, "spec": requested_spec, "limits": limits})
        # Preparation may rebuild derived artifacts after a corrected decision.
        # Discovery validates source evidence; the global gate pins compiled inputs.
        drift = check_freeze(repo, out, "prepare", evidence=False)
        if drift:
            print(drift[0], file=sys.stderr)
            return 3
        manifest = read_json(out / "manifest.json")
        if not manifest_selected(manifest):
            print(f"nothing reviewable: {manifest['counts']}")
            return 0
        decision_path = Path(args.decisions).resolve() if args.decisions else out / "decisions.json"
        if args.decisions and not decision_path.exists():
            raise ValueError(f"decision file does not exist: {decision_path}")
        decisions = read_json(decision_path) if decision_path.exists() else None
        knowledge, registry = discover(repo, manifest, decisions)
        write_json(out / "knowledge.json", knowledge)
        write_json(out / "rules.template.json", registry)
        (out / "knowledge.md").write_text(knowledge_markdown(knowledge, registry))
        retained = {k: v for k, v in (decisions or {}).items() if k != "groups"}
        template = {**retained, **registry, "intent": (decisions or {}).get("intent", ""),
                    "linters": (decisions or {}).get("linters", [{**l, "status": "pending", "result": "", "input_snapshot": manifest["worktree_snapshot"]} for l in linter_candidates(repo, list(manifest_selected(manifest)))]),
                    "sweeps": (decisions or {}).get("sweeps", []), "spec_artifacts": (decisions or {}).get("spec_artifacts", [])}
        write_json(out / "decisions.template.json", template)
        pending = [s for s in registry["sources"] if s["status"] == "pending"]
        if pending:
            print(f"{len(pending)} source decisions pending. Read {out / 'knowledge.md'}; edit {out / 'decisions.template.json'} as {decision_path}; rerun with --decisions. No review jobs dispatched.")
            return 2
        decisions = decisions or template
        write_json(out / "rules.json", registry)
        compile_context(repo, out, manifest, registry, decisions, requested_spec)
        plan = decisions.get("plan") or automatic_plan(manifest_selected(manifest), limits["cohort_files"],
                                                       repo, limits["context_lines"], limits["cohort_changed_lines"])
        plan["limits"] = {**plan.get("limits", {}), **limits}
        plan["sweeps"] = decisions.get("sweeps", plan.get("sweeps", []))
        write_json(out / "plan.json", plan)
        summary = materialize_jobs(repo, out, limits["cohort_files"], limits["cohort_changed_lines"],
                                   limits["polish_files"], limits["polish_changed_lines"])
        write_json(out / "prepare-status.json", {"started_at": started, "finished_at": datetime.now(timezone.utc).isoformat(), "status": "ready", **summary})
        print(f"ready: {summary}; {out / 'jobs.json'}. Walkthrough is generated at report time.")
        return 0
    except (RuntimeError, OSError, ValueError, KeyError, TypeError) as error:
        print(str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
