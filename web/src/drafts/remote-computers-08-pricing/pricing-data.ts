/** Copy and structured data for `/pricing` (ADR-014 §1, `_uiux.md` U11). */

export type BillingInterval = "year" | "month";

export const FREE_FEATURES: readonly string[] = [
  "Every local feature included",
  "One remote computer over LAN or SSH",
  "Unlimited projects and sessions on your own computers",
  "Runs your own agent CLIs and API keys",
];

export const PRO_FEATURES: readonly string[] = [
  "Unlimited remote computers",
  "Hosted route: reach your computers from any network (up to 10 at once)",
  "Phone and browser access",
  "Hosted webhooks",
];

export interface ComparisonRow {
  readonly label: string;
  readonly detail?: string;
  readonly free: string;
  readonly pro: string;
  readonly proEmphasis?: boolean;
}

export interface ComparisonGroup {
  readonly id: string;
  readonly title: string;
  readonly rows: readonly ComparisonRow[];
}

export const COMPARISON_GROUPS: readonly ComparisonGroup[] = [
  {
    id: "local",
    title: "Local runtime",
    rows: [
      {
        label: "Sessions, Loops, automations, tasks, and worktrees",
        free: "Included",
        pro: "Included",
      },
      {
        label: "Projects and sessions on your own computers",
        free: "Unlimited",
        pro: "Unlimited",
      },
      {
        label: "Agent CLIs, subscriptions, and API keys",
        detail: "Claude Code, Codex, Cursor, and every built-in integration.",
        free: "Bring your own",
        pro: "Bring your own",
      },
      {
        label: "Open-source runtime (MIT)",
        free: "Included",
        pro: "Included",
      },
    ],
  },
  {
    id: "remote",
    title: "Remote computers",
    rows: [
      {
        label: "Remote computers over LAN or SSH",
        free: "1 computer",
        pro: "Unlimited",
        proEmphasis: true,
      },
      {
        label: "Hosted route from any network",
        detail: "No router port forwarding or VPN setup.",
        free: "Not included",
        pro: "Up to 10 computers",
        proEmphasis: true,
      },
      {
        label: "Phone and browser access",
        detail: "Sessions, command approvals, and terminals from any browser.",
        free: "Not included",
        pro: "Included",
        proEmphasis: true,
      },
      {
        label: "Hosted webhooks",
        detail: "Public webhook endpoints routed to your computer.",
        free: "Not included",
        pro: "Included",
        proEmphasis: true,
      },
      {
        label: "Load balancing (Run on: Auto)",
        free: "Included",
        pro: "Included",
      },
    ],
  },
  {
    id: "account",
    title: "Account and privacy",
    rows: [
      {
        label: "Code, files, prompts, and transcripts",
        free: "Stay on your computers",
        pro: "Stay on your computers",
      },
      {
        label: "Offline use after first sign-in",
        free: "Included",
        pro: "Included",
      },
      {
        label: "Billing and refunds",
        free: "No card needed",
        pro: "14-day full refund",
      },
    ],
  },
];

export interface PricingFaqItem {
  readonly id: string;
  readonly question: string;
  readonly answer: readonly string[];
  readonly link?: { readonly label: string; readonly href: string };
}

export const PRICING_FAQ: readonly PricingFaqItem[] = [
  {
    id: "why-sign-in",
    question: "Why does Compozy ask me to sign in?",
    answer: [
      "Signing in with Google or GitHub links your installations to one personal account, manages your Free or Pro plan, and lets Compozy count active people without tracking your sessions.",
      "You only need an internet connection for the first sign-in. Local sessions and projects keep working offline after that, and product news emails are strictly opt-in.",
    ],
    link: { label: "Why Compozy asks you to sign in", href: "/docs/account/sign-in" },
  },
  {
    id: "what-stays-free",
    question: "What stays free?",
    answer: [
      "Every local feature stays free: durable sessions, always-on agents, Loops, automations, tasks, worktrees, extensions, and unlimited projects and sessions on your own computers.",
      "Free also includes one remote computer connected over LAN or SSH, with no time limit and no credit card required.",
    ],
  },
  {
    id: "when-pro-ends",
    question: "What happens when Pro ends?",
    answer: [
      "Your hosted routes stay reachable for 7 days in entitlement-only mode so you have time to renew, after which the cloud tunnels are removed.",
      "Your most recently used remote computer stays usable over LAN or SSH. Additional remote computers pause until you renew, nothing is unlinked, and all local sessions and files stay untouched.",
    ],
  },
  {
    id: "refunds",
    question: "What is the refund policy?",
    answer: [
      "You can request a full refund within 14 days of your first Pro payment, and within 14 days of any annual renewal. Email support@compozy.com from your account address and we issue it back to your original payment method.",
      "There are no pro-rata refunds after 14 days. Cancelling stops renewal and keeps Pro active until the end of the current period.",
    ],
    link: { label: "Read the refund policy", href: "/legal/refunds" },
  },
  {
    id: "cancel",
    question: "How do I cancel or switch between monthly and annual?",
    answer: [
      "Open Settings, go to Account, and choose Manage billing to open the Stripe Customer Portal. From there you can cancel renewal, switch between monthly and annual (Stripe prorates the difference), or update your payment method.",
      "Deleting your account while Pro is active cancels the subscription immediately and applies the 14-day refund rule automatically.",
    ],
  },
  {
    id: "code-privacy",
    question: "Does Compozy see my code or agent sessions?",
    answer: [
      "No. Sessions, prompts, transcripts, and project files run on your own computers and never cross Compozy Cloud.",
      "Compozy Cloud only handles sign-in, plan status, and connection handshakes between your devices. Hosted-route traffic between your Compozy apps and computers is encrypted end to end.",
    ],
  },
];
