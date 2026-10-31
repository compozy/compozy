import type { ComponentType, SVGProps } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  AntigravityLogo,
  ClaudeLogo,
  CursorLogo,
  GeminiLogo,
  GithubLogo,
  KimiLogo,
  LinearLogo,
  OpenAILogo,
  OpenCodeLogo,
  OpenRouterLogo,
  PiLogo,
  QwenLogo,
  SlackLogo,
  VercelLogo,
  XAILogo,
  ZAILogo,
} from "../index";

type LogoProps = Pick<SVGProps<SVGSVGElement>, "aria-label" | "className">;
type LogoComponent = ComponentType<LogoProps>;
type LogoGroup = "all" | "agents" | "integrations";

interface LogoGalleryProps {
  group?: LogoGroup;
}

const AGENT_LOGOS: Array<{ label: string; Logo: LogoComponent }> = [
  { label: "Antigravity", Logo: AntigravityLogo },
  { label: "Claude", Logo: ClaudeLogo },
  { label: "Cursor", Logo: CursorLogo },
  { label: "Gemini", Logo: GeminiLogo },
  { label: "Kimi", Logo: KimiLogo },
  { label: "OpenAI", Logo: OpenAILogo },
  { label: "OpenCode", Logo: OpenCodeLogo },
  { label: "OpenRouter", Logo: OpenRouterLogo },
  { label: "Pi", Logo: PiLogo },
  { label: "Qwen", Logo: QwenLogo },
  { label: "Vercel", Logo: VercelLogo },
  { label: "xAI", Logo: XAILogo },
  { label: "Z.ai", Logo: ZAILogo },
];

const INTEGRATION_LOGOS: Array<{ label: string; Logo: LogoComponent }> = [
  { label: "GitHub", Logo: GithubLogo },
  { label: "Linear", Logo: LinearLogo },
  { label: "Slack", Logo: SlackLogo },
];

function LogoSection({
  title,
  logos,
}: {
  title: string;
  logos: Array<{ label: string; Logo: LogoComponent }>;
}) {
  return (
    <section className="grid gap-4">
      <h2 className="font-mono text-eyebrow font-medium uppercase tracking-badge text-subtle">
        {title}
      </h2>
      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-line bg-line sm:grid-cols-3 md:grid-cols-4">
        {logos.map(({ label, Logo }) => (
          <div
            key={label}
            className="grid min-h-28 place-items-center gap-3 bg-canvas-soft p-4 text-center"
          >
            <Logo aria-label={`${label} logo`} className="size-8 text-fg" />
            <span className="font-mono text-badge font-medium uppercase tracking-badge text-muted-foreground">
              {label}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}

function LogoGallery({ group = "all" }: LogoGalleryProps) {
  const showAgents = group === "all" || group === "agents";
  const showIntegrations = group === "all" || group === "integrations";

  return (
    <div className="grid w-[min(960px,calc(100vw-2rem))] gap-8 rounded-lg border border-line bg-canvas p-6 text-fg">
      <div className="grid gap-2">
        <p className="font-mono text-eyebrow font-medium uppercase tracking-badge text-brand">
          Logo registry
        </p>
        <h1 className="text-xl font-medium">Agent and integration logos</h1>
        <p className="max-w-[62ch] text-sm leading-6 text-muted-foreground">
          Brand SVGs exported by `@compozy/ui/logos` for Compozy site and runtime surfaces.
        </p>
      </div>
      {showAgents ? <LogoSection title="Agent providers" logos={AGENT_LOGOS} /> : null}
      {showIntegrations ? (
        <LogoSection title="Integration services" logos={INTEGRATION_LOGOS} />
      ) : null}
    </div>
  );
}

const meta: Meta<typeof LogoGallery> = {
  title: "ui/Logos",
  component: LogoGallery,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Shared brand logo registry for agent providers and integration services consumed by Compozy public pages.",
      },
    },
  },
  argTypes: {
    group: {
      control: "select",
      options: ["all", "agents", "integrations"],
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Full logo registry used by the Compozy site.
 */
export const Default: Story = {
  args: {
    group: "all",
  },
};

/**
 * Agent-provider logos used by the supported agents section.
 */
export const AgentProviders: Story = {
  args: {
    group: "agents",
  },
};

/**
 * Brand logos used by integration sections.
 */
export const IntegrationServices: Story = {
  args: {
    group: "integrations",
  },
};
