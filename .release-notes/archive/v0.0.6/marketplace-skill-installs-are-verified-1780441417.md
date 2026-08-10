---
title: Marketplace skill installs are verified end-to-end
type: fix
---

Marketplace skill installs are now verified end-to-end. After downloading and writing a skill, CompozyOS confirms the runtime can actually discover it as an enabled marketplace skill with matching provenance, instead of reporting success for an install that would never resolve. Installs that are disabled, shadowed by a higher-precedence skill of the same name, missing provenance, or resolved to a different slug now fail with a clear, terminal error and remediation guidance across `compozy skill install` and the daemon API. Use `compozy skill where <name>` to inspect the winning source before retrying.
