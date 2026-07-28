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
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

sys.dont_write_bytecode = True  # keep the tracked skill tree free of __pycache__

from _common import (
    ASSETS_DIR,
    glob_to_regex,
    hunk_text,
    load_schema,
    manifest_selected,
    read_json,
    rel,
    repo_root,
    skill_rel,
    write_json,
)

DEFAULT_MAX_COHORT_FILES = 100
MAX_COHORT_CHANGED_LINES = 6000
DEFAULT_MAX_POLISH_FILES = 20
MAX_POLISH_CHANGED_LINES = 1200

REVIEWER_PLACEHOLDERS = {
    "cohort_name", "risk", "target", "file_list", "scope_instruction", "context",
    "taxonomy", "rules_block", "diff_command", "base", "output", "schema",
    "lane_instruction", "coverage_contract",
}
SWEEP_PLACEHOLDERS = {
    "sweep_key", "lens", "target", "context", "manifest", "taxonomy",
    "diff_command", "spec_extra", "output", "schema", "rules_block",
    "coverage_contract",
}

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


def canonical_hunk(hunk: dict) -> tuple[int, int, str]:
    return int(hunk["start"]), int(hunk["lines"]), str(hunk.get("side", "new"))


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


def validate_cohorts(
    cohorts: list[dict], selected: dict[str, dict], max_cohort_files: int
) -> list[str]:
    errors: list[str] = []
    seen_ids: set[str] = set()
    full_owners: dict[str, list[str]] = defaultdict(list)
    scoped_owners: dict[str, list[tuple[str, tuple[int, int, str]]]] = defaultdict(list)
    for cohort in cohorts:
        cohort_id = cohort.get("id")
        if not cohort_id or cohort_id in seen_ids:
            errors.append(f"duplicate or missing cohort id {cohort_id!r}")
        seen_ids.add(cohort_id)
        if cohort.get("risk") not in {"high", "normal", "low"}:
            errors.append(f"{cohort_id}: risk must be high|normal|low")
        files = cohort.get("files", [])
        if not files or len(files) > max_cohort_files:
            errors.append(
                f"{cohort_id}: invalid file count {len(files)} (1..{max_cohort_files})"
            )
        scope = cohort.get("hunk_scope") or {}
        extra_scope = set(scope) - set(files)
        if extra_scope:
            errors.append(f"{cohort_id}: hunk_scope paths absent from files: {sorted(extra_scope)}")
        for path in files:
            if path not in selected:
                errors.append(f"{cohort_id}: non-selected or unknown path {path}")
                continue
            if path in scope:
                for hunk in scope[path]:
                    scoped_owners[path].append((cohort_id, canonical_hunk(hunk)))
            else:
                full_owners[path].append(cohort_id)
        if scope:
            scoped_lines = sum(int(h["lines"]) for hunks in scope.values() for h in hunks)
            if scoped_lines > MAX_COHORT_CHANGED_LINES:
                errors.append(f"{cohort_id}: scoped changed lines {scoped_lines} > {MAX_COHORT_CHANGED_LINES}")
        else:
            changed = sum(
                int(selected[path].get("adds") or 0) + int(selected[path].get("dels") or 0)
                for path in files
                if path in selected
            )
            if changed > MAX_COHORT_CHANGED_LINES:
                errors.append(
                    f"{cohort_id}: changed lines {changed} > {MAX_COHORT_CHANGED_LINES} without hunk_scope"
                )

    for path, item in selected.items():
        full, scoped = full_owners.get(path, []), scoped_owners.get(path, [])
        if full and scoped:
            errors.append(f"{path}: mixed full and scoped ownership")
        elif full:
            if len(full) != 1:
                errors.append(f"{path}: owned by {len(full)} cohorts ({full})")
        elif scoped:
            want = Counter(
                (side, line)
                for start, lines, side in (canonical_hunk(h) for h in item["hunks"])
                for line in range(start, start + lines)
            )
            got = Counter(
                (side, line)
                for _, (start, lines, side) in scoped
                for line in range(start, start + lines)
            )
            if got != want:
                errors.append(
                    f"{path}: hunk slice mismatch missing_lines={sum((want - got).values())} "
                    f"duplicated_or_extra_lines={sum((got - want).values())}"
                )
        else:
            errors.append(f"{path}: missing cohort ownership")
    return errors


