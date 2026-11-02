/**
 * Closed-selector CPU diagnostic: 40 fresh SSR mounts with 2,500 curated models.
 * Run from the repo root with the pinned Bun version on each compared revision:
 * bun web/e2e/scripts/benchmark-runtime-selector.tsx
 * Includes React SSR and initial controller projection, without React Compiler or DOM work.
 * Results are mount CPU evidence, not browser interaction latency or network timing.
 */
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { useRuntimeSelector } from "../../src/systems/runtime/components/runtime-selector/use-runtime-selector";

const providers = [{ id: "opencode", name: "OpenCode" }];
const models = Array.from({ length: 2500 }, (_, index) => ({
  id: `upstream-${index % 25}/model-${index}`,
  name: `Model ${index}`,
  provider: "opencode",
  curated: true,
  efforts: [],
  availability: "live" as const,
}));
const value = { provider: "opencode", model: models[0]!.id, reasoning_effort: "" as const };
const onChange = () => {};
let projectedRows = 0;
function SelectorMount() {
  const controller = useRuntimeSelector({ providers, models, value, onChange });
  return createElement(
    "span",
    { "data-rows": controller.listModel.flatRows.length },
    controller.selectedModel?.name
  );
}
const samples: number[] = [];
for (let sample = 0; sample < 23; sample++) {
  let markup = "";
  const started = performance.now();
  for (let index = 0; index < 40; index++)
    markup = renderToStaticMarkup(createElement(SelectorMount));
  const elapsed = performance.now() - started;
  projectedRows = Number(markup.match(/data-rows="(\d+)"/)?.[1]) * 40;
  if (sample >= 3) samples.push(elapsed);
}
samples.sort((a, b) => a - b);
console.log(
  JSON.stringify({
    runtime: process.versions.bun,
    fixture: "40 closed selector mounts, 2500 curated OpenCode models",
    samples: samples.length,
    medianMs: (samples[9]! + samples[10]!) / 2,
    minMs: samples[0],
    maxMs: samples.at(-1),
    projectedRows,
  })
);
