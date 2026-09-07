---
title: Spend less time repeating development and verification work
type: highlight
---

Repeated development startup and verification reuse successful evidence when source, generated artifacts, toolchain, and build settings still match. Development entry points avoid unchanged generation, and Air avoids relinking an unchanged daemon while preserving atomic publication and failed-build recovery. Explicit generation and drift checks remain available.

Web tests use the existing worker bound with a threads pool; migration fixtures reuse an empty historical prefix while still exercising real upgrades. Active lint-plugin tests are included in normal verification. The full-checkptr audit now installs its Bun dependencies and uses the existing eight-way Go partitioner without reducing instrumentation or timeouts.

Marketplace MCP configuration no longer waits for unrelated model-catalog discovery. Provider and catalog changes still reconcile normally.

PRs: [#536](https://github.com/compozy/compozy/pull/536), [#556](https://github.com/compozy/compozy/pull/556).
