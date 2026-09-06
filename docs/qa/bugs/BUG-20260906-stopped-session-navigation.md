# BUG-20260906-stopped-session-navigation: Search and trail fail after the session stops

- **Status:** fixed — public stopped-session re-walk passed
- **Impact:** Data-Unavailable
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-14 read a finished transcript
- **Scenarios:** ET-web-session-transcript-calm-grammar, RT-session-context-rebuild
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

The real handbook transcript remains readable after daemon restart, but public `session search` and `session outline` fail with `event recorder does not expose the materialized transcript projection: navigation reads`. The task08 navigation methods existed on writable and read-only databases, but the production read-only pool lease omitted their delegation. Active-session parity tests did not exercise the stopped-session wrapper.

The fix implements NavigationReader on the pooled lease and forwards both operations to its owned reader, preserving missing-capability refusal. The canonical real-pool lifecycle integration case fails before the repair and verifies search/outline after the writer closes. The existing query and cursor semantics remain unchanged.

Evidence: .cache/sessions-qa-navigation-pool-red.log. Public re-walk passed after daemon PID9550 restarted: handbook search/outline succeed, and the 3,023-entry incident history returns all 1,510 sent messages through outline and matches incident0000 outside the latest page. Evidence: integrated lab evidence/handbook-outline-reloaded.json, handbook-search-reloaded.json, navigation-outline-reloaded.json, navigation-search-reloaded.json, navigation-history-audit.json. Canonical real-pool regression + rewind passthrough passed with race in .cache/sessions-qa-navigation-pool-green.log (1.686s).
