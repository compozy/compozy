import type {
  SkillActionResponse,
  SkillExposeResponse,
  SkillExposeFailureResponse,
  SkillExposurePayload,
  SkillMarketplaceInstallPayload,
  SkillMarketplaceRemovePayload,
  SkillMarketplaceUpdatePayload,
  SkillPayload,
} from "../types";
import {
  storySkillNames,
  storyWorkspacePaths,
  storyWorkspaceSkillDir,
} from "@/storybook/fintech-scenario";

export const skillFixtures: SkillPayload[] = [
  {
    name: storySkillNames.executiveBrief,
    description:
      "Turn cross-functional launch traffic into a concise executive brief with risks, owners, and next steps.",
    source: "workspace",
    origin: "",
    enabled: true,
    activation: { active: true },
    dir: storyWorkspaceSkillDir(storySkillNames.executiveBrief),
    version: "1.2.0",
    exposures: [
      {
        target: "agents",
        path: `${storyWorkspacePaths.hq}/.agents/skills/${storySkillNames.executiveBrief}`,
        status: "healthy",
      },
    ],
    metadata: {
      tags: ["executive", "launch", "briefing"],
      downloads: 318,
    },
    provenance: {
      installed_at: "2026-04-17T16:40:00Z",
      precedence_tier: "workspace",
      registry: "workspace",
      slug: "workspace",
      version: "1.2.0",
      shadowed_by: [
        {
          detected_at: "2026-04-17T16:41:00Z",
          path: "/opt/compozy/skills/executive-brief-synth/SKILL.md",
          resolved_to_winner: false,
          tier: "marketplace",
        },
      ],
    },
  },
  {
    name: storySkillNames.launchCopy,
    description:
      "Polish launch headlines, CRM copy, pricing claims, and ad lines without violating the approved guardrails.",
    source: "workspace",
    origin: "agents",
    enabled: true,
    activation: { active: true },
    dir: storyWorkspaceSkillDir(storySkillNames.launchCopy, storyWorkspacePaths.growth),
    version: "1.0.3",
    metadata: {
      tags: ["marketing", "copy", "claims"],
      downloads: 284,
    },
    provenance: {
      precedence_tier: "workspace",
    },
  },
  {
    name: storySkillNames.frontendQa,
    description:
      "Run launch-surface QA for hero states, pricing banners, mobile breakpoints, and fallback banners.",
    source: "workspace",
    origin: "",
    enabled: true,
    activation: {
      active: false,
      reasons: [
        {
          gate: "requires_tools",
          code: "missing_tool",
          missing: ["compozy__browser_screenshot"],
          message: "gate requires_tools unmet: compozy__browser_screenshot",
        },
      ],
    },
    dir: storyWorkspaceSkillDir(storySkillNames.frontendQa, storyWorkspacePaths.product),
    version: "1.1.0",
    metadata: {
      tags: ["frontend", "qa", "launch"],
      downloads: 227,
    },
    provenance: {
      precedence_tier: "workspace",
      installed_from_extension: "launch-qa-pack",
    },
  },
  {
    name: storySkillNames.financePrep,
    description:
      "Prepare launch GMV, burn, and reserve snapshots for finance reviews and launch-room decisions.",
    source: "workspace",
    origin: "team-skills",
    enabled: true,
    activation: { active: true },
    dir: storyWorkspaceSkillDir(storySkillNames.financePrep, storyWorkspacePaths.finance),
    version: "0.9.4",
    metadata: {
      tags: ["finance", "gmv", "reporting"],
      downloads: 141,
    },
    provenance: {
      precedence_tier: "workspace",
      installed_from_extension: "launch-room",
    },
  },
  {
    name: storySkillNames.merchantEscalation,
    description:
      "Guide support and risk through launch-day merchant escalations with clear customer-safe next steps.",
    source: "marketplace",
    origin: "",
    enabled: false,
    activation: { active: true },
    dir: "/opt/compozy/skills/merchant-escalation-handoff",
    version: "0.8.2",
    metadata: {
      tags: ["support", "risk", "merchant"],
      downloads: 173,
    },
    provenance: {
      installed_at: "2026-04-17T15:00:00Z",
      precedence_tier: "marketplace",
      registry: "community",
      slug: "@community/merchant-escalation-handoff",
      version: "0.8.2",
    },
  },
];

