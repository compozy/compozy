export const storyCompany = {
  name: "Northstar Pay",
  shortName: "Northstar",
  registryBaseUrl: "https://registry.northstarpay.internal/agh",
  hooksMarketplaceBaseUrl: "https://extensions.northstarpay.internal/agh",
  tagline: "Launch week for Northstar Pay Checkout across Brazil and Mexico",
} as const;

export const storyWorkspaceIds = {
  hq: "ws_launch_hq",
  risk: "ws_risk_ops",
  growth: "ws_growth_studio",
  platform: "ws_platform_control",
  finance: "ws_finance_command",
  support: "ws_merchant_success",
  product: "ws_product_studio",
} as const;

export const storyWorkspaceNames = {
  hq: "launch-hq",
  risk: "risk-ops",
  growth: "growth-studio",
  platform: "platform-control",
  finance: "finance-command",
  support: "merchant-success",
  product: "product-studio",
} as const;

export const storyWorkspacePaths = {
  hq: "/workspaces/northstar-pay/launch-hq",
  risk: "/workspaces/northstar-pay/risk-ops",
  growth: "/workspaces/northstar-pay/growth-studio",
  platform: "/workspaces/northstar-pay/platform-control",
  finance: "/workspaces/northstar-pay/finance-command",
  support: "/workspaces/northstar-pay/merchant-success",
  product: "/workspaces/northstar-pay/product-studio",
  sharedPolicies: "/workspaces/northstar-pay/shared/policies",
  sharedCampaigns: "/workspaces/northstar-pay/shared/campaigns",
  sharedLaunch: "/workspaces/northstar-pay/shared/launch-week",
  sharedAnalytics: "/workspaces/northstar-pay/shared/analytics",
} as const;

export const storyDefaultWorkspaceId = storyWorkspaceIds.hq;
export const storyDefaultWorkspaceName = storyWorkspaceNames.hq;

export const storyAgentNames = {
  cto: "cto-agent",
  cfo: "cfo-agent",
  product: "product-manager-agent",
  frontend: "frontend-engineer-agent",
  platform: "platform-engineer-agent",
  release: "release-manager-agent",
  marketing: "marketing-lead-agent",
  copywriter: "copywriter-agent",
  support: "support-lead-agent",
  fraud: "fraud-ops-agent",
  compliance: "compliance-review-agent",
} as const;

export const storySessionIds = {
  cto: "sess_cto_command",
  cfo: "sess_cfo_watch",
  product: "sess_product_brief",
  frontend: "sess_frontend_launch_qa",
  platform: "sess_platform_rollout",
  release: "sess_release_control",
  marketing: "sess_marketing_launch_copy",
  copywriter: "sess_copywriter_claims",
  support: "sess_support_swarm",
  fraud: "sess_fraud_watch",
  compliance: "sess_compliance_review",
} as const;

export const storyChannels = {
  launchWarRoom: "launch-war-room",
  execSignal: "exec-signal",
  financeWatch: "finance-watch",
  landingPage: "landing-page",
  supportSwarm: "support-swarm",
  riskOps: "risk-ops",
  merchantEscalations: "merchant-escalations",
  growthLaunch: "growth-launch",
  releaseControl: "release-control",
  partnerSync: "partner-sync",
} as const;

export const storyHeroNetworkChannel = storyChannels.launchWarRoom;

export const storyPeerIds = {
  local: "peer_northstar_launch",
  partner: "peer_partner_bank",
  finance: "peer_northstar_finance",
  growth: "peer_northstar_growth",
  support: "peer_northstar_support",
  frontend: "peer_northstar_frontend",
  cto: "peer_northstar_cto",
  creative: "peer_creative_studio",
} as const;

export const storyPeople = {
  primaryOperator: "sofia.mendes",
  cto: "helen.park",
  cfo: "tiago.alves",
  productLead: "maya.singh",
  riskLead: "marina.chen",
  growthLead: "rafael.costa",
  copyLead: "laura.ferreira",
  supportLead: "bruno.silva",
  frontendLead: "isabela.rossi",
  engineer: "davi.lima",
  financeManager: "camila.pereira",
} as const;

export const storySkillNames = {
  merchantEscalation: "merchant-escalation-handoff",
  launchCopy: "launch-copy-polish",
  frontendQa: "frontend-launch-qa",
  executiveBrief: "executive-brief-synth",
  financePrep: "burn-report-prep",
} as const;

export const storyCoordinatorAgentName = "launch-coordinator-agent";

export function storyWorkspaceSkillDir(
  name: string,
  workspacePath: string = storyWorkspacePaths.hq
) {
  return `${workspacePath}/.agh/skills/${name}`;
}