def normalize_sweeps(plan: dict, context_pack: str) -> list[dict]:
    sweeps, errors = [], []
    for entry in plan.get("sweeps", []):
        if isinstance(entry, str):
            key, lens = entry, DEFAULT_LENSES.get(entry)
            if lens is None:
                errors.append(f"sweep {entry!r} has no built-in lens; use {{key, lens}}")
                continue
        else:
            key, lens = entry.get("key"), entry.get("lens")
            if not key or not lens:
                errors.append(f"sweep entry {entry!r} needs key and lens")
                continue
        sweeps.append({"key": key, "lens": lens})
    keys = [sweep["key"] for sweep in sweeps]
    if len(keys) != len(set(keys)):
        errors.append("duplicate sweep keys")
    if "spec-parity" in keys and not re.search(r"^#{1,2} Spec contract\b", context_pack, re.M):
        errors.append("spec-parity sweep planned but context-pack.md has no Spec contract section")
    if errors:
        raise RuntimeError("sweep validation failed:\n- " + "\n- ".join(errors))
    return sweeps


def cohort_rules(rules: list[dict], files: list[str]) -> list[dict]:
    compiled = [(rule, [glob_to_regex(str(glob)) for glob in rule["scope"]]) for rule in rules]
    return [
        rule
        for rule, regexes in compiled
        if any(rx.match(path) for rx in regexes for path in files)
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


def file_list_block(cohort: dict, selected: dict[str, dict]) -> str:
    return "\n".join(
        f"- `{path}` status={selected[path]['status']} hunks="
        + ", ".join(
            hunk_text(h)
            for h in (cohort.get("hunk_scope", {}).get(path) or selected[path]["hunks"])
        )
        for path in cohort["files"]
    )


def scope_instruction(cohort: dict) -> str:
    scope = cohort.get("hunk_scope")
    if scope:
        return (
            "Judge ONLY these hunks — sibling reviewers own the rest of the file; read beyond them "
            f"freely, report inside them: `{json.dumps(scope, separators=(',', ':'))}`."
        )
    return "Judge every manifest hunk of every listed file."


def owned_hunks(cohort: dict, selected: dict[str, dict]) -> list[dict]:
    scope = cohort.get("hunk_scope") or {}
    return [
        {"file": path, "hunk": hunk_text(hunk)}
        for path in cohort["files"]
        for hunk in (scope.get(path) or selected[path]["hunks"])
    ]


def split_hunk(hunk: dict, limit: int) -> list[dict]:
    start, remaining = int(hunk["start"]), int(hunk["lines"])
    side, chunks = str(hunk.get("side", "new")), []
    while remaining:
        size = min(limit, remaining)
        chunks.append({"start": start, "lines": size, "side": side})
        start += size
        remaining -= size
    return chunks


def polish_cohorts(
    cohorts: list[dict], selected: dict[str, dict], max_files: int, max_lines: int
) -> list[dict]:
    """Create a second, smaller ownership partition for the polish lane."""
    result: list[dict] = []
    for cohort in cohorts:
        units: list[tuple[str, list[dict]]] = []
        source_scope = cohort.get("hunk_scope") or {}
        for path in cohort["files"]:
            hunks = source_scope.get(path) or selected[path]["hunks"]
            expanded = [piece for hunk in hunks for piece in split_hunk(hunk, max_lines)]
            current: list[dict] = []
            current_lines = 0
            for hunk in expanded:
                lines = int(hunk["lines"])
                if current and current_lines + lines > max_lines:
                    units.append((path, current))
                    current, current_lines = [], 0
                current.append(hunk)
                current_lines += lines
            if current or not expanded:
                units.append((path, current))

        batch: dict[str, list[dict]] = {}
        batch_lines = 0

        def flush() -> None:
            nonlocal batch, batch_lines
            if not batch:
                return
            index = len([item for item in result if item["parent_id"] == cohort["id"]]) + 1
            result.append({
                "id": f"{cohort['id']}-p{index:02d}",
                "parent_id": cohort["id"],
                "name": f"{cohort['name']} — polish {index}",
                "risk": cohort["risk"],
                "files": list(batch),
                "hunk_scope": {path: hunks for path, hunks in batch.items()},
            })
            batch, batch_lines = {}, 0

        for path, hunks in units:
            unit_lines = sum(int(hunk["lines"]) for hunk in hunks)
            adds_file = path not in batch
            if batch and (
                batch_lines + unit_lines > max_lines
                or (adds_file and len(batch) >= max_files)
                or path in batch
            ):
                flush()
            batch[path] = hunks
            batch_lines += unit_lines
        flush()
    return result


def coverage_contract(required_hunks: list[dict], rule_ids: list[str], check: str) -> str:
    return (
        "HUNK COVERAGE (one exact row per assignment; include check "
        f"`{check}`): `{json.dumps(required_hunks, separators=(',', ':'))}`\n"
        "RULE COVERAGE (one exact row per id, even when compliant or not applicable): "
        f"`{json.dumps(rule_ids, separators=(',', ':'))}`"
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument(
        "--max-cohort-files",
        type=positive_int,
        default=DEFAULT_MAX_COHORT_FILES,
        help=f"maximum files per cohort (default: {DEFAULT_MAX_COHORT_FILES})",
    )
    args = parser.parse_args()

    repo = repo_root()
    out = Path(args.out).resolve()
    try:
        manifest = read_json(out / "manifest.json")
        plan = read_json(out / "plan.json")
        registry = read_json(out / "rules.json")
        knowledge = read_json(out / "knowledge.json")
        rules = registry.get("rules")
        if not isinstance(rules, list):
            raise RuntimeError("rules.json: rules must be an array")
        context_pack = (out / "context-pack.md").read_text(encoding="utf-8")
        if "diff_command" not in manifest:
            raise RuntimeError("manifest.json lacks diff_command — rebuild it with the current build_manifest.py")

        selected = manifest_selected(manifest)
        errors = validate_registry(registry, knowledge, selected) + validate_cohorts(
            plan["cohorts"], selected, args.max_cohort_files
        )
        if errors:
            raise RuntimeError("plan validation failed:\n- " + "\n- ".join(errors))
        sweeps = normalize_sweeps(plan, context_pack)

        reviewer_template = load_template("reviewer")
        sweep_template = load_template("sweep")
        schema = json.dumps(load_schema("findings"), separators=(",", ":"))
        shared = {
            "target": manifest["target"],
            "context": rel(out / "context-pack.md", repo),
            "taxonomy": f"{skill_rel(repo)}/references/taxonomy.md",
            "diff_command": manifest["diff_command"],
            "schema": schema,
        }
        prompts_dir = out / "prompts"
        prompts_dir.mkdir(parents=True, exist_ok=True)
        (out / "agents").mkdir(exist_ok=True)
        (out / "runs").mkdir(exist_ok=True)

        jobs, bound_counts = [], []
        for cohort in plan["cohorts"]:
            label = f"cohort-{cohort['id'].lower()}"
            output = out / "agents" / f"{label}.json"
            bound_rules = cohort_rules(rules, cohort["files"])
            block, bound = rules_block(rules, cohort["files"])
            rule_ids = [rule["id"] for rule in bound_rules]
            required_hunks = owned_hunks(cohort, selected)
            bound_counts.append(bound)
            prompt = render_template("reviewer", reviewer_template, REVIEWER_PLACEHOLDERS, {
                **shared,
                "cohort_name": cohort["name"],
                "risk": cohort["risk"],
                "file_list": file_list_block(cohort, selected),
                "scope_instruction": scope_instruction(cohort),
                "rules_block": block,
                "base": manifest["base"],
                "output": rel(output, repo),
                "lane_instruction": (
                    "DEFECT LANE: report only concrete correctness, security, data, contract, "
                    "reliability, or failing-capable test defects. Put survivors in `defects`; "
                    "leave `advisories` empty."
                ),
                "coverage_contract": coverage_contract(required_hunks, rule_ids, "defect"),
            })
            (prompts_dir / f"{label}.md").write_text(prompt, encoding="utf-8")
            jobs.append({
                "label": label, "kind": "cohort", "lane": "defect",
                "coverage_check": "defect", "required_hunks": required_hunks,
                "rule_ids": rule_ids,
                "prompt": rel(prompts_dir / f"{label}.md", repo),
                "output": rel(output, repo),
            })

        polish = polish_cohorts(
            plan["cohorts"], selected, DEFAULT_MAX_POLISH_FILES, MAX_POLISH_CHANGED_LINES
        )
        for cohort in polish:
            label = f"polish-{cohort['id'].lower()}"
            output = out / "agents" / f"{label}.json"
            bound_rules = cohort_rules(rules, cohort["files"])
            block, bound = rules_block(rules, cohort["files"])
            rule_ids = [rule["id"] for rule in bound_rules]
            required_hunks = owned_hunks(cohort, selected)
            bound_counts.append(bound)
            prompt = render_template("reviewer", reviewer_template, REVIEWER_PLACEHOLDERS, {
                **shared,
                "cohort_name": cohort["name"],
                "risk": cohort["risk"],
                "file_list": file_list_block(cohort, selected),
                "scope_instruction": scope_instruction(cohort),
                "rules_block": block,
                "base": manifest["base"],
                "output": rel(output, repo),
                "lane_instruction": (
                    "POLISH LANE: report every specific, actionable maintainability, simplification, "
                    "clarity, naming, documentation, idiom, and project-rule improvement. A runtime "
                    "failure is not required. Put survivors in `advisories`; leave `defects` empty."
                ),
                "coverage_contract": coverage_contract(required_hunks, rule_ids, "polish"),
            })
            (prompts_dir / f"{label}.md").write_text(prompt, encoding="utf-8")
            jobs.append({
                "label": label, "kind": "polish", "lane": "polish",
                "coverage_check": "polish", "required_hunks": required_hunks,
                "rule_ids": rule_ids,
                "prompt": rel(prompts_dir / f"{label}.md", repo),
                "output": rel(output, repo),
            })
        for sweep in sweeps:
            label = f"sweep-{sweep['key']}"
            output = out / "agents" / f"{label}.json"
            bound_rules = cohort_rules(rules, list(selected))
            block, _ = rules_block(rules, list(selected))
            rule_ids = [rule["id"] for rule in bound_rules]
            prompt = render_template("sweep", sweep_template, SWEEP_PLACEHOLDERS, {
                **shared,
                "sweep_key": sweep["key"],
                "lens": sweep["lens"],
                "manifest": rel(out / "manifest.json", repo),
                "spec_extra": SPEC_EXTRA if sweep["key"] == "spec-parity" else "",
                "output": rel(output, repo),
                "rules_block": block,
                "coverage_contract": coverage_contract([], rule_ids, f"sweep:{sweep['key']}"),
            })
            (prompts_dir / f"{label}.md").write_text(prompt, encoding="utf-8")
            jobs.append({
                "label": label, "kind": "sweep", "lane": "sweep",
                "coverage_check": f"sweep:{sweep['key']}", "required_hunks": [],
                "rule_ids": rule_ids,
                "prompt": rel(prompts_dir / f"{label}.md", repo),
                "output": rel(output, repo),
            })
        write_json(out / "jobs.json", {"jobs": jobs})
    except RuntimeError as error:
        sys.stderr.write(f"{error}\n")
        return 1

    with_rules = sum(1 for count in bound_counts if count)
    print(
        f"jobs: {len(plan['cohorts'])} defect cohorts + {len(polish)} polish cohorts + "
        f"{len(sweeps)} sweeps -> {out / 'jobs.json'}\n"
        f"cohort limit: {args.max_cohort_files} files / {MAX_COHORT_CHANGED_LINES} changed lines\n"
        f"polish limit: {DEFAULT_MAX_POLISH_FILES} files / {MAX_POLISH_CHANGED_LINES} changed lines\n"
        f"rules: {len(rules)} registered; {with_rules}/{len(bound_counts)} review lanes carry bound rules\n"
        f"every selected hunk has defect + polish ownership; prompts under {out / 'prompts'}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
