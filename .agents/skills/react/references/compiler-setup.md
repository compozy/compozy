# React Compiler setup and adoption

## Adoption sequence

Lint first, compiler second. The lint rules run without the compiler installed, so this order is zero-risk and surfaces every violation before any output changes.

1. **Survey** — `npx react-compiler-healthcheck@latest` for the compiled-component ratio, StrictMode usage, and incompatible libraries.
2. **Lint** — remove `eslint-plugin-react-compiler` (a dead end, last published as `19.1.0-rc.2`) and install `eslint-plugin-react-hooks@latest`.
3. **Fix violations**, prioritizing `preserve-manual-memoization` and `immutability`.
4. **Pin exactly** — `npm i -D --save-exact babel-plugin-react-compiler@1.0.0`. Official advice when test coverage is thin, because compiler versions change output.
5. **Enable on a subset** via Babel `overrides` or `compilationMode: 'annotation'`.
6. **Verify** with the DevTools ✨ badge and Performance Tracks.
7. **Only then** consider removing legacy memos, and only after confirming no effect dependencies rely on reference stability.

## Lint configuration

`eslint-plugin-react-hooks` **7.1.1+**. Version 6 is deprecated; v7 slimmed to two presets and enables all compiler rules by default. `recommended-latest-legacy` and `flat/recommended` were removed.

```js
// eslint.config.js — flat config
import reactHooks from "eslint-plugin-react-hooks";
import { defineConfig } from "eslint/config";

export default defineConfig([
  reactHooks.configs.flat.recommended,
  // or reactHooks.configs.flat["recommended-latest"] for experimental compiler rules
]);
```

Legacy config (ESLint < 9): `extends: ["plugin:react-hooks/recommended"]`.

Pin to ≥ 7.1.1 — earlier releases had real regressions: 6.1.0 broke `recommended` under ESLint 9 `defineConfig()`, 7.0.1 had module-resolution and typing regressions, and under `noUncheckedIndexedAccess` the `configs.flat.recommended` type is wrong in 7.0.0.

`exhaustive-deps` has an `additionalHooks` regex option; the docs say to use it "very sparingly, if at all".

If you migrate linting to Oxlint, note that `config`, `gating`, `incompatible-library`, and `preserve-manual-memoization` have no ESLint-React equivalents.

## Install

```bash
npm install --save-dev --save-exact babel-plugin-react-compiler@latest
```

React 17 and 18 additionally need `react-compiler-runtime@latest`; output then imports from `react-compiler-runtime` rather than `react/compiler-runtime`. `Cannot find module 'react/compiler-runtime'` means `target` and the installed React version disagree.

## Configuration keys

| Key | Type | Default |
| --- | --- | --- |
| `compilationMode` | `'infer' \| 'annotation' \| 'syntax' \| 'all'` | `'infer'` |
| `target` | `'17' \| '18' \| '19'` | `'19'` |
| `panicThreshold` | `'none' \| 'critical_errors' \| 'all_errors'` | `'none'` |
| `sources` | `Array<string> \| ((filename: string) => boolean) \| null` | excludes `node_modules` |
| `logger` | `Logger \| null` with `logEvent(filename, event)` | `null` |
| `gating` | `{ source: string, importSpecifierName: string } \| null` | `null` |
| `environment` | `Partial<EnvironmentConfig>` | defaults |

Details that are easy to get wrong:

- `compilationMode` has **four** values. `'syntax'` compiles only Flow `component`/`hook` syntax and will not work with TypeScript. `'all'` is not recommended — it compiles non-React utility functions.
- `panicThreshold` defaults to **`'none'`**, meaning problematic components are skipped rather than failing the build. The Configuration overview page's phrasing implies otherwise; `'none'` is correct.
- `target` takes strings only — `'18'`, never `18` or `'18.2.0'`.
- `sources` returns a **boolean**.

## Per-bundler setup

### Babel

```js
// babel.config.js
module.exports = {
  plugins: ["babel-plugin-react-compiler"], // must run first in the pipeline
};
```

### Next.js

Top-level `reactCompiler`, not `experimental`:

```ts
// next.config.ts
const nextConfig: NextConfig = { reactCompiler: true };

// opt-in mode
const nextConfig: NextConfig = { reactCompiler: { compilationMode: "annotation" } };
```

Requires `babel-plugin-react-compiler` installed. Next.js runs an SWC pre-pass so the Babel compiler only touches files containing JSX or hooks.

### Vite

Two shapes; picking the wrong one silently disables the compiler.

```js
// @vitejs/plugin-react v6+ — Babel was replaced by oxc, so Babel is a separate plugin
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import babel from "@rolldown/plugin-babel";

export default defineConfig({
  plugins: [react(), babel({ presets: [reactCompilerPreset()] })],
});

// older @vitejs/plugin-react
react({ babel: { plugins: ["babel-plugin-react-compiler"] } });
```

The old `react({ babel: { … } })` form **silently no-ops** on plugin v6 / Vite 8.

### React Router

```js
import babel from "vite-plugin-babel";

export default defineConfig({
  plugins: [
    reactRouter(),
    babel({
      filter: /\.[jt]sx?$/, // required for TS; the default filter is jsx-only
      babelConfig: {
        presets: ["@babel/preset-typescript"],
        plugins: [["babel-plugin-react-compiler", {}]],
      },
    }),
  ],
});
```

### Others

Expo SDK 54+, Vite, and Next.js templates ship the compiler enabled for new apps. Webpack uses the community `react-compiler-webpack` loader; Rspack and Rsbuild have native docs. React Native goes through Metro's Babel config — restart with `npx expo start --clear` after editing `babel.config.js`.

## Incremental adoption

```js
// 1. Directory scoping with Babel overrides
overrides: [
  { test: "./src/modern/**/*.{js,jsx,ts,tsx}", plugins: ["babel-plugin-react-compiler"] },
];

// 2. compilationMode: 'annotation' — opt in per component with "use memo"

// 3. Runtime gating for A/B testing
["babel-plugin-react-compiler", {
  gating: { source: "ReactCompilerFeatureFlags", importSpecifierName: "isCompilerEnabled" },
}];
```
