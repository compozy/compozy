import Image from "next/image";
import { Database, ShieldCheck, Terminal } from "lucide-react";
import { LandingCodeBlock } from "./primitives/code-block";
import { FeatureCard } from "./primitives/feature-card";
import { SectionFrame } from "./primitives/section-frame";
import { Eyebrow } from "@agh/ui";

const FEATURES = [
  {
    icon: <Database className="size-4" />,
    eyebrow: "Sessions",
    title: "Durable sessions in SQLite",
    description:
      "Every session gets a per-session event DB plus an entry in the global catalog. Resume after a restart, re-read the full history, or fork a new session from any point.",
    cite: {
      href: "/runtime/core/sessions/lifecycle",
      label: "sessions lifecycle",
    },
  },
  {
    icon: <Terminal className="size-4" />,
    eyebrow: "Surfaces",
    title: "Three operator surfaces, one daemon",
    description:
      "CLI over a Unix socket. HTTP + SSE API on :2123. A React 19 web UI with ten feature modules. All read from the same state.",
    cite: { href: "/runtime/core/operations/daemon", label: "daemon surfaces" },
  },
  {
    icon: <ShieldCheck className="size-4" />,
    eyebrow: "Permissions",
    title: "Permission modes with an audit trail",
    description:
      "AGH enforces session permission modes, keeps workspace boundaries intact, and records every approval decision.",
    cite: { href: "/runtime/core/sessions/permissions", label: "permissions" },
  },
];

const RUNTIME_CODE = `agh daemon start
agh session new --cwd "$PWD" --agent general
agh session events <session-id> --follow
agh session resume <session-id>`;

export function RuntimeSection() {
  return (
    <SectionFrame className="relative" background="canvas" padY="lg">
      <div className="grid gap-12 lg:grid-cols-[minmax(0,360px)_1fr] lg:items-start lg:gap-16">
        <div className="h-full flex flex-col justify-between lg:sticky lg:top-24">
          <div>
            <Eyebrow className="text-accent">Runtime</Eyebrow>
            <h2 className="mt-3 text-site-subsection-title leading-tight font-normal tracking-tight text-fg">
              A daemon built for sessions, not chats.
            </h2>
            <p className="mt-4 max-w-[50ch] text-sm leading-relaxed text-muted">
              Start <code className="font-mono text-fg">agh daemon start</code>. Every agent run
              becomes a session with a durable event log, an SSE stream, resumable state, and one
              operator surface shared by the CLI, API, and web UI.
            </p>
          </div>
          <div className="absolute bottom-0 left-0 invisible lg:visible">
            <Image
              src="/images/runtime/illustration_1.png"
              alt="AGH daemon connecting CLI, API, and web UI surfaces to sessions, memory, skills, workspaces, and observability."
              width={848}
              height={636}
              decoding="async"
              sizes="424px"
              quality={90}
              className="max-w-106 select-none object-contain opacity-95"
            />
          </div>
        </div>

        <div className="flex flex-col gap-6">
          <div className="grid gap-4 sm:grid-cols-2">
            {FEATURES.map(f => (
              <FeatureCard
                key={f.eyebrow}
                icon={f.icon}
                eyebrow={f.eyebrow}
                title={f.title}
                description={f.description}
                cite={f.cite}
              />
            ))}
          </div>

          <LandingCodeBlock code={RUNTIME_CODE} caption="agh session" shell />
        </div>
      </div>
    </SectionFrame>
  );
}
