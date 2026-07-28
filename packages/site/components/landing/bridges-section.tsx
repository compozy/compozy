import { ArrowRight } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Eyebrow, Pill } from "@compozy/ui";
import {
  DiscordLogo,
  GithubLogo,
  GoogleChatLogo,
  LinearLogo,
  MicrosoftTeamsLogo,
  SlackLogo,
  TelegramLogo,
  WhatsAppLogo,
} from "@compozy/ui/logos";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

type Bridge = {
  id: string;
  name: string;
  logo: ReactNode;
};

// Source of record: content/runtime/core/bridges/index.mdx and extensions/bridges/. All eight
// providers exist in-tree; none is embedded in the released binary, which is what the caveat
// under the grid states. Keep this list and that sentence aligned with the docs page.
const BRIDGES: Bridge[] = [
  { id: "slack", name: "Slack", logo: <SlackLogo aria-hidden className="size-7" /> },
  { id: "discord", name: "Discord", logo: <DiscordLogo aria-hidden className="size-7" /> },
  { id: "telegram", name: "Telegram", logo: <TelegramLogo aria-hidden className="size-7" /> },
  { id: "whatsapp", name: "WhatsApp", logo: <WhatsAppLogo aria-hidden className="size-7" /> },
  {
    id: "teams",
    name: "Microsoft Teams",
    logo: <MicrosoftTeamsLogo aria-hidden className="size-7" />,
  },
  {
    id: "google-chat",
    name: "Google Chat",
    logo: <GoogleChatLogo aria-hidden className="size-7" />,
  },
  { id: "github", name: "GitHub", logo: <GithubLogo aria-hidden className="size-7 text-fg" /> },
  {
    id: "linear",
    name: "Linear",
    logo: <LinearLogo aria-hidden className="size-7" mode="dark" />,
  },
];

export function BridgesSection() {
  return (
    <SectionFrame background="surface" padY="lg" className="border-b border-line">
      <SectionHeader
        align="start"
        eyebrow="Bridges"
        title="Your users work in these channels. Your agents can meet them there."
        description="Webhooks in, sessions out. Responses stream back to the original thread. No serverless glue, no second runtime, the bridge adapter runs inside the daemon."
      />

      <ul className="mt-12 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {BRIDGES.map(bridge => (
          <li key={bridge.id}>
            <article className="group relative flex h-full flex-col items-start gap-3 rounded-diagram border border-line bg-canvas p-5 transition-colors hover:border-accent/35">
              <div className="flex items-center justify-between self-stretch">
                <div className="flex size-10 items-center justify-center">{bridge.logo}</div>
                <Pill mono tone="accent">
                  in-tree
                </Pill>
              </div>
              <p className="text-sm font-medium text-fg">{bridge.name}</p>
              <Eyebrow className="text-subtle">bridge:{bridge.id}</Eyebrow>
            </article>
          </li>
        ))}
      </ul>

      <p className="mt-5 max-w-[64ch] text-small-body leading-relaxed text-subtle">
        All eight providers ship in-tree. They are not embedded in the released binary: build and
        install the provider from a trusted source checkout before configuring a bridge instance.
      </p>

      {/* Flow strip */}
      <div className="mt-10 rounded-diagram border border-line bg-canvas p-6">
        <div className="flex items-center justify-between border-b border-line pb-4">
          <Eyebrow className="text-subtle">How a bridge delivers a session</Eyebrow>
          <Pill mono tone="accent">
            inside the daemon
          </Pill>
        </div>
        <div className="mt-6 grid grid-cols-1 items-center gap-4 md:grid-cols-[auto_1fr_auto_1fr_auto_1fr_auto]">
          <FlowNode title="Platform" sub="slack / discord / tg" />
          <FlowArrow label="webhook" />
          <FlowNode title="compozy daemon" sub="verify · route" highlight />
          <FlowArrow label="session" />
          <FlowNode title="Agent" sub="claude / codex / ..." />
          <FlowArrow label="stream" />
          <FlowNode title="Thread reply" sub="streamed updates" />
        </div>
      </div>

      <p className="mt-6 max-w-[64ch] text-small-body leading-relaxed text-subtle">
        Every bridge is a workspace-scoped adapter. One platform message maps to one durable
        session, so a user thread keeps its context across restarts.
      </p>

      <div className="mt-5 flex flex-col gap-3 sm:flex-row">
        <Link
          href="/runtime/core/bridges/setup"
          className="inline-flex items-center gap-2 rounded-lg border border-accent px-4 py-2 text-small-body font-medium text-accent transition-colors hover:bg-accent-tint"
        >
          Set up a bridge
          <ArrowRight aria-hidden="true" className="size-4" />
        </Link>
        <Link
          href="/runtime/core/bridges/adding-a-bridge"
          className="inline-flex items-center gap-2 rounded-lg border border-line px-4 py-2 text-small-body font-medium text-fg transition-colors hover:border-accent/35 hover:text-accent"
        >
          Build a bridge adapter
          <ArrowRight aria-hidden="true" className="size-4" />
        </Link>
      </div>
    </SectionFrame>
  );
}

function FlowNode({
  title,
  sub,
  highlight = false,
}: {
  title: string;
  sub: string;
  highlight?: boolean;
}) {
  return (
    <div
      className={`rounded-md border px-3 py-2 text-center ${
        highlight ? "border-accent bg-accent-tint" : "border-line bg-canvas-soft"
      }`}
    >
      <p className={`text-xs font-medium ${highlight ? "text-accent" : "text-fg"}`}>{title}</p>
      <p className="font-mono text-badge tracking-mono text-subtle">{sub}</p>
    </div>
  );
}

function FlowArrow({ label }: { label: string }) {
  return (
    <div className="flex flex-col items-center justify-center">
      <Eyebrow className="text-subtle">{label}</Eyebrow>
      <ArrowRight aria-hidden className="size-4 text-accent" />
    </div>
  );
}
