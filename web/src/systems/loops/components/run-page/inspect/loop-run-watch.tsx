import { Radio } from "lucide-react";

import { Eyebrow } from "@compozy/ui";

import type { LoopWatchEventsState } from "../../../types";

export function LoopRunWatch({ watchEvents }: { watchEvents: LoopWatchEventsState }) {
  const cursors = Object.entries(watchEvents.cursors ?? {}).sort(([left], [right]) =>
    left.localeCompare(right)
  );
  return (
    <section className="border-t border-line-soft p-4" data-testid="loop-run-inspect-watch">
      <div className="mb-2 flex items-center gap-1.5">
        <Radio aria-hidden="true" className="size-3 text-info" />
        <Eyebrow className="text-subtle">Watch</Eyebrow>
      </div>
      <ul className="flex flex-col divide-y divide-line-soft">
        {watchEvents.subscriptions.map(subscription => (
          <li className="flex flex-wrap items-center gap-x-2 gap-y-1 py-2" key={subscription.kind}>
            <span className="font-mono text-mono-id text-fg-strong">{subscription.kind}</span>
            {subscription.filter ? (
              <span className="font-mono text-mono-id text-subtle">{subscription.filter}</span>
            ) : null}
          </li>
        ))}
      </ul>
      {cursors.length > 0 ? (
        <dl
          className="mt-2 grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-1 border-t border-line-soft pt-2"
          data-testid="loop-run-inspect-cursors"
        >
          {cursors.map(([name, cursor]) => (
            <div className="contents" key={name}>
              <dt className="font-mono text-mono-id text-subtle">{name}</dt>
              <dd className="font-mono text-mono-id text-fg-strong">{cursor}</dd>
            </div>
          ))}
        </dl>
      ) : null}
    </section>
  );
}
