# Page Outline: YAML to SDK Migration
- **Purpose:** Guide teams through migrating from YAML configurations to the SDK. Sources: tasks/prd-sdk/06-migration-guide.md, task_53.md (basics), task_54.md (advanced), task_55.md (failure modes).
- **Audience:** Existing YAML users planning migration.
- **Prerequisites:** Read Core YAML guide and SDK overview.
- **Key Sections:**
  1. Migration decision matrix (from 06-migration-guide.md §"When to Migrate").
  2. Preparation checklist (codebase readiness, dependency updates) referencing 06-migration-guide.md §"Prerequisites".
  3. Step-by-step migration flow (mapping YAML resources to builders) referencing task_53.md deliverable table.
  4. Hybrid operation patterns (SDK + YAML) referencing 06-migration-guide.md §"Hybrid" and 02-architecture.md deployment topologies.
  5. Validation & rollout (testing guidance cross-link, troubleshooting) referencing task_54.md advanced checks.
- **Cross-links:** Core YAML schema docs, CLI deployment doc, troubleshooting page.
- **Examples Strategy:** Link to migration-focused examples `sdk/examples/09_yaml_parity.go` and `10_hybrid_runner.go` in 05-examples.md.
- **Notes:** Include callout about context-first requirements and `config.ContextWithManager` from _techspec.md.
