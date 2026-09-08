# Spec contracts

Product content states outcome, users, scope, and acceptance; implementation terms are acceptable when they describe an actual public contract or fixed constraint. `references/spec-part1-checks.md` and the read-only `.agents/skills/cy-spec-preflight/scripts/check-spec-part1-leak.py <spec_path>` are advisory terminology aids, not lexical approval gates.

Technical content makes applicable interfaces, data ownership, storage decisions, and safety invariants concrete. Read `references/spec-six-markers.md` for applicability. The read-only `.agents/skills/cy-spec-preflight/scripts/check-spec-markers.py <spec_path>` checks common outcome/boundary markers; add `--require <marker>` for each additional contract actually changed. Review the meaning, since heuristic matches are not proof of completeness.

Changed user state needs a lossless migration; public surfaces follow the SD-013 deprecation ladder; only internal code carries no-compat hard cuts. Record delete targets and affected config/extension/agent/Web/Docs surfaces once. Preserve `_dx.md` and UI-bearing `_uiux.md` contracts when applicable; `_tests.md` owns concrete test cases. ADRs may narrow an accepted goal only with the user's recorded decision.
