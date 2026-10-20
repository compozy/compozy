# Prototyping host recipes

These are adaptable shapes, not templates. Follow the project's framework, versions, and conventions; verify every command in the actual repository.

## Reuse an existing host

- **Story-based hosts** (Storybook, Ladle, Histoire, React Cosmos): put prototypes in their own folder, title them under one prototype group, and render each as a full-screen story or fixture. Reuse global decorators for theme, router, data, and mocks. Exclude the folder from story test runners, visual-regression baselines, and documentation pages when those would treat prototypes as product components. The preview URL is the host's isolated canvas URL for the story, not its manager shell: for Storybook, `http://localhost:<port>/iframe.html?id=<story-id>&viewMode=story` rather than `/?path=/story/<story-id>`. The manager wraps the prototype in its own UI, which pollutes screenshots and rendered evidence.
- **Existing playground or examples app:** add one route or page per prototype in the app's existing routing style. Keep it development-only when the app ships to users.

## Playground inside an existing app

Use this when the app already compiles the library and its providers. Add a development-only route, for example `/__prototypes/$slug`, that imports prototype entries through the bundler's glob import (`import.meta.glob` in Vite, or the framework's equivalent) and renders the selected one. Guard it so production builds exclude it: an environment check alone may still bundle the code, so prefer a development-only route file or build exclusion supported by the framework.

## Minimal prototype app

Use this when no host exists or the library lives in a package without an app.

- Create a workspace package such as `apps/prototypes` with the same framework, bundler, and versions as the project. For React with Vite: `vite`, `@vitejs/plugin-react`, the project's Tailwind plugin when it uses Tailwind, and the library as a workspace dependency.
- One `index.html` and one entry that reads the slug from the URL and lazy-imports `src/designs/<slug>/index.tsx` through `import.meta.glob("./designs/*/index.tsx")`.
- A `providers.tsx` wrapping the prototype with the library's required providers. Keep app-state providers (auth, real data clients) out; use fixtures.
- Import the library's global stylesheet once. With Tailwind v4 in a separate package, add `@source` entries so the prototype folder and the library sources are scanned.
- Reuse TypeScript path aliases from the base configuration. Vite 8 can read them through `resolve.tsconfigPaths`; older versions need `vite-tsconfig-paths`.
- Add `dev` and `typecheck` scripts, and register the package with the workspace tooling (for example Turborepo) without adding it to production deploys.

## Projects without a component bundler

Static-site or server-rendered frameworks (for example Astro) can render React islands, but a prototype that will become an application is better hosted in a small React app next to them, using the shared component package. If the final surface is framework-native markup, keep the HTML target for that surface and record that decision.

## Rendered evidence for lint and review

The lint tool reads HTML under `docs/design/`. Library prototypes provide rendered evidence at `docs/design/<slug>/index.html`, refreshed after every change:

- **Export (preferred when available):** a build that emits one self-contained HTML file for a prototype, for example Vite with `vite-plugin-singlefile` and the prototype as the only entry. Record the command with a `{slug}` placeholder as `evidence`.
- **Snapshot:** open the preview URL with `open-design-browser`, wait for the prototype to render, and capture the DOM without scripts:

  ```bash
  agent-browser --session "od-<task-id>" eval "(() => { const root = document.documentElement.cloneNode(true); root.querySelectorAll('script').forEach((node) => node.remove()); root.querySelectorAll('[src],[href]').forEach((node) => { for (const name of ['src', 'href']) { const value = node.getAttribute(name); if (value && !value.startsWith('#') && !value.startsWith('data:')) node.setAttribute(name, new URL(value, location.href).href); } }); return '<!doctype html>' + root.outerHTML; })()"
  ```

  The script makes asset URLs absolute, because development servers emit root-relative paths (for example `/@fs/...`) that break in a saved file; those assets load only while the development server runs. Inline small images such as logos when the evidence must stand alone. Write the returned markup to the evidence file. Decode it first when the CLI prints a quoted JSON string. Development servers usually inject the compiled CSS as `<style>` elements, so the snapshot keeps the rendered styles; if styles are linked instead, inline them or use an export. A snapshot is lint and review evidence, not a standalone deliverable.

## Pitfalls

- Generated files, ignored directories, and unused-file checks can reject or hide prototype folders; record the exclusions you add.
- Tailwind v4 automatic detection, linters, and formatters skip files ignored by version control. Ignoring the prototype folder then needs an explicit `@source` for it and leaves typecheck as its only static check; prefer versioning prototypes unless the user asks otherwise, and record the trade-off.
- Do not copy library components into the prototype folder or restyle them locally to fake a missing variant; note the gap instead.
- Monorepos may have several candidate apps; choose the one that owns the library's styles and providers, and record why.
- Prefer loopback-only development servers and a fixed port, and record it in the preview URL.
