# Performance

Read this when shipping any user-facing surface that has not been profiled, when the page is sluggish or janky, when Core Web Vitals regress, or when adding a heavy dependency to a hot route. Performance is not a separate phase — it is a surface contract.

## The Floor — Core Web Vitals

The user does not care about your bundler. They care about the screen drawing on time. These are non-negotiable for any production route.

| Metric                              | Floor     | What it measures                                                       |
| ----------------------------------- | --------- | ---------------------------------------------------------------------- |
| LCP (Largest Contentful Paint)      | < 2.5s    | Hero / above-fold content fully drawn                                  |
| INP (Interaction to Next Paint)     | < 200ms   | Worst-case input latency in the session                                |
| CLS (Cumulative Layout Shift)       | < 0.1     | Unexpected layout movement after first paint                           |
| TTFB (Time to First Byte)           | < 800ms   | Server response start                                                  |
| FCP (First Contentful Paint)        | < 1.8s    | First text/image painted                                               |
| Total Blocking Time (TBT, lab only) | < 200ms   | Long-task aggregate that pushes INP into the red                       |

INP replaced FID in March 2024 and is the harder one to hit. Most failures here are React re-renders or expensive event handlers, not network.

## The 80ms Perceived Performance Threshold

Anything under 80ms feels instantaneous. Anything over needs a visual acknowledgment or it reads as broken.

- < 80ms — no spinner, no skeleton, no "Loading…" — silence is fine, the eye does not register
- 80–300ms — micro feedback: button press state, optimistic update, inline shimmer
- 300ms–1s — explicit indicator: skeleton in the exact shape of the result, or a determinate progress bar
- 1–3s — explain *why*: skeleton + secondary text ("Fetching workflow runs…")
- > 3s — block with a real explanation and a cancel affordance; the user is now actively waiting

The number-one mistake here is showing a spinner for a 60ms call. It introduces flicker. The fix is to gate the spinner behind a 100ms delay so fast responses skip it entirely.

```ts
// Show skeleton only if the request takes > 100ms
let timer: ReturnType<typeof setTimeout>;
const showSkeleton = () => setLoading(true);
timer = setTimeout(showSkeleton, 100);

fetch(...).finally(() => {
  clearTimeout(timer);
  setLoading(false);
});
```

## Skeleton vs Spinner vs Optimistic vs Blur-Up

Different waits demand different signals. Picking the wrong one is a UX failure as real as a layout bug.

| Wait type                          | Signal                            | Why                                                                          |
| ---------------------------------- | --------------------------------- | ---------------------------------------------------------------------------- |
| First page load, known shape       | **Skeleton** in exact final shape | Reserves space, eliminates CLS, reads as "structure is here, content coming" |
| Submit / mutation                  | **Optimistic update**             | Update UI immediately, reconcile on response, rollback on error              |
| Image / video                      | **Blur-up / LQIP**                | Tiny base64 placeholder unblurs as the full asset arrives                    |
| Indeterminate wait, no known shape | **Spinner** (delayed 100ms)       | Last resort — communicates work without committing to a shape                |
| Long determinate process           | **Progress bar** with percentage  | Anything > 3s where you can measure                                          |

Skeletons must match the final content's dimensions exactly. A skeleton at the wrong height shifts the layout when content arrives — that is the CLS metric tanking in real time.

## Font Loading — Zero-CLS Strategy

Web fonts are the largest single source of CLS in production. The fix is fallback-metric matching, not preload-everything.

```css
@font-face {
  font-family: "Inter";
  src: url("/fonts/inter-variable.woff2") format("woff2-variations");
  font-weight: 100 900;
  font-display: swap;
}

@font-face {
  font-family: "InterFallback";
  src: local("Arial");
  size-adjust: 107%;
  ascent-override: 90%;
  descent-override: 22%;
  line-gap-override: 0%;
}

body {
  font-family: "Inter", "InterFallback", system-ui, sans-serif;
}
```

