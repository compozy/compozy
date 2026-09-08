# Applicable Spec Markers

Outcome/scope and architectural boundaries apply to every technical spec. Other markers apply only to changed contracts; no fabricated section is needed to pass a heuristic.

| Marker ID | When applicable |
| --- | --- |
| `1-mvp-boundary` | Scope and observable outcome (always) |
| `2-architectural-boundaries` | Owning surface and dependencies (always) |
| `3-go-interface-signatures` | A Go interface/type contract changes; other languages use their actual signatures and semantic review |
| `4-data-model-rationale` | Data/config fields change |
| `5-side-table-vs-json` | Queryable state has a real storage/ownership choice |
| `6-numbered-invariants` | Concurrency, ownership, leases, or permissions change |

Run the read-only `.agents/skills/cy-spec-preflight/scripts/check-spec-markers.py <spec_path>` and add `--require <marker-id>` for applicable additional markers. Its matches are advisory structure evidence: inspect the actual contract and explain a false positive/negative rather than inventing text to satisfy a regex. A single meaningful invariant can be sufficient.

Changed state/public interfaces also need SD-013 compatibility and the owning impact analysis. File References points to the concrete contract inputs. Use the existing authoring review once; no separate six-marker approval round.
