#!/usr/bin/env python3
"""Compile scoped knowledge and explicit applicability decisions under --out.

Discovery visits selected-path instruction ancestors and local skill routers.
The model triages the compact knowledge.md queue; references expand only after
an applied parent decision. Edit the generated rules.template.json, keeping its
evidence fields, and pass --decisions to compile exact rule spans to rules.json.
Unresolved sources remain pending and prevent the downstream review gate.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

sys.dont_write_bytecode = True

from _common import glob_to_regex, manifest_selected, read_json, rel, repo_root, write_json
from _knowledge_sources import (
    direct_references, frontmatter, instruction_paths, lexical_rel, walk_skills,
)

CONFIG_SOURCES = (".deep-review.yaml", ".deep-review.yml", ".coderabbit.yaml", ".coderabbit.yml")
SOURCE_ORDER = {"config": 0, "learning": 1, "instruction": 2, "skill": 3, "skill-reference": 4}


def digest(value: object) -> str:
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()


def source_paths(knowledge: dict, source: dict | str) -> list[str]:
    """Resolve normalized bindings, with support for pre-normalization registries."""
    if isinstance(source, str):
        source = next((row for row in knowledge.get("sources", []) if source in source_names(row)), None)
        if source is None:
            raise ValueError("unknown knowledge source")
    if "path_set" in source:
        key = source["path_set"]
        if key not in knowledge.get("path_sets", {}):
            raise ValueError(f"knowledge source {source.get('path')!r}: unknown path set {key!r}")
        return list(knowledge["path_sets"][key])
    return list(source.get("applies_to", []))


def source_names(source: dict) -> set[str]:
    return {name for name in (source.get("path"), source.get("id"), *(alias["path"] for alias in source.get("aliases", []))) if name}


def source_fingerprint(source: dict, path_sets: dict[str, list[str]]) -> str:
    aliases = sorted(source["aliases"], key=lambda alias: (alias["precedence"], alias["path"], alias["kind"], alias["path_set"]))
    parents = sorted(source.get("parent_bindings", []), key=lambda binding: binding["parent_skill"])
    return digest({"path": source["path"], "kinds": source["kinds"], "sha256": source["sha256"], "aliases": aliases, "paths": path_sets[source["path_set"]], "parents": parents})


def _decision_rows(decisions: dict | None) -> list[dict]:
    if decisions is None:
        return []
    if not isinstance(decisions, dict):
        raise ValueError("decisions must be an object")
    rows = decisions.get("sources", [])
    groups = decisions.get("groups", [])
    if not isinstance(rows, list) or not isinstance(groups, list):
        raise ValueError("decisions.sources and decisions.groups must be arrays")
    result = []
    for row in rows:
        if not isinstance(row, dict) or not isinstance(row.get("source"), str) or not row["source"]:
            raise ValueError("each decisions.sources entry needs a nonempty source path or id")
        result.append(dict(row))
    for group in groups:
        if not isinstance(group, dict) or not isinstance(group.get("sources"), list) or not group["sources"]:
            raise ValueError("each decisions.groups entry needs a nonempty sources array")
        if any(not isinstance(name, str) or not name for name in group["sources"]):
            raise ValueError("decisions.groups sources must be nonempty source paths or ids")
        result.extend({**{key: value for key, value in group.items() if key != "sources"}, "source": name} for name in group["sources"])
    for row in result:
        if row.get("status") not in {"pending", "applied", "not-applicable"}:
            raise ValueError(f"source {row['source']!r}: status must be pending|applied|not-applicable")
        if not isinstance(row.get("reason"), str) or not row["reason"].strip():
            raise ValueError(f"source {row['source']!r}: needs a concrete reason")
    return result


def _resolve_decisions(rows: list[dict], sources: list[dict], *, allow_unknown: bool = False) -> dict[str, dict]:
    names = {name: source for source in sources for name in source_names(source)}
    resolved: dict[str, dict] = {}
    for row in rows:
        source = names.get(row["source"])
        if source is None:
            if allow_unknown:
                continue
            raise ValueError(f"decision references unknown/stale source {row['source']!r}; regenerate knowledge and use its ids")
        path = source["path"]
        previous = resolved.get(path)
        if previous is not None:
            # A compact group can resolve untouched template rows. Two actual
            # decisions are ambiguous and never silently override one another.
            if previous["status"] == "pending" and row["status"] != "pending":
                resolved[path] = row
                continue
            if row["status"] == "pending" and previous["status"] != "pending":
                continue
            raise ValueError(f"duplicate source decision for {path!r}; keep one explicit decision")
        resolved[path] = row
    return resolved


def discover(repo: Path, manifest: dict, decisions: dict | None = None) -> tuple[dict, dict]:
    repo = repo.resolve()
    selected_map = manifest_selected(manifest)
    selected = sorted(selected_map)
    scope_fingerprint = digest({
        "target": manifest.get("target"), "base": manifest.get("base"),
        "effective_base": manifest.get("effective_base"), "head": manifest.get("head"),
        "worktree_snapshot": manifest.get("worktree_snapshot"),
        "files": [selected_map[path] for path in selected],
    })
    rows = _decision_rows(decisions)
    if decisions is not None and decisions.get("scope_fingerprint") != scope_fingerprint:
        raise ValueError("decisions: missing/stale scope_fingerprint; regenerate the template and reassess the current selection")
    path_sets: dict[str, list[str]] = {}
    sources: dict[str, dict] = {}
    texts: dict[str, str] = {}

    def intern(paths: list[str]) -> str:
        normalized = sorted(set(paths))
        key = "p" + digest(normalized)[:12]
        path_sets[key] = normalized
        return key

    def add(path: Path, kind: str, applicable: list[str], scopes: list[str], precedence: int = 0) -> dict:
        canonical = rel(path, repo)
        source = sources.get(canonical)
        if source is None:
            raw = path.read_bytes()
            texts[canonical] = raw.decode("utf-8", errors="replace")
            source = {
                "id": "s" + digest(canonical)[:12], "path": canonical, "kind": kind, "kinds": [],
                "sha256": hashlib.sha256(raw).hexdigest(), "aliases": [],
                "scope": [], "path_set": intern([]), "candidate": True,
            }
            sources[canonical] = source
        elif SOURCE_ORDER[kind] < SOURCE_ORDER[source["kind"]]:
            source["kind"] = kind
        source["kinds"] = sorted(set(source["kinds"]) | {kind}, key=SOURCE_ORDER.get)
        alias = {"path": lexical_rel(path, repo), "kind": kind, "scope": scopes, "precedence": precedence, "path_set": intern(applicable)}
        if alias not in source["aliases"]:
            source["aliases"].append(alias)
        source["scope"] = sorted(set(source["scope"]) | set(scopes))
        source["path_set"] = intern(path_sets[source["path_set"]] + applicable)
        return source

    for path in instruction_paths(repo, selected):
        parent = lexical_rel(path.parent, repo)
        applicable = [name for name in selected if parent == "." or name.startswith(parent + "/")]
        scopes = ["**/*"] if parent == "." else [f"{parent}/**"]
        source = add(path, "instruction", applicable, scopes, len(path.relative_to(repo).parts) - 1)
        source["candidate_reason"] = "instructions on selected-path ancestors; retain each alias's directory precedence"
    for name in CONFIG_SOURCES:
        path = repo / name
        if path.is_file():
            source = add(path, "config", selected, ["**/*"])
            source["candidate_reason"] = "repository review configuration can define path instructions"
    learnings = repo / ".deep-review" / "learnings.md"
    if learnings.is_file():
        source = add(learnings, "learning", selected, ["**/*"])
        source["candidate_reason"] = "repository review learnings can refine finding validity"

    instructions = [row for row in sources.values() if "instruction" in row["kinds"]]
    for path in walk_skills(repo):
        canonical = rel(path, repo)
        if canonical not in texts:
            # Scripts hash router bytes for freshness; reviewers need only its
            # metadata to reject it. Never infer relevance from lockfile words.
            text = path.read_text(encoding="utf-8", errors="replace")
        else:
            text = texts[canonical]
        metadata = frontmatter(text)
        name = metadata.get("name") or path.parent.name
        source = add(path, "skill", selected, ["**/*"])
        source.update(name=name, description=metadata.get("description", ""))
        source.setdefault("references", [])

    # A canonical skill can have differently named installed aliases. Compute
    # dispatch after collecting all aliases so an undispatched alias cannot
    # broaden another alias's explicitly scoped instruction binding.
    for source in sources.values():
        if "skill" not in source["kinds"]:
            continue
        pattern = re.compile(rf"(?<![a-z0-9-])\$?{re.escape(source['name'].lower())}(?![a-z0-9-])")
        dispatched = [row for row in instructions if pattern.search(texts[row["path"]].lower()) or any(alias["path"].lower() in texts[row["path"]].lower() for alias in source["aliases"])]
        applicable = sorted({item for row in dispatched for item in path_sets[row["path_set"]]}) if dispatched else selected
        source["dispatch_sources"] = sorted(row["path"] for row in dispatched)
        if source["kind"] == "skill":
            source["candidate_reason"] = "explicit instruction dispatch; assess its condition" if dispatched else "triage router metadata against selected behavior; no automatic word-overlap decision"
        for alias in source["aliases"]:
            if alias["kind"] == "skill":
                alias["path_set"] = intern(applicable)
        source["path_set"] = intern([path for alias in source["aliases"] for path in path_sets[alias["path_set"]]])

    first_decisions = _resolve_decisions(rows, list(sources.values()), allow_unknown=True)
    # Previously expanded reference rows retain their parent graph when a
    # parent decision is changed. This lets all-parent exclusion account for
    # those references without asking the model to read their bodies.
    carried_parents: set[str] = set()
    for row in rows:
        if row.get("parent_skills"):
            parents = row.get("parent_skills", [])
            if not isinstance(parents, list) or any(not isinstance(name, str) for name in parents):
                raise ValueError(f"source {row['source']!r}: parent_skills must be a path/id array")
            carried_parents.update(parents)
    routers = sorted((row for row in sources.values() if "skill" in row["kinds"]), key=lambda row: (not bool(source_names(row) & carried_parents), row["path"]))
    prior_fingerprints = {path: source_fingerprint(source, path_sets) for path, source in sources.items()}
    for parent in routers:
        status = first_decisions.get(parent["path"], {}).get("status")
        if status != "applied" and not (source_names(parent) & carried_parents):
            continue
        routes: dict[str, list[dict]] = {}
        for alias in parent["aliases"]:
            if alias["kind"] == "skill":
                for path, hints in direct_references(repo / alias["path"], repo, texts[parent["path"]]).items():
                    routes.setdefault(path, [])
                    routes[path].extend(hint for hint in hints if hint not in routes[path])
        parent["references"] = sorted(routes)
        for path, hints in routes.items():
            # Preserve every lexical parent binding across semantic exclusions.
            # Changing a decision is not a filesystem/scope change; removing a
            # parent must not erase another parent's reference accounting.
            reference = add(repo / path, "skill-reference", path_sets[parent["path_set"]], parent["scope"])
            binding = {"parent_skill": parent["path"], "path_set": parent["path_set"], "routing_hints": hints}
            reference.setdefault("parent_bindings", []).append(binding)
            reference["parent_skills"] = sorted({*reference.get("parent_skills", []), parent["path"]})
            reference["candidate_reason"] = "triage routing hints from applied parents; references require their own explicit assessment"
        if source_names(parent) & carried_parents:
            prior_fingerprints = {path: source_fingerprint(source, path_sets) for path, source in sources.items()}

    ordered_sources = sorted(sources.values(), key=lambda row: (SOURCE_ORDER[row["kind"]], row["path"]))
    resolved = _resolve_decisions(rows, ordered_sources)
    for source in ordered_sources:
        source["aliases"].sort(key=lambda alias: (alias["precedence"], alias["path"], alias["kind"], alias["path_set"]))
        source["fingerprint"] = source_fingerprint(source, path_sets)
        if source.get("parent_bindings"):
            source["parent_bindings"].sort(key=lambda binding: binding["parent_skill"])
            source["parent_skill"] = source["parent_skills"][0]  # legacy single-parent reader
            source["routing_hints"] = [hint for binding in source["parent_bindings"] for hint in binding["routing_hints"]]

    fingerprints = decisions.get("source_fingerprints", {}) if decisions else {}
    if not isinstance(fingerprints, dict):
        raise ValueError("decisions.source_fingerprints must map source ids/paths to fingerprints")
    registry_rows = []
    for source in ordered_sources:
        decision = resolved.get(source["path"])
        expanded_binding = False
        if decision is not None:
            supplied = decision.get("fingerprint")
            if supplied is None:
                supplied = next((fingerprints[name] for name in sorted(source_names(source)) if name in fingerprints), None)
            if supplied != source["fingerprint"]:
                if supplied and supplied == prior_fingerprints.get(source["path"]):
                    expanded_binding = True
                else:
                    raise ValueError(f"source {source['id']} ({source['path']}): missing/stale fingerprint; regenerate the template and reassess this source")
        status = decision["status"] if decision else "pending"
        reason = decision["reason"] if decision else source["candidate_reason"]
        if expanded_binding:
            status, reason = "pending", "new parent reference bindings expand this source; reassess the updated scope and routes"
        if source["kind"] == "skill-reference":
            parents = [first_decisions.get(path, {}).get("status", "pending") for path in source["parent_skills"]]
            if parents and all(value == "not-applicable" for value in parents):
                if status == "applied":
                    raise ValueError(f"source {source['path']!r}: cannot apply a reference when all parents are excluded")
                status, reason = "not-applicable", "all parent skills are explicitly not-applicable"
            elif status == "applied" and "applied" not in parents:
                raise ValueError(f"source {source['path']!r}: apply at least one parent before applying its reference")
        row = {"source": source["path"], "id": source["id"], "kind": source["kind"], "fingerprint": source["fingerprint"], "status": status, "reason": reason}
        if source.get("parent_skills"):
            row["parent_skills"] = source["parent_skills"]
        registry_rows.append(row)

    knowledge = {
        "version": 2, "target": manifest["target"], "selected_paths": selected,
        "scope_fingerprint": scope_fingerprint, "path_sets": path_sets, "sources": ordered_sources,
        "summary": {
            "instructions": sum(row["kind"] == "instruction" for row in ordered_sources),
            "supplemental": sum(row["kind"] in {"config", "learning"} for row in ordered_sources),
            "skills": sum(row["kind"] == "skill" for row in ordered_sources),
            "skill_references": sum(row["kind"] == "skill-reference" for row in ordered_sources),
            "pending_candidates": sum(row["status"] == "pending" for row in registry_rows),
        },
    }
    registry = {
        "version": 2, "scope_fingerprint": scope_fingerprint,
        "source_fingerprints": {row["id"]: row["fingerprint"] for row in ordered_sources},
        "sources": registry_rows, "rules": [],
    }
    if decisions:
        registry["rules"] = compile_rules(repo, knowledge, registry, decisions.get("rules", []))
    return knowledge, registry


def verify_evidence(repo: Path, manifest: dict, knowledge: dict, registry: dict) -> list[str]:
    """Recheck current local/external sources before using compiled decisions.

    Rebuilding discovery also catches added ancestor instructions, rewired skill
    aliases and newly installed routers, which a source-content-only hash check
    would miss. It never writes artifacts or fabricates pending decisions.
    """
    try:
        current, compiled = discover(repo, manifest, registry)
        errors = []
        if knowledge.get("scope_fingerprint") != current["scope_fingerprint"]:
            errors.append("knowledge.json: selected-scope evidence is missing/stale; rebuild the knowledge template")
        expected = {source["path"]: source for source in knowledge.get("sources", [])}
        actual = {source["path"]: source for source in current["sources"]}
        if set(expected) != set(actual):
            errors.append(f"knowledge.json: source discovery changed; missing={sorted(set(actual) - set(expected))[:8]} removed={sorted(set(expected) - set(actual))[:8]}; rebuild and triage the template")
        for path in sorted(set(expected) & set(actual)):
            if source_fingerprint(expected[path], knowledge.get("path_sets", {})) != actual[path]["fingerprint"] or expected[path].get("fingerprint") != actual[path]["fingerprint"]:
                errors.append(f"knowledge.json: source {path!r} has stale content or scope evidence; rebuild and reassess")
        original_status = {row["source"]: row["status"] for row in registry.get("sources", [])}
        for row in compiled["sources"]:
            if row["status"] != original_status.get(row["source"]):
                errors.append(f"rules.json: source {row['source']!r} needs updated applicability accounting ({row['status']})")
        if compiled["rules"] != registry.get("rules", []):
            errors.append("rules.json: rules are not the compiled exact-text registry; rerun build_knowledge.py --decisions")
        return errors
    except (OSError, RuntimeError, ValueError, KeyError, TypeError) as error:
        return [f"knowledge/rules evidence: {error}"]


def compile_rules(repo: Path, knowledge: dict, registry: dict, rules: list[dict]) -> list[dict]:
    if not isinstance(rules, list):
        raise ValueError("decisions.rules must be an array")
    names = {name: source for source in knowledge["sources"] for name in source_names(source)}
    applied = {row["source"] for row in registry["sources"] if row["status"] == "applied"}
    compiled: list[dict] = []
    seen: set[str] = set()
    for rule in rules:
        if not isinstance(rule, dict) or not isinstance(rule.get("id"), str) or not rule["id"].strip():
            raise ValueError("each rule needs a nonempty id")
        label = rule["id"]
        if label in seen:
            raise ValueError(f"duplicate rule id {label!r}")
        seen.add(label)
        if not isinstance(rule.get("source"), str):
            raise ValueError(f"rule {label!r}: source must be a path or compact source id")
        source = names.get(rule["source"])
        if source is None or source["path"] not in applied:
            raise ValueError(f"rule {label!r}: source {rule.get('source')!r} must be known and explicitly applied")
        scope = rule.get("scope")
        if not isinstance(scope, list) or not scope or any(not isinstance(item, str) or not item for item in scope):
            raise ValueError(f"rule {label!r}: scope must be a nonempty glob array")
        matched = {path for glob in scope for path in knowledge["selected_paths"] if glob_to_regex(glob).match(path)}
        if not matched or matched - set(source_paths(knowledge, source)):
            raise ValueError(f"rule {label!r}: scope must match selected paths within its source bindings")
        text = (repo / source["path"]).read_text(encoding="utf-8", errors="replace")
        has_span = "start_line" in rule or "end_line" in rule
        if has_span:
            start, end = rule.get("start_line"), rule.get("end_line")
            lines = text.splitlines()
            if type(start) is not int or type(end) is not int or not 1 <= start <= end <= len(lines):
                raise ValueError(f"rule {label!r}: start_line/end_line must select existing source lines (1..{len(lines)})")
            guideline = "\n".join(lines[start - 1:end])
            if "guideline" in rule and rule["guideline"] != guideline:
                raise ValueError(f"rule {label!r}: guideline contradicts its exact source span")
        else:
            guideline = rule.get("guideline")
        if not isinstance(guideline, str) or not guideline.strip() or guideline not in text:
            raise ValueError(f"rule {label!r}: guideline must be exact text from its applied source, or supply line spans")
        compiled.append({**rule, "source": source["path"], "guideline": guideline})
    return compiled


def knowledge_markdown(knowledge: dict, registry: dict) -> str:
    statuses = {row["source"]: row for row in registry["sources"]}
    lines = ["# Review knowledge", "", f"Selected paths: {len(knowledge['selected_paths'])}. Pending decisions: {knowledge['summary']['pending_candidates']}.", "", "Triage skill metadata here. Read a source only when applying it or resolving an unclear boundary. Applied parents reveal direct reference routes on the next compile. Keep template fingerprints; edit status/reason and add exact rule spans. Groups may resolve pending template rows by source id.", "", "| ID | Source | Kind / paths | Status | Applicability metadata |", "| --- | --- | --- | --- | --- |"]
    def cell(value: object) -> str:
        return " ".join(str(value).split()).replace("|", "\\|")
    for source in knowledge["sources"]:
        row = statuses[source["path"]]
        details = source.get("description") or source["candidate_reason"]
        if source.get("dispatch_sources"):
            details += " Dispatch: " + ", ".join(source["dispatch_sources"])
        lines.append(f"| `{source['id']}` | `{cell(source['path'])}` | {source['kind']} / {len(source_paths(knowledge, source))} | {row['status']} | {cell(details)} |")
        for alias in source["aliases"] if source["kind"] == "instruction" else []:
            if alias["path"] != source["path"] or len(source["aliases"]) > 1:
                lines.append(f"| | Alias `{cell(alias['path'])}` | precedence {alias['precedence']} | | scope: {cell(', '.join(alias['scope']))} |")
        for binding in source.get("parent_bindings", []):
            for hint in binding["routing_hints"]:
                lines.append(f"| | From `{cell(binding['parent_skill'])}:{hint['line']}` | route | | {cell(hint['text'])} |")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--decisions", help="edited rules.template.json (paths or compact source ids)")
    args = parser.parse_args()
    try:
        repo, out = repo_root(), Path(args.out).resolve()
        manifest = read_json(out / "manifest.json")
        decisions = read_json(Path(args.decisions)) if args.decisions else None
        knowledge, registry = discover(repo, manifest, decisions)
        write_json(out / "knowledge.json", knowledge)
        write_json(out / "rules.template.json", registry)
        (out / "knowledge.md").write_text(knowledge_markdown(knowledge, registry), encoding="utf-8")
        if decisions is not None:
            write_json(out / "rules.json", registry)
    except (OSError, RuntimeError, ValueError) as error:
        sys.stderr.write(f"{error}\n")
        return 1
    summary = knowledge["summary"]
    print(f"knowledge -> {out / 'knowledge.md'}")
    print(f"{summary['instructions']} instructions + {summary['supplemental']} config/learnings + {summary['skills']} routers + {summary['skill_references']} references; {summary['pending_candidates']} pending")
    print(f"decision template -> {out / 'rules.template.json'}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
