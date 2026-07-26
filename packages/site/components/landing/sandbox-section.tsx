import { Cloud, RefreshCcw, Terminal } from "lucide-react";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";
import { Eyebrow } from "@agh/ui";

const CARDS = [
  {
    icon: <Terminal className="size-4" />,
    eyebrow: "Local",
    title: "Run on the host when isolation is not needed",
    description:
      "The local backend keeps the same tool path and filesystem expectations while still recording sandbox metadata on the session.",
  },
  {
    icon: <Cloud className="size-4" />,
    eyebrow: "Daytona",
    title: "Move a workspace into a remote sandbox",
    description:
      "Daytona profiles create or reattach cloud sandboxes from an image or snapshot, then connect AGH through the provider tool host.",
  },
  {
    icon: <RefreshCcw className="size-4" />,
    eyebrow: "Sync",
    title: "Control how files move",
    description:
      "Profiles choose sync mode, persistence, runtime root, network policy, and provider-specific lifecycle settings.",
  },
];

function SandboxDiagram() {
  return (
    <div
      className="relative min-h-90 overflow-hidden rounded-diagram border border-line bg-canvas-soft p-5"
      aria-label="AGH sandbox lifecycle diagram"
      role="img"
    >
      <div className="absolute inset-x-5 top-5 h-px bg-line" />
      <div className="grid h-full min-h-80 grid-rows-[auto_1fr_auto] gap-5">
        <div className="grid grid-cols-3 gap-3">
          {["workspace", "agh daemon", "sandbox provider"].map(label => (
            <div key={label} className="rounded-md border border-line bg-canvas px-3 py-2">
              <Eyebrow className="text-muted">{label}</Eyebrow>
            </div>
          ))}
        </div>

        <div className="grid grid-cols-[1fr_auto_1fr_auto_1fr] items-center gap-3">
          <DiagramNode title="Host files" lines={["root_dir", "add_dirs", ".agh/config.toml"]} />
          <div className="h-px w-10 bg-accent" />
          <DiagramNode title="Session" lines={["prepare", "sync", "stop"]} active />
          <div className="h-px w-10 bg-accent" />
          <DiagramNode title="Daytona" lines={["image/snapshot", "runtime_root", "tool host"]} />
        </div>

        <div className="grid grid-cols-3 gap-3">
          {["sandbox_id", "sandbox_ref", "sandbox.exec"].map(label => (
            <code
              key={label}
              className="rounded-mono-badge border border-line bg-rail px-3 py-2 text-center font-mono text-eyebrow text-muted"
            >
              {label}
            </code>
          ))}
        </div>
      </div>
    </div>
  );
}

function DiagramNode({
  title,
  lines,
  active = false,
}: {
  title: string;
  lines: string[];
  active?: boolean;
}) {
  return (
    <div
      className={`rounded-md border p-4 ${
        active ? "border-accent bg-accent/8" : "border-line bg-canvas"
      }`}
    >
      <p className="text-sm font-medium text-fg">{title}</p>
      <ul className="mt-3 space-y-1">
        {lines.map(line => (
          <li key={line} className="font-mono text-eyebrow text-subtle">
            {line}
          </li>
        ))}
      </ul>
    </div>
  );
}

export function SandboxSection() {
  return (
    <SectionFrame background="surface" padY="lg" ariaLabel="Sandbox execution">
      <SectionHeader
        align="start"
        eyebrow="Sandbox"
        title="Run agents away from the host filesystem."
        description="Keep a session local when that is enough, or bind a workspace to a Daytona sandbox with explicit sync, lifecycle, and provider metadata."
      />

      <div className="mt-12 grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.9fr)] lg:items-stretch">
        <div className="grid gap-4">
          {CARDS.map(card => (
            <article key={card.eyebrow} className="rounded-md border border-line bg-canvas p-5">
              <div className="flex items-center gap-3">
                <span className="inline-flex size-8 items-center justify-center rounded-mono-badge border border-line text-accent">
                  {card.icon}
                </span>
                <Eyebrow className="text-accent">{card.eyebrow}</Eyebrow>
              </div>
              <h3 className="mt-4 text-base font-medium leading-snug text-fg">{card.title}</h3>
              <p className="mt-3 text-sm leading-relaxed text-muted">{card.description}</p>
            </article>
          ))}
        </div>

        <SandboxDiagram />
      </div>
    </SectionFrame>
  );
}