export const skillShadowsFixtures = Object.fromEntries(
  skillFixtures.map(skill => {
    const winner = {
      detected_at: "2026-04-17T16:41:00Z",
      path: `${skill.dir}/SKILL.md`,
      resolved_to_winner: true,
      tier: skill.provenance?.precedence_tier ?? skill.source,
    };
    return [
      skill.name,
      {
        name: skill.name,
        winner,
        shadows: [winner, ...(skill.provenance?.shadowed_by ?? [])],
      },
    ];
  })
);

export const primarySkillFixture: SkillPayload = skillFixtures[0];

export const skillContentFixtures: Record<string, string> = {
  [storySkillNames.executiveBrief]:
    "# Executive Brief Synth\n\nSummarize blockers, owners, fallbacks, and launch readiness in four bullet points.\n",
  [storySkillNames.launchCopy]:
    "# Launch Copy Polish\n\nRewrite launch copy so pricing language stays approved and conversion-friendly.\n",
  [storySkillNames.frontendQa]:
    "# Frontend Launch QA\n\nVerify hero states, pricing banners, fallback banners, and mobile spacing before launch.\n",
  [storySkillNames.financePrep]:
    "# Burn Report Prep\n\nCompile launch GMV, burn, reserve exposure, and refund-risk notes for finance.\n",
  [storySkillNames.merchantEscalation]:
    "# Merchant Escalation Handoff\n\nPrepare a merchant-safe escalation summary with owner, ETA, and next update.\n",
};

export const skillActionFixture: SkillActionResponse = {
  ok: true,
};

export const skillMarketplaceInstallFixture: SkillMarketplaceInstallPayload = {
  name: "merchant-escalation-handoff",
  slug: "@community/merchant-escalation-handoff",
  status: "installed",
  hash: "sha256:fixture",
  path: "/opt/compozy/skills/merchant-escalation-handoff",
  registry: "clawhub",
  version: "0.9.0",
};

export const skillMarketplaceUpdateFixtures: SkillMarketplaceUpdatePayload[] = [
  {
    name: "merchant-escalation-handoff",
    slug: "@community/merchant-escalation-handoff",
    status: "updated",
    path: "/opt/compozy/skills/merchant-escalation-handoff",
    current_version: "0.8.2",
    latest_version: "0.9.0",
  },
];

/** The four reconciled expose states, one row each. */
export const skillExposuresFixture: SkillExposurePayload[] = [
  {
    target: "agents",
    path: "/Users/ana/.agents/skills/review-checklist",
    status: "healthy",
  },
  {
    target: "claude",
    path: "/Users/ana/.claude/skills/review-checklist",
    status: "missing",
  },
  {
    target: "agents",
    path: "/Users/ana/.agents/skills/deploy-runbook",
    status: "broken",
  },
  {
    target: "claude",
    path: "/Users/ana/.claude/skills/deploy-runbook",
    status: "foreign_conflict",
  },
];

export const skillExposeSuccessFixture: SkillExposeResponse = {
  name: storySkillNames.executiveBrief,
  results: [
    {
      target: "agents",
      ok: true,
      exposure: skillExposuresFixture[0],
    },
  ],
  rolled_back: false,
};

/** One target refused, the completed one compensated — the `expose_failed` envelope. */
export const skillExposePartialFailureFixture: SkillExposeFailureResponse = {
  error: {
    code: "expose_failed",
    message: "1 of 2 targets failed; completed targets rolled back",
  },
  name: storySkillNames.executiveBrief,
  results: [
    {
      target: "claude",
      ok: false,
      error: {
        code: "expose_name_conflict",
        occupied_by: "/Users/ana/.claude/skills/review-checklist",
      },
    },
    { target: "agents", ok: false, error: { code: "rolled_back" } },
  ],
  rolled_back: true,
};

export const skillMarketplaceRemoveFixture: SkillMarketplaceRemovePayload = {
  name: "merchant-escalation-handoff",
  slug: "@community/merchant-escalation-handoff",
  status: "removed",
  path: "/opt/compozy/skills/merchant-escalation-handoff",
};
