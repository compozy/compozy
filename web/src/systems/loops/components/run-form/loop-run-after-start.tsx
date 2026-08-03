import { Hand, MonitorPlay, RotateCcw } from "lucide-react";

import { LoopRailSection } from "../loop-rail-section";

const ROWS = [
  {
    id: "run-page",
    icon: MonitorPlay,
    title: "You land on the run page",
    body: "Live nodes, events and controls for this run.",
  },
  {
    id: "retry",
    icon: RotateCcw,
    title: "Failures retry on their own",
    body: "Steps that keep failing are set aside in quarantine, never lost — you can requeue them.",
  },
  {
    id: "control",
    icon: Hand,
    title: "You can always step in",
    body: "Pause, resume, cancel or kill from the run page. Cancel finishes in-flight work; kill stops it now.",
  },
] as const;

/**
 * What happens after Start run — three destinations the runtime actually provides,
 * each one a control that already exists on the run page (task_08). Nothing here
 * promises a repair path the daemon does not expose.
 */
export function LoopRunAfterStart() {
  return (
    <LoopRailSection
      data-testid="loop-run-after-start"
      defaultOpen
      gist="live run page · you stay in control"
      title="After you start"
    >
      <ul className="flex flex-col gap-3 px-4 py-3.5">
        {ROWS.map(row => (
          <li className="flex items-start gap-2.5" key={row.id}>
            <row.icon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
            <div className="flex min-w-0 flex-col gap-0.5">
              <span className="text-form-label font-medium text-fg">{row.title}</span>
              <span className="text-form-hint leading-relaxed text-subtle">{row.body}</span>
            </div>
          </li>
        ))}
      </ul>
    </LoopRailSection>
  );
}
