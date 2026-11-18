// Vite-only entry: `?worker` bundles Pierre's highlight worker as a same-origin
// module worker (the daemon CSP allows `worker-src 'self' blob:`). It is reached as
// `@compozy/ui/lib/code-highlight-worker`, outside the root barrel,
// so non-Vite consumers of `@compozy/ui` never load it.
import CodeHighlightWorker from "@pierre/diffs/worker/worker.js?worker";

export function createCodeHighlightWorker(): Worker {
  return new CodeHighlightWorker();
}
