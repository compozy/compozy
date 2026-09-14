#!/usr/bin/env python3
"""Deep-review plan gate (bootstrap helper; writes only under --out).

Validates plan.json cohorts against the manifest (every selected file owned
exactly once; hunk_scope slices line-exact; size caps), renders every reviewer
and sweep prompt from assets/PROMPT.md — injecting only the rules whose scope
globs match that cohort's files — and materializes jobs.json, the work
contract every execution engine runs.

Prompt consistency is enforced here: the build fails when a template lost a
mandatory placeholder or a rendered prompt still carries an unfilled one.

Requires in <out>: manifest.json and knowledge.json (bootstrap-authored),
context-pack.md, rules.json, and plan.json (orchestrator-authored).
Exit codes: 0 ok, 1 validation failure or missing artifact.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import shlex
import sys
from pathlib import Path

sys.dont_write_bytecode = True  # keep the tracked skill tree free of __pycache__

from _planning import (
    DEFAULT_MAX_COHORT_FILES, DEFAULT_MAX_POLISH_FILES, MAX_COHORT_CHANGED_LINES,
    MAX_POLISH_CHANGED_LINES, automatic_plan, polish_cohorts, validate_cohorts,
)

from _common import (
    ASSETS_DIR,
    glob_to_regex,
    hunk_text,
    manifest_selected,
    read_json,
    rel,
    repo_root,
    skill_rel,
    write_json,
)

REVIEWER_PLACEHOLDERS = {"label", "contract", "draft", "submit_command", "lane_instruction", "result_contract"}
SWEEP_PLACEHOLDERS = REVIEWER_PLACEHOLDERS

DEFAULT_LENSES = {
    "contracts": (
        "MISSION: prove changed contracts co-ship across implementation, clients, and generated artifacts. "
        "FOCUS: breaking shapes, defaults, requiredness, and version drift. REPORT GATE: trace a changed "
        "producer contract to a concrete incompatible consumer."
    ),
    "security": (
        "MISSION: trace attacker-controlled input to impact. FOCUS: injection, authn/authz gaps, secret "
        "leakage, cross-tenant access, and unsafe input handling. REPORT GATE: name the controllable input, "
        "missing boundary, and reachable impact."
    ),
    "migrations": (
        "MISSION: prove schema and code can roll forward safely. FOCUS: destructive operations, missing model "
        "migrations, ordering, identity, and compatibility windows. REPORT GATE: name the database state and "
        "operation that loses data or breaks a deployed version."
    ),
    "tests": (
        "MISSION: find changed behavior with no failing-capable protection. FOCUS: untested branches, mock-only "
        "assertions, and weakened expectations. REPORT GATE: name the regression the current suite would pass."
    ),
    "consistency": (
        "MISSION: prove a cross-file change is complete. FOCUS: incomplete renames, sibling paths that share an "
        "invariant, and duplicated fixes. REPORT GATE: connect every missed occurrence to the same changed invariant."
    ),
    "config": (
        "MISSION: trace configuration from declaration through defaulting to consumption. FOCUS: unwired keys, "
        "dead flags, undocumented public settings, and default mismatches. REPORT GATE: name the runtime path "
        "where the configured value is ignored or misread."
    ),
    "spec-parity": (
        "MISSION: prove field-by-field conformance with every artifact in the context pack's Spec contract section. "
        "FOCUS: names, types, defaults, requiredness, shapes, topology, and behavior. REPORT GATE: cite the exact "
        "artifact field and contradictory implementation path."
    ),
}

SPEC_EXTRA = (
    " Read EVERY artifact in the context pack's Spec contract section in full and compare the "
    "implementation to each one FIELD BY FIELD: names, types, defaults, required-vs-optional flags, "
    "shapes, topologies, command surfaces, behaviors. A deliverable that contradicts a canonical "
    "artifact is a Critical potential-issue, never a nitpick; never reinterpret the artifact to match "
    "what was built. When an artifact names a visual reference, require its parity evidence bundle. "
    "Set guideline to `<artifact path> — <section/field>` on every finding. An empty result asserts "
    "every listed artifact conforms."
)

PLACEHOLDER_RE = re.compile(r"\{\{([a-z_]+)\}\}")


def positive_int(value: str) -> int:
    parsed = int(value)
    if parsed < 1:
        raise argparse.ArgumentTypeError("must be at least 1")
    return parsed


def load_template(name: str) -> str:
    text = (ASSETS_DIR / "PROMPT.md").read_text(encoding="utf-8")
    match = re.search(
        rf"<!-- template:{name} -->\n(.*?)\n<!-- /template -->", text, re.S
    )
    if match is None:
        raise RuntimeError(f"assets/PROMPT.md has no `<!-- template:{name} -->` block")
    return match.group(1)


def render_template(name: str, template: str, required: set[str], values: dict[str, str]) -> str:
    present = set(PLACEHOLDER_RE.findall(template))
    missing = required - present
    if missing:
        raise RuntimeError(
            f"assets/PROMPT.md template `{name}` lost mandatory placeholder(s): {sorted(missing)}"
        )
    unknown = present - set(values)
    if unknown:
        raise RuntimeError(
            f"template `{name}` uses placeholder(s) with no value: {sorted(unknown)}"
        )
    rendered = PLACEHOLDER_RE.sub(lambda m: values[m.group(1)], template)
    leftover = PLACEHOLDER_RE.findall(rendered)
    if leftover:
        raise RuntimeError(f"template `{name}` rendered with unfilled placeholder(s): {leftover}")
    return rendered


def validate_registry(registry: dict, knowledge: dict, selected: dict[str, dict]) -> list[str]:
    errors = []
    rules = registry.get("rules", [])
    if sorted(knowledge.get("selected_paths", [])) != sorted(selected):
        errors.append("knowledge.json: selected paths are stale; rerun build_knowledge.py")
    expected_sources = {source["path"]: source for source in knowledge.get("sources", [])}
    rows = registry.get("sources")
    if not isinstance(rows, list):
        return ["rules.json: sources must account for every knowledge.json source"]
    actual_sources = [row.get("source") for row in rows]
    if len(actual_sources) != len(set(actual_sources)):
        errors.append("rules.json: duplicate source accounting rows")
    missing = set(expected_sources) - set(actual_sources)
    extra = set(actual_sources) - set(expected_sources)
    if missing or extra:
        errors.append(
            f"rules.json: source accounting mismatch missing={sorted(missing)[:8]} "
            f"extra={sorted(extra)[:8]}"
        )
    applied_sources = set()
    for row in rows:
        source, status = row.get("source"), row.get("status")
        if status not in {"applied", "not-applicable"}:
            errors.append(f"rules.json: source {source!r} status must be applied|not-applicable")
        if not str(row.get("reason", "")).strip():
            errors.append(f"rules.json: source {source!r} needs a concrete reason")
        if status == "applied":
            applied_sources.add(source)
    ids = [rule.get("id") for rule in rules]
    if len(ids) != len(set(ids)):
        errors.append("rules.json: duplicate rule ids")
    for rule in rules:
        if not rule.get("id") or not str(rule.get("guideline", "")).strip():
            errors.append(f"rules.json: rule {rule.get('id')!r} lacks id or guideline")
        scope = rule.get("scope")
        if not isinstance(scope, list) or not scope:
            errors.append(f"rules.json: rule {rule.get('id')!r} needs a scope glob list")
            continue
        if rule.get("source") not in applied_sources:
            errors.append(
                f"rules.json: rule {rule.get('id')!r} cites source {rule.get('source')!r} "
                "that is not marked applied"
            )
        regexes = [glob_to_regex(str(glob)) for glob in scope]
        if not any(rx.match(path) for rx in regexes for path in selected):
            errors.append(
                f"rules.json: rule {rule.get('id')!r} scope matches no selected path"
            )
    return errors


def normalize_sweeps(plan: dict, context_pack: str) -> list[dict]:
    sweeps, errors = [], []
    for entry in plan.get("sweeps", []):
        if isinstance(entry, str):
            entry = {"key": entry}
        if not isinstance(entry, dict):
            errors.append("sweeps must contain keys or objects")
            continue
        key = entry.get("key")
        lens = entry.get("lens") or DEFAULT_LENSES.get(key)
        if not isinstance(key, str) or not re.fullmatch(r"[a-z0-9][a-z0-9_-]*", key) or not lens:
            errors.append(f"sweep {entry!r} needs a safe key and a built-in or explicit lens")
            continue
        if key != "spec-parity" and not str(entry.get("hypothesis", "")).strip():
            errors.append(f"sweep {key!r}: name the cross-cohort hypothesis; local behavior/tests already have owners")
            continue
        sweeps.append({**entry, "key": key, "lens": lens})
    keys = [sweep["key"] for sweep in sweeps]
    if len(keys) != len(set(keys)):
        errors.append("duplicate sweep keys")
    if "spec-parity" in keys and not re.search(r"^#{1,2} Spec contract\b", context_pack, re.M):
        errors.append("spec-parity sweep planned but context-pack.md has no Spec contract section")
    if errors:
        raise RuntimeError("sweep validation failed:\n- " + "\n- ".join(errors))
    return sweeps


def cohort_rules(rules: list[dict], files: list[str], lane: str | None = None, sweep: str | None = None) -> list[dict]:
    compiled = [(rule, [glob_to_regex(str(glob)) for glob in rule["scope"]]) for rule in rules]
    return [
        rule
        for rule, regexes in compiled
        if any(rx.match(path) for rx in regexes for path in files)
        and (lane is None or lane in rule.get("lanes", ["defect", "polish"]))
        and (sweep is None or sweep in rule.get("sweeps", []))
    ]


def rules_block(rules: list[dict], files: list[str]) -> tuple[str, int]:
    bound = cohort_rules(rules, files)
    if not bound:
        return (
            "- No repo rules map to these files — judge on the taxonomy and general correctness alone.",
            0,
        )
    lines = [
        f"- [{rule['id']}] (`{rule['source']}`): \"{' '.join(str(rule['guideline']).split())}\""
        for rule in bound
    ]
    return "\n".join(lines), len(bound)


def owned_hunks(cohort: dict, selected: dict[str, dict]) -> list[dict]:
    scope = cohort.get("hunk_scope") or {}
    result = []
    for path in cohort["files"]:
        for hunk in scope.get(path, selected[path]["hunks"]):
            anchor = hunk_text(hunk)
            identity = hashlib.sha256(f"{path}\0{anchor}".encode()).hexdigest()[:12]
            result.append({"id": f"H{identity}", "file": path, "hunk": anchor})
    return result


def result_contract(lane: str) -> str:
    classes = "defects" if lane == "defect" else "advisories" if lane == "polish" else "defects or advisories"
    return (
        'Draft shape: {"job_digest":"copy from contract", "summary":"what changed in this assignment", '
        '"defects":[], "advisories":[], "suppressions":[], '
        '"coverage":{"hunks":[["H-id","clear or reported"]], '
        '"rules":[["R-id","compliant or violated or not-applicable","evidence note"]]}}. '
        'Every assigned hunk/rule needs an explicit assessment; do not infer success from absence. '
        'When required_assessment is true, also fill assessment with status "complete" and a nonblank note '
        'describing the investigation and its evidence; an untouched template cannot complete a sweep or a job without hunks. '
        f'Findings go in {classes}. Each finding has file, line (integer), in_diff (boolean), '
        'hunk (assigned H-id, or null outside diff), category, severity, quick_win (boolean), title (<=100 chars), '
        'body, rule_ids (array), evidence (nonempty array). Optional: end_line, also_applies, guideline, suggestion. '
        'Defects: category potential-issue, severity critical/major/minor. '
        'Advisories: category refactor/nitpick, severity minor/trivial. '
        'Suppressions: {file,line,hunk,candidate,reason,rule_ids,note}; line/hunk may be null outside diff. '
        'Only use assigned rule IDs and quote their guideline verbatim. '
        'Do not generate Python, parse prompt Markdown, or enumerate paths again to create coverage; '
        'the contract JSON supplies the IDs and the submit command expands metadata.'
    )


def filtered_context(out: Path, files: list[str], sweep: bool = False) -> dict:
    path = out / "review-context.json"
    if not path.exists():
        return {"text": (out / "context-pack.md").read_text(encoding="utf-8")}
    context = read_json(path)
    lanes = []
    for lane in context.get("linters", []):
        patterns = lane.get("scope", ["**/*"])
        if any(glob_to_regex(p).match(file) for p in patterns for file in files):
            lanes.append(lane)
    return {"intent": context.get("intent", ""), "linters": lanes,
            "spec_artifacts": context.get("spec_artifacts", []) if sweep else []}


def materialize_jobs(repo: Path, out: Path, max_files: int | None = None, max_lines: int | None = None,
                     max_polish_files: int | None = None, max_polish_lines: int | None = None) -> dict:
    manifest, plan = read_json(out / "manifest.json"), read_json(out / "plan.json")
    limits = dict(plan.get("limits", {}))
    for name, value, default in (
        ("cohort_files", max_files, DEFAULT_MAX_COHORT_FILES),
        ("cohort_changed_lines", max_lines, MAX_COHORT_CHANGED_LINES),
        ("polish_files", max_polish_files, DEFAULT_MAX_POLISH_FILES),
        ("polish_changed_lines", max_polish_lines, MAX_POLISH_CHANGED_LINES),
    ):
        resolved = value if value is not None else limits.get(name, default)
        if type(resolved) is not int or resolved < 1:
            raise ValueError(f"{name} must be a positive integer")
        limits[name] = resolved
    registry, knowledge = read_json(out / "rules.json"), read_json(out / "knowledge.json")
    rules = registry.get("rules")
    if not isinstance(rules, list):
        raise RuntimeError("rules.json: rules must be an array")
    context_pack = (out / "context-pack.md").read_text(encoding="utf-8")
    if "diff_command" not in manifest:
        raise RuntimeError("manifest.json lacks diff_command — rebuild the manifest")
    selected = manifest_selected(manifest)
    errors = validate_registry(registry, knowledge, selected) + validate_cohorts(
        plan["cohorts"], selected, limits["cohort_files"], limits["cohort_changed_lines"])
    if knowledge.get("version") == 2:
        from build_knowledge import verify_evidence
        errors.extend(verify_evidence(repo, manifest, knowledge, registry))
    for rule in rules:
        lanes = rule.get("lanes", ["defect", "polish"])
        if not isinstance(lanes, list) or not lanes or set(lanes) - {"defect", "polish"}:
            errors.append(f"rule {rule.get('id')}: lanes must assign defect and/or polish")
        if not isinstance(rule.get("sweeps", []), list):
            errors.append(f"rule {rule.get('id')}: sweeps must be a list")
    if errors:
        raise RuntimeError("plan validation failed:\n- " + "\n- ".join(errors))
    sweeps = normalize_sweeps(plan, context_pack)
    if re.search(r"^#{1,2} Spec contract\b", context_pack, re.M) and not any(s["key"] == "spec-parity" for s in sweeps):
        sweeps.append({"key": "spec-parity", "lens": DEFAULT_LENSES["spec-parity"]})
    for folder in ("prompts", "agents", "runs", "contracts"):
        (out / folder).mkdir(parents=True, exist_ok=True)
    polish = polish_cohorts(plan["cohorts"], selected, limits["polish_files"], limits["polish_changed_lines"])
    specs = [(f"cohort-{c['id'].lower()}", "cohort", "defect", c) for c in plan["cohorts"]]
    specs += [(f"polish-{c['id'].lower()}", "polish", "polish", c) for c in polish]
    specs += [(f"sweep-{s['key']}", "sweep", "sweep", s) for s in sweeps]
    jobs = []
    skill_digest = hashlib.sha256()
    for file in sorted([*Path(__file__).parent.glob("*.py"), *ASSETS_DIR.glob("*.md"), *ASSETS_DIR.glob("*.json")]):
        skill_digest.update(file.name.encode() + file.read_bytes())
    for label, kind, lane, item in specs:
        files = list(selected) if lane == "sweep" else item["files"]
        key = item.get("key") if lane == "sweep" else None
        bound = cohort_rules(rules, files, lane if lane != "sweep" else None, key)
        required = [] if lane == "sweep" else owned_hunks(item, selected)
        context = filtered_context(out, files, lane == "sweep")
        job = {"label": label, "kind": kind, "lane": lane, "protocol": "compact-v1", "skill_digest": skill_digest.hexdigest(),
               "coverage_check": f"sweep:{key}" if key else lane, "required_hunks": required,
               "required_assessment": lane == "sweep" or not required,
               "rule_ids": [r["id"] for r in bound], "rules": bound, "context": context,
               "files": [{"path": p, "status": selected[p]["status"]} for p in files],
               "diff_command": manifest["diff_command"], "base": manifest["base"],
               "snapshot": manifest.get("worktree_snapshot"), "target": manifest["target"],
               "risk": item.get("risk", "high"), "name": item.get("name", key),
               "lens": item.get("lens", ""), "hypothesis": item.get("hypothesis", ""),
               "prompt": rel(out / "prompts" / f"{label}.md", repo),
               "contract": rel(out / "contracts" / f"{label}.json", repo),
               "draft": rel(out / "agents" / f"{label}.draft.json", repo),
               "output": rel(out / "agents" / f"{label}.json", repo)}
        if lane == "sweep":
            job["anchor_hunks"] = owned_hunks({"files": files}, selected)
        job["job_digest"] = hashlib.sha256(json.dumps(job, sort_keys=True).encode()).hexdigest()
        write_json(repo / job["contract"], job)
        template = {"job_digest": job["job_digest"], "summary": "", "defects": [], "advisories": [], "suppressions": [],
                    "coverage": {"hunks": [[h["id"], None] for h in required], "rules": [[r["id"], None, ""] for r in bound]}}
        if job["required_assessment"]:
            template["assessment"] = {"status": None, "note": ""}
        write_json(out / "agents" / f"{label}.draft.template.json", template)
        command = " ".join(shlex.quote(v) for v in ["python3", f"{skill_rel(repo)}/scripts/run_jobs.py", "--out", rel(out, repo), "--job", label, "--submit"])
        instructions = "Investigate correctness, security, data, contracts, reliability and failing-capable test defects; leave advisories empty." if lane == "defect" else "Investigate every actionable maintainability, simplification, naming, documentation, idiom and project-rule improvement; leave defects empty." if lane == "polish" else item["lens"] + (SPEC_EXTRA if key == "spec-parity" else "")
        values = {"label": label, "contract": job["contract"], "draft": job["draft"], "submit_command": command,
                  "lane_instruction": instructions, "result_contract": result_contract(lane)}
        prompt = render_template("sweep" if lane == "sweep" else "reviewer", load_template("sweep" if lane == "sweep" else "reviewer"), REVIEWER_PLACEHOLDERS, values)
        (repo / job["prompt"]).write_text(prompt, encoding="utf-8")
        jobs.append(job)
    # The dispatch index stays small: complete contracts live once in per-job files.
    # Canonical consumers still receive ownership/rules, without context and source text duplication.
    index = [{k: v for k, v in j.items() if k not in {"rules", "context", "files", "lens", "hypothesis"}} for j in jobs]
    payload = {"protocol": "compact-v1", "skill_digest": skill_digest.hexdigest(), "limits": limits, "jobs": index}
    payload["input_hashes"] = {name: hashlib.sha256((out / name).read_bytes()).hexdigest()
                             for name in ("manifest.json", "knowledge.json", "rules.json", "plan.json", "context-pack.md", "review-context.json")
                             if (out / name).is_file()}
    write_json(out / "jobs.json", payload)
    return {"defect": len(plan["cohorts"]), "polish": len(polish), "sweeps": len(sweeps), "jobs": len(jobs)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--max-cohort-files", type=positive_int, help="defect files per job; inherit plan or default 200")
    parser.add_argument("--max-cohort-lines", type=positive_int, help="defect changed lines per job; inherit plan or default 15000")
    parser.add_argument("--max-polish-files", type=positive_int, help="polish files per job; inherit plan or default 200")
    parser.add_argument("--max-polish-lines", type=positive_int, help="polish changed lines per job; inherit plan or default 15000")
    args = parser.parse_args()
    try:
        out = Path(args.out).resolve()
        summary = materialize_jobs(repo_root(), out, args.max_cohort_files, args.max_cohort_lines,
                                   args.max_polish_files, args.max_polish_lines)
        limits = read_json(out / "jobs.json")["limits"]
    except (RuntimeError, OSError, ValueError, KeyError, TypeError) as error:
        sys.stderr.write(f"{error}\n")
        return 1
    print(f"jobs: {summary['defect']} defect + {summary['polish']} polish + {summary['sweeps']} sweeps; "
          f"limits {limits['cohort_files']}/{limits['cohort_changed_lines']} defect, "
          f"{limits['polish_files']}/{limits['polish_changed_lines']} polish")
    return 0


if __name__ == "__main__":
    sys.exit(main())
