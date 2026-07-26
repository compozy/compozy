export type AgentId = "DISCOVERY" | "CODER" | "REVIEWER" | "OPS" | "QA" | "DEPLOYER";

export type ToolIcon = "read" | "diff" | "deploy" | "test" | "net";

export interface AgentBeat {
  id: AgentId;
  labelStart: number;
  tool?: {
    start: number;
    doneAt: number;
    icon: ToolIcon;
    label: string;
    summary: string;
  };
  reply: {
    text: string;
    start: number;
    duration: number;
  };
  timestamp: string;
}

export interface Conversation {
  session: { name: string };
  agents: AgentBeat[];
  chromeIn: { start: number; end: number };
  chromeOut: { start: number; end: number };
  bodyOut: { start: number; end: number };
}

export const FPS = 30;
export const DURATION_IN_FRAMES = 450;
export const COMPOSITION_WIDTH = 720;
export const COMPOSITION_HEIGHT = 720;

export const CONVERSATION: Conversation = {
  session: {
    name: "mesh · incident-payments",
  },
  agents: [
    {
      id: "DISCOVERY",
      labelStart: 18,
      tool: {
        start: 24,
        doneAt: 52,
        icon: "net",
        label: "Mesh",
        summary: "2 peers joined",
      },
      reply: {
        text: "reviewer-03 and ops-bot active. Routing to incident-payments.",
        start: 56,
        duration: 32,
      },
      timestamp: "09:41",
    },
    {
      id: "CODER",
      labelStart: 96,
      tool: {
        start: 102,
        doneAt: 128,
        icon: "test",
        label: "Run",
        summary: "tests/auth/*.ts",
      },
      reply: {
        text: "auth-service broken on main. Verbose logs attached.",
        start: 132,
        duration: 32,
      },
      timestamp: "09:41",
    },
    {
      id: "REVIEWER",
      labelStart: 174,
      tool: {
        start: 180,
        doneAt: 208,
        icon: "diff",
        label: "Diff",
        summary: "PR #482 · auth/jwt.ts",
      },
      reply: {
        text: "Regression is the JWT claim shape. Patching now.",
        start: 212,
        duration: 30,
      },
      timestamp: "09:42",
    },
    {
      id: "OPS",
      labelStart: 252,
      tool: {
        start: 258,
        doneAt: 286,
        icon: "deploy",
        label: "Rollback",
        summary: "staging → last-green",
      },
      reply: {
        text: "Staging rolled back. ETA 40s on the restore.",
        start: 290,
        duration: 28,
      },
      timestamp: "09:42",
    },
    {
      id: "QA",
      labelStart: 326,
      tool: {
        start: 332,
        doneAt: 360,
        icon: "test",
        label: "Smoke",
        summary: "staging suite",
      },
      reply: {
        text: "Smoke green on rollback. Greenlight the patch.",
        start: 364,
        duration: 28,
      },
      timestamp: "09:43",
    },
    {
      id: "DEPLOYER",
      labelStart: 398,
      reply: {
        text: "Promoting feat/jwt-hotfix to prod. Receipts inbound.",
        start: 404,
        duration: 26,
      },
      timestamp: "09:43",
    },
  ],
  chromeIn: { start: 0, end: 14 },
  chromeOut: { start: 438, end: 450 },
  bodyOut: { start: 432, end: 446 },
};

export function activeAgentAt(frame: number, conv: Conversation): AgentId {
  let current: AgentId = conv.agents[0].id;
  for (const a of conv.agents) {
    if (frame >= a.labelStart) current = a.id;
  }
  return current;
}
