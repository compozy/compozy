/** The one create screen's fixtures and rules: nothing here renders. */
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime/components/runtime-selector";

import type { AgentAppearance } from "../persistent-agents-shared/agent-state";
import { EPHEMERAL_AGENTS, PERSISTENT_AGENTS } from "../persistent-agents-shared/fixtures";

export type AgentMode = "on-demand" | "stays-on";
/** A project name, `@user` for Everywhere, or `later` for no place yet. */
export type Place = string;

export const PLACE_LATER = "later";
export const PLACE_EVERYWHERE = "@user";
export const ACTIVE_LIMIT = 10;

export interface Template {
  id: string;
  label: string;
  body: string;
}

export const TEMPLATES: readonly Template[] = [
  {
    id: "issues",
    label: "Watch issues",
    body: "Keep an eye on new issues.\n\n- Say what's blocked and who can unblock it\n- Suggest the next step\n- Stay quiet when nothing changed",
  },
  {
    id: "review",
    label: "Review code",
    body: "Read each change before it goes in.\n\n- Say what could break, and why\n- Point to the exact place\n- Skip style opinions",
  },
  {
    id: "releases",
    label: "Keep releases moving",
    body: "Follow the next release until it's out.\n\n- List what's still missing\n- Flag anything that slips\n- Tell me when it's ready",
  },
  {
    id: "summary",
    label: "Sum up the week",
    body: "Every Friday, tell me what happened.\n\n- What shipped\n- What's stuck\n- What needs a decision from me",
  },
];

export const PROVIDERS: RuntimeProviderOption[] = [
  { id: "claude", name: "Claude", runtime_provider: "claude", harness: "acp" },
  { id: "codex", name: "Codex", runtime_provider: "codex", harness: "acp" },
];

function model(id: string, provider: string, name: string, featured = false): RuntimeModelOption {
  return {
    id,
    provider,
    name,
    featured,
    efforts: ["low", "medium", "high"],
    default_effort: "medium",
    availability: "live",
    curated: true,
    supports_tools: true,
  };
}

export const MODELS: RuntimeModelOption[] = [
  model("claude-opus-5-5", "claude", "Claude Opus 5.5", true),
  model("claude-sonnet-5-5", "claude", "Claude Sonnet 5.5"),
  model("gpt-6-astra", "codex", "GPT-6 Astra", true),
];

export const DEFAULT_MODEL: RuntimeSelectorValue = {
  provider: "claude",
  model: "claude-opus-5-5",
  reasoning_effort: "",
};

export interface Draft {
  name: string;
  description: string;
  instructions: string;
  appearance: AgentAppearance;
  mode: AgentMode;
  place: Place;
  runtime: RuntimeSelectorValue;
  shortName?: string;
}

export const EMPTY_DRAFT: Draft = {
  name: "",
  description: "",
  instructions: "",
  appearance: { avatar: "generated", seed: "new-agent" },
  mode: "on-demand",
  place: "compozy",
  runtime: DEFAULT_MODEL,
};

const TAKEN = new Set([...PERSISTENT_AGENTS, ...EPHEMERAL_AGENTS].map(agent => agent.name));

export function shortNameFor(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export type NameIssue = { kind: "taken"; suggestion: string } | { kind: "no-letters" } | null;

/** The spec's slug rules, spoken about the name: the slug itself stays under Advanced. */
export function nameIssue(name: string, shortName?: string): NameIssue {
  if (name.trim() === "") return null;
  const slug = shortName?.trim() || shortNameFor(name);
  if (slug === "") return { kind: "no-letters" };
  if (!TAKEN.has(slug)) return null;
  return { kind: "taken", suggestion: `${name.trim()} 2` };
}

export function placeLabel(place: Place) {
  if (place === PLACE_EVERYWHERE) return "Everywhere";
  if (place === PLACE_LATER) return "Decide later";
  return place;
}

/** Fixture padding so every picker is reviewed at the size the brief asks for: 50 projects. */
export const MORE_PROJECTS = [
  "acme-billing",
  "acme-mobile",
  "acme-web",
  "analytics",
  "api-gateway",
  "atlas",
  "auth-service",
  "blog",
  "brand-site",
  "checkout",
  "ci-runners",
  "compliance",
  "customer-portal",
  "data-pipeline",
  "design-tokens",
  "docs",
  "edge-proxy",
  "email-service",
  "experiments",
  "feature-flags",
  "growth",
  "helpdesk",
  "infra",
  "ios-app",
  "kiosk",
  "landing",
  "legal-ops",
  "marketing-site",
  "metrics",
  "mobile-sdk",
  "notifications",
  "onboarding",
  "partner-api",
  "payments",
  "pricing",
  "reports",
  "search",
  "security",
  "storybook",
  "support-tools",
  "terraform",
  "translations",
  "video-studio",
  "warehouse",
  "webhooks",
  "website",
  "zapier-app",
] as const;

export function nextSeed(seed: string) {
  const [base = seed, count = "0"] = seed.split("~");
  return `${base}~${Number(count) + 1}`;
}
