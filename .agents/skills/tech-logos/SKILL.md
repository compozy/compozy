---
name: tech-logos
description: "Reuse or add official company, provider, and social-platform logos, including placeholders and landing/auth/integration UIs. Use the existing brand inventory. Excludes custom logos, generic icon libraries, and marketing imagery."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Tech Logos

Reuse the repository's existing official brand assets before fetching or installing anything. Inspect the logo index and the component's real props; do not assume every logo supports `variant` or `mode`.

1. Find the logo in the existing inventory. In Compozy, check `packages/ui/src/logos/index.ts` and import from `@compozy/ui/logos`.
2. When it is missing, use an authoritative brand asset and the project's established SVG/component pattern. Preserve provenance, brand geometry, accessible labeling, and appropriate light/dark behavior.
3. Add only the requested asset to the shared logo directory and public export. Extend the relevant story when it demonstrates a new supported state; no all-brand bundles or duplicated app-local logos.
4. Use an external registry such as Elements only when the task calls for that source or the project already adopts it. Inspect the proposed files and use the repository package manager. A missing logo does not require opening a third-party issue.

In other repositories, locate the equivalent inventory and package path instead of creating a Compozy-shaped folder tree.