`size-adjust` + `ascent-override` + `descent-override` + `line-gap-override` make the fallback occupy the same vertical space as the real font, so the swap is invisible. Generate the override values with [Fontaine](https://github.com/unjs/fontaine) or [Capsize](https://seek-oss.github.io/capsize/).

Other rules:
- `font-display: swap` — always. Never `block`. Never `optional` unless the brand can truly tolerate fallback rendering.
- Preload only the fonts above the fold (`<link rel="preload" as="font" crossorigin>`) — preloading everything kills the bandwidth budget.
- Use variable fonts when the brand needs > 2 weights. One file, all weights, smaller than two static files.
- Never load 3+ font families. Two is the working ceiling: one for UI, one for code.

## Image Optimization

| Rule                                                | Why                                                              |
| --------------------------------------------------- | ---------------------------------------------------------------- |
| `width` + `height` attributes (or `aspect-ratio`)   | Reserves space before the byte arrives — zero CLS                |
| AVIF > WebP > JPEG (with fallback)                  | AVIF beats JPEG by ~50%, WebP by ~30% at equal quality           |
| `loading="lazy"` on everything below the fold       | Defer off-screen image fetch                                     |
| `decoding="async"`                                  | Decode off the main thread                                       |
| `<picture>` with `<source media>` for art direction | Different crops at different breakpoints                         |
| `srcset` + `sizes` for resolution switching         | Browser picks the right pixel density                            |
| `fetchpriority="high"` on LCP image only            | Tell the browser this is the hero — never on more than one image |

Next.js `<Image>`, Astro `<Image>`, SvelteKit `enhanced-img`, Vite `imagetools` — use the framework's optimizer. Hand-rolling responsive images correctly is harder than it looks.

## Motion Performance

Motion that drops frames is worse than no motion. Hard rules:

- Animate **only** `transform` and `opacity`. These are the compositor properties — they do not trigger layout or paint.
- Never animate `width`, `height`, `top`, `left`, `margin`, `padding`. They trigger layout on every frame.
- Never animate `box-shadow`, `filter`, `background-color` on more than a single small element. They are paint-heavy.
- Honor `prefers-reduced-motion: reduce`. ~35% of adults over 40 have vestibular sensitivity. Animation is consent-based.
- Use `will-change: transform` with discipline. It promotes the element to its own compositor layer — useful for one animating element, catastrophic on a list of 200.

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

For full motion patterns, easings, and library-specific recipes, read `motion-patterns.md`.

## Bundle Hygiene

A 2MB JS bundle is a 2MB indictment of every decision that led to it. Apply these in order.

1. **Route-level code splitting.** Each route loads only its own code. The framework usually does this; verify with `next build` / `vite build --report` / `astro build`.
2. **Dynamic import for heavy dependencies.** Charts, rich-text editors, code editors, PDF viewers, video players — `const Editor = dynamic(() => import('./Editor'))`. These should never ship on first paint.
3. **Tree-shake aggressive.** Use named imports (`import { debounce } from 'lodash-es'`), never default-import the whole library. Verify with bundle analyzer.
4. **Audit `framer-motion` placement.** It is ~30kb gzipped. If motion is not central to a route, lean on Tailwind transitions or CSS keyframes.
5. **Audit `lodash`, `moment`, `date-fns` weight.** Prefer `date-fns` with named imports, or `Intl.DateTimeFormat` if requirements allow. Moment is on maintenance and ships a kitchen sink.
6. **Audit icon libraries.** `lucide-react` and `@heroicons/react` are tree-shakeable. `react-icons` is *not* — using one icon imports the whole pack.
7. **No barrel files in hot paths.** A re-exporting `index.ts` defeats tree-shaking in some bundler configs. Import from the source file.

## Server / Edge Considerations

- Cache aggressively on the edge. Static pages, static API responses, anything safe to cache should sit on the CDN.
- Stream responses where possible. React Server Components, Suspense boundaries, and HTTP streaming let the user see the first byte while the server keeps working.
- Defer non-critical work to the next tick (`requestIdleCallback`, post-load) or move it server-side entirely.
- Database query in the response path is almost always the bottleneck — N+1 queries, missing indexes, unbounded list endpoints. Profile before guessing.

## Verification Recipes

Performance is observable. Untested = unknown.

- **Lighthouse CI** in the PR pipeline. Block merge if any Core Web Vital regresses past the floor.
- **WebPageTest** for cold-cache, throttled-network, real-device profiling. Synthetic Lighthouse on local fiber is not reality.
- **CrUX / RUM** (Real User Monitoring) for the live distribution. Field data is the only truth; lab numbers are advisory.
- **React DevTools Profiler** for INP investigation. Find the long render that pushes interaction past 200ms.
- **Chrome DevTools Performance panel** with CPU throttling 4x and Network "Slow 4G" — this matches a real mid-range Android, which is more honest than your M2.
- **`performance.mark()` + `performance.measure()`** for custom flow timing (e.g. "first paint of a workflow run after Run button").
- **Bundle analyzer** (`@next/bundle-analyzer`, `rollup-plugin-visualizer`) before every deploy that touches dependencies.

## Common AI Failures

These appear constantly in LLM-generated code. Catch them on review.

- `import _ from 'lodash'` — imports the entire library to use one function
- `<img src="..." />` without `width`/`height`/`aspect-ratio` — CLS on every load
- `setTimeout(() => setSomething(), 300)` to "make it smooth" — racy, fragile, often masks a real bug
- Animating `width`, `top`, `height`, `margin` — paint storm on every frame
- `useEffect` doing heavy work on every render with no dependency array
- Loading `framer-motion` on a landing page that has one fade-in — replace with CSS
- `react-icons` for a single icon — replace with `lucide-react` or inline SVG
- `<a href="..." onClick={preventDefault + navigate}>` — defeats prefetch + the browser's native navigation optimizations
- Polling a REST endpoint on a 1s interval instead of streaming / WebSocket / SSE

## Pre-Ship Performance Gate

If any of these are red, the surface is not shippable:

- [ ] LCP < 2.5s on mid-range Android over throttled 4G
- [ ] INP < 200ms p75 in the worst interaction on the page
- [ ] CLS < 0.1 after fonts and images settle
- [ ] No image without dimensions or `aspect-ratio`
- [ ] No animation on layout-triggering properties
- [ ] `prefers-reduced-motion: reduce` honored
- [ ] Bundle size delta is justified — every new dependency over 10kb is questioned
- [ ] Heavy components (editor, chart, viewer) dynamically imported
- [ ] No barrel-import of icon packs or utility libraries

## Strong Opinion

> Performance is the only feature every user uses. Skip it, and every other feature ships behind a wait.

## Sources

- web.dev — Core Web Vitals, INP migration, font loading metrics
- Addy Osmani — *Image Optimization* (book), *Speed at Scale* talks
- Vercel Engineering — *Optimizing Largest Contentful Paint*
- Jake Archibald — *In The Loop* (event loop + INP)
- Sentry / DataDog / Vercel Analytics — RUM dashboards as ground truth
- web.dev/articles/lcp, web.dev/articles/inp, web.dev/articles/cls
