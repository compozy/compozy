/** Real docs content for `/docs/account/sign-in` (ADR-014 §4, zero em dashes). */
import type { TocItem } from "../docs/mdx";
import type { DocPage, NavGroup } from "../docs/nav";

export const SIGN_IN_DOC_TITLE = "Why CompozyOS asks you to sign in";
export const SIGN_IN_DOC_DESCRIPTION =
  "What Compozy Cloud stores, what stays on your computers, how offline mode works, and how to delete your account.";

export const SIGN_IN_DOC_TOC: readonly TocItem[] = [
  { id: "why-sign-in-is-required", text: "Why sign-in is required", depth: 2 },
  { id: "what-compozyos-stores", text: "What CompozyOS stores", depth: 2 },
  { id: "what-compozyos-never-stores", text: "What CompozyOS never stores", depth: 2 },
  {
    id: "working-offline-after-first-sign-in",
    text: "Working offline after first sign-in",
    depth: 2,
  },
  { id: "ci-and-headless-environments", text: "CI and headless environments", depth: 2 },
  { id: "deleting-your-account", text: "Deleting your account", depth: 2 },
];

export const SIGN_IN_DOC_MARKDOWN = `Official CompozyOS releases ask you to sign in with Google or GitHub on first run. Your agents, code, and sessions still run on your own computers, never on CompozyOS servers.

## Why sign-in is required

Sign-in solves three practical problems without tracking what you build:

1. **Counting real installations:** Knowing how many developers use CompozyOS helps fund and prioritize the runtime without embedding invasive analytics inside the daemon.
2. **Product news only with consent:** If you check **Email me product news and offers** on sign-in (unchecked by default), CompozyOS can email major releases and tips. Transactional security emails, like new device sign-in notices, do not require marketing opt-in.
3. **Personal plans and remote computers:** One account links your laptop, desktop, Linux servers, and phone so Free (1 remote computer) and Pro (unlimited computers and hosted routes) work cleanly across every device you own.

## What CompozyOS stores

Compozy Cloud stores only the metadata needed to identify your account and connect your machines:

| Category | Stored fields |
| --- | --- |
| **Account** | Verified email, display name, sign-in provider (Google or GitHub), provider subject ID |
| **Preferences** | Product news opt-in state, timestamp, and source |
| **Devices** | Device name, kind (desktop, CLI, browser), public key thumbprint, admin trust flag, last-seen time |
| **Computers** | Computer name, platform, CompozyOS version, route metadata |
| **Billing** | Stripe customer ID and Free/Pro subscription state |

## What CompozyOS never stores

Compozy Cloud never sees or persists your engineering work:

- Prompts, agent messages, reasoning traces, or session transcripts;
- Project files, git diffs, worktrees, or terminal output;
- Your Claude, OpenAI, Cursor, or third-party API keys and subscriptions.

Sessions run on your own hardware. On remote connections between your CompozyOS apps and computers, hosted-route traffic is encrypted end to end.

## Working offline after first sign-in

Your **first sign-in** requires an internet connection to complete Google or GitHub authentication. After your device is enrolled, CompozyOS caches your signed-in state locally in SQLite:

- Local sessions, Loops, tasks, worktrees, and terminals keep working offline.
- Temporary network drops or cloud outages never sign you out.
- Only an explicit rejection (revoking the device, unlinking the computer, or deleting the account) ends a session.

## CI and headless environments

For CI runners or headless automation where interactive browser sign-in is impractical, create a local-only account token from **Settings → Account → Account tokens** or the CLI:

\`\`\`bash
compozy account token create --name ci
\`\`\`

Pass the returned token in \`COMPOZY_ACCOUNT_TOKEN\`. Account tokens unlock local daemon use only and cannot link remote computers or change billing.

## Deleting your account

You can delete your account at any time from **Settings → Account → Delete account** or from the terminal:

\`\`\`bash
compozy account delete
\`\`\`

Deleting your account signs out every installation, unlinks your remote computers, cancels any active Pro subscription immediately (refunding your last payment in full if within 14 days), and removes your personal data within 30 days. **All local sessions, projects, and files stay on your computer.**

For full legal details, read the [Privacy Policy](/legal/privacy), [Terms of Service](/legal/terms), and [Refund Policy](/legal/refunds).`;

export const ACCOUNT_DOCS_GROUP: NavGroup = {
  id: "account",
  label: "Account & Computers",
  count: 5,
  items: [
    {
      t: "Account",
      c: [
        {
          t: "Why CompozyOS asks you to sign in",
          s: "account/sign-in",
          d: SIGN_IN_DOC_DESCRIPTION,
        },
        {
          t: "Plans and billing",
          s: "account/billing",
          d: "Free and Pro limits, upgrades, renewals, and refunds.",
        },
        {
          t: "Account tokens (CI & headless)",
          s: "account/tokens",
          d: "Unlock local use in CI with COMPOZY_ACCOUNT_TOKEN.",
        },
      ],
    },
    {
      t: "Computers",
      c: [
        {
          t: "Add a computer over SSH",
          s: "computers/ssh",
          d: "Install, link, and connect a Linux or macOS machine.",
        },
        {
          t: "Hosted routes and phone access",
          s: "computers/hosted-route",
          d: "Reach your computers from any network on Pro.",
        },
      ],
    },
  ],
};

export const SIGN_IN_DOC_PAGE: DocPage = {
  slug: "account/sign-in",
  title: SIGN_IN_DOC_TITLE,
  description: SIGN_IN_DOC_DESCRIPTION,
  maturity: null,
  group: ACCOUNT_DOCS_GROUP,
  trail: [ACCOUNT_DOCS_GROUP.items[0]!],
  node: ACCOUNT_DOCS_GROUP.items[0]!.c![0]!,
};
