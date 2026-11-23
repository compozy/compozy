/** Real draft legal copy following ADR-014 §8 (zero em dashes). */
import type { TocItem } from "../docs/mdx";

export type LegalDocId = "terms" | "privacy" | "refunds";

export interface LegalDocument {
  readonly id: LegalDocId;
  readonly navLabel: string;
  readonly title: string;
  readonly route: string;
  readonly summary: string;
  readonly lastUpdated: string;
  readonly toc: readonly TocItem[];
  readonly markdown: string;
}

export const LEGAL_DOCUMENTS: Record<LegalDocId, LegalDocument> = {
  terms: {
    id: "terms",
    navLabel: "Terms",
    title: "Terms of Service",
    route: "/legal/terms",
    summary:
      "The terms that govern your Compozy account, the Compozy Cloud control plane, and Compozy Pro subscriptions.",
    lastUpdated: "November 23, 2026",
    toc: [
      { id: "entity-and-governing-law", text: "Entity and governing law", depth: 2 },
      { id: "eligibility-and-accounts", text: "Eligibility and accounts", depth: 2 },
      { id: "software-and-cloud-service", text: "Software and cloud service", depth: 2 },
      { id: "acceptable-use-and-suspension", text: "Acceptable use and suspension", depth: 2 },
      {
        id: "subscriptions-renewal-and-cancellation",
        text: "Subscriptions, renewal, and cancellation",
        depth: 2,
      },
      { id: "hosted-routes-and-fair-use", text: "Hosted routes and fair use", depth: 2 },
      { id: "no-sla-and-liability-limits", text: "No SLA and liability limits", depth: 2 },
      { id: "changes-and-contact", text: "Changes and contact", depth: 2 },
    ],
    markdown: `## Entity and governing law

These Terms of Service form a binding agreement between you and **Compozy, LLC**, a Delaware limited liability company (Delaware file number 10308898) with its business address at 2261 Market Street, San Francisco, CA 94114, United States ("Compozy"), governing your access to Compozy Cloud, your Compozy account, and Compozy Pro subscriptions.

These terms are governed by the laws of the State of Delaware, United States, without regard to conflict-of-law principles. Any dispute arising from these terms will be resolved in the state or federal courts located in Delaware, unless mandatory consumer protection laws in your country of residence require otherwise.

## Eligibility and accounts

You must be at least **16 years old** to create a Compozy account or use Compozy Cloud. Accounts are personal and intended for one individual. You are responsible for keeping your Google or GitHub login credentials, your enrolled devices, and any account tokens secure.

If you believe an unauthorized device has signed in to your account, revoke it immediately from **Settings → Account → Devices** or from the link in your new sign-in email notice.

## Software and cloud service

The CompozyOS daemon, CLI, and desktop application are released under the **MIT License**. Your rights to inspect, modify, and build the open-source code come from the MIT License in the repository.

Compozy Cloud is the hosted service operated by Compozy that handles account sign-in, device enrollment, plan entitlements, connection handshakes between your computers, and Pro hosted routes. Signing in with a Compozy account is required for official release builds.

## Acceptable use and suspension

You agree not to misuse Compozy Cloud or hosted routes. Prohibited behavior includes:

- exposing public websites, file-sharing services, proxies, or unlawful content through hosted routes;
- attempting to bypass rate limits, plan limits, authentication checks, or bot verification;
- interfering with other customers, probing cloud infrastructure, or running automated abuse traffic;
- violating applicable laws or third-party rights while using hosted routes or webhooks.

Compozy may immediately suspend an account, revoke active sessions, or take down a hosted route if it detects abuse, security threats, fraud, or a violation of these terms. When an account is suspended, Compozy sends a notice to the account email and does not delete local data stored on your computers.

## Subscriptions, renewal, and cancellation

Compozy Pro is billed in USD on a monthly (**USD 9.99/month**) or annual (**USD 79.99/year**) basis, plus applicable taxes calculated at checkout. **Stripe Managed Payments** acts as the merchant of record for all Pro purchases, handling tax calculation, collection, receipts, and payment processing.

Subscriptions renew automatically at the end of each billing period unless cancelled before the renewal date. You can cancel, switch between monthly and annual billing, or update your payment method anytime through **Manage billing** in Settings → Account. Cancelling stops future renewals and keeps Pro active until the end of your current paid period.

Refunds are governed by our [Refund Policy](/legal/refunds). Deleting your account while Pro is active cancels your subscription immediately and applies the 14-day refund rule when eligible.

## Hosted routes and fair use

Compozy Pro includes hosted routes for up to **10 computers at once** per account. If you have a legitimate personal need above 10 simultaneous hosted routes, write to [support@compozy.com](mailto:support@compozy.com).

To keep cloud resources healthy, a hosted-route tunnel for a computer that stays offline for **14 consecutive days** is removed automatically (Compozy emails you at day 10). Turning the computer back on provisions a fresh hosted route on demand. When a Pro subscription ends, existing hosted routes stay in entitlement-only mode for 7 days and are then deleted.

## No SLA and liability limits

Compozy Cloud and hosted routes are provided **"as is"** and **"as available"**, with no uptime service-level agreement (SLA). Compozy publishes live operational status at \`status.compozy.com\`, and local sessions on your own computer remain usable offline after your first sign-in.

To the maximum extent permitted by law, Compozy is not liable for indirect, incidental, special, consequential, or punitive damages, or for lost profits, revenue, or data. Compozy's total aggregate liability for any claim arising out of these terms or the service will not exceed the fees you paid to Compozy in the **12 months** before the event giving rise to the claim, or USD 50 if you have not paid any fees.

## Changes and contact

Compozy may update these terms as the product evolves. Material changes will be posted here with an updated date and emailed to active accounts before taking effect.

Questions about these terms? Contact [support@compozy.com](mailto:support@compozy.com), or write to Compozy, LLC, 2261 Market Street, San Francisco, CA 94114, United States.`,
  },

  privacy: {
    id: "privacy",
    navLabel: "Privacy",
    title: "Privacy Policy",
    route: "/legal/privacy",
    summary:
      "Exactly what Compozy Cloud stores, what never leaves your computers, how long data is kept, and who processes it.",
    lastUpdated: "November 23, 2026",
    toc: [
      { id: "who-we-are", text: "Who we are", depth: 2 },
      { id: "data-we-store", text: "Data we store", depth: 2 },
      { id: "data-we-never-store", text: "Data we never store", depth: 2 },
      { id: "purposes-and-legal-bases", text: "Purposes and legal bases", depth: 2 },
      { id: "retention-and-deletion", text: "Retention and deletion", depth: 2 },
      { id: "sub-processors", text: "Sub-processors", depth: 2 },
      { id: "cookies-and-analytics", text: "Cookies and analytics", depth: 2 },
      { id: "your-rights-and-lgpd-contact", text: "Your rights and LGPD contact", depth: 2 },
      { id: "international-transfers", text: "International transfers", depth: 2 },
    ],
    markdown: `## Who we are

This Privacy Policy explains how **Compozy, LLC** ("Compozy"), a Delaware limited liability company at 2261 Market Street, San Francisco, CA 94114, United States, collects, uses, and protects personal data when you sign in to CompozyOS, use Compozy Cloud, or purchase Compozy Pro.

For privacy requests or data-protection questions, write to [privacy@compozy.com](mailto:privacy@compozy.com).

## Data we store

Compozy Cloud stores only the account and connectivity metadata required to operate your account and link your computers:

- **Account identity:** your verified email address, display name, sign-in provider (Google or GitHub), and provider subject identifier.
- **Email consent:** whether you opted in to product news and offers, together with the timestamp and source of each change.
- **Devices:** device name, kind (desktop, CLI, or browser), public key thumbprint, admin trust flag, and last-seen timestamp.
- **Computers:** computer name, operating system and architecture, Compozy version, and route metadata (such as hosted-route hostname and link state).
- **Billing:** Stripe customer identifier, subscription plan, billing interval, and entitlement dates. Card numbers are handled solely by Stripe.
- **Security log:** sign-in notices (including coarse city and country at activation), revocations, and account lifecycle events.

## Data we never store

Compozy Cloud never sees, stores, or trains on your work:

- **No prompts or transcripts:** agent conversations stay in SQLite on the computer running the session.
- **No source code or files:** repositories, diffs, worktrees, and tool outputs stay on your machines.
- **No model API keys:** your Claude, OpenAI, Cursor, or other provider credentials stay on your computers.

## Purposes and legal bases

We process personal data for four purposes:

1. **Providing the service (contract performance):** authenticating your devices, enforcing Free and Pro entitlements, and connecting your computers.
2. **Security and abuse prevention (legitimate interest):** preventing unauthorized sign-ins, rate limiting, and investigating hosted-route abuse.
3. **Billing and tax compliance (legal obligation and contract):** processing subscriptions, receipts, refunds, and statutory tax records through Stripe.
4. **Product news and offers (consent):** sent only when you explicitly opt in. You can opt out in Settings → Account → Email or via the one-click unsubscribe link in every broadcast.

## Retention and deletion

- **Active accounts:** account, device, and computer records are kept while your account remains active.
- **Account deletion:** when you delete your account from Settings or \`compozy account delete\`, every installation is signed out immediately, active Pro subscriptions are cancelled, and personal data is erased from primary systems within **30 days**.
- **90-day tombstone:** a PII-free tombstone record (account ID only, no email or name) is kept for **90 days** in \`deleted_accounts\` so late offline refreshes receive an explicit \`account_deleted\` rejection instead of retrying indefinitely.
- **Operational logs and security events:** infrastructure logs rotate on short operational windows. \`security_events\` rows are retained for **1 year** for fraud and abuse auditing, and are stripped to account ID and aggregate counts once an account tombstone is written.

## Sub-processors

Compozy uses the following sub-processors to operate Compozy Cloud:

| Sub-processor | Role | Data handled |
| --- | --- | --- |
| **Cloudflare** | Edge routing, Workers, Containers, Tunnels, Turnstile, R2 encrypted backups | Request IP, bot verification tokens, encrypted tunnel traffic, encrypted database backups |
| **Neon** | Managed PostgreSQL database | Account, device, computer, subscription, and security event records |
| **Stripe** | Merchant of record, billing, tax, Customer Portal | Email, billing address, tax details, payment method, subscription state |
| **Resend** | Transactional email and opted-in product broadcasts | Email address, display name, transactional notices, opted-in audience list |
| **Google** | OAuth / OpenID Connect sign-in provider | Provider subject ID, verified email, display name (when chosen) |
| **GitHub** | OAuth sign-in provider | Provider user ID, verified primary email, display name (when chosen) |
| **Sentry** | Cloud error tracking | Server error traces and request metadata (no session content) |
| **Better Stack** | Uptime monitoring, alerting, and status page | Service health checks and operational incidents |

**Hosted-route encryption note:** on connections between Compozy desktop or CLI apps and your remote computers, Cloudflare Tunnels carry inner TLS traffic that is **encrypted end to end** between your devices; Cloudflare sees ciphertext only. When you access a computer directly from a mobile or desktop web browser, browser TLS terminates at Cloudflare before reaching your computer over the tunnel.

## Cookies and analytics

Pages served from \`cloud.compozy.com\` set **only strictly necessary cookies** required to complete OAuth sign-in, protect against CSRF, and maintain your authenticated billing session. Because no tracking or advertising cookies are set, no cookie consent banner is needed on \`cloud.compozy.com\`, and **analytics scripts are never loaded on \`cloud.compozy.com\`**.

## Your rights and LGPD contact

Depending on where you live (including under the **LGPD** in Brazil, the **GDPR** in the EU/UK, and US state privacy laws), you have the right to access, correct, export (JSON), or delete your personal data, and to withdraw marketing consent at any time.

- Delete your account anytime in **Settings → Account → Delete account** or via \`compozy account delete\`.
- Request a full JSON export or exercise any privacy right by emailing [privacy@compozy.com](mailto:privacy@compozy.com). We respond within **15 days**.
- **LGPD data-protection contact (Encarregado / DPO):** [privacy@compozy.com](mailto:privacy@compozy.com), Compozy, LLC, 2261 Market Street, San Francisco, CA 94114, United States.

## International transfers

Compozy, LLC operates in the United States, and our sub-processors may process account metadata in the United States and other regions where they maintain infrastructure. Where required by law (including under the GDPR and LGPD), transfers rely on standard contractual clauses or equivalent legal transfer safeguards.`,
  },

  refunds: {
    id: "refunds",
    navLabel: "Refunds",
    title: "Refund Policy",
    route: "/legal/refunds",
    summary:
      "Clear, plain rules for Compozy Pro refunds, renewals, cancellations, and account deletion.",
    lastUpdated: "November 23, 2026",
    toc: [
      { id: "14-day-full-refund-window", text: "14-day full refund window", depth: 2 },
      { id: "after-14-days-and-cancellations", text: "After 14 days and cancellations", depth: 2 },
      { id: "deleting-your-account-while-on-pro", text: "Deleting your account while on Pro", depth: 2 },
      { id: "switching-between-monthly-and-annual", text: "Switching between monthly and annual", depth: 2 },
      { id: "how-to-request-a-refund", text: "How to request a refund", depth: 2 },
    ],
    markdown: `## 14-day full refund window

Compozy offers a **full refund on request within 14 days** of:

- your **first Compozy Pro payment** (monthly or annual); and
- any **annual Pro renewal** payment.

This policy also satisfies the 14-day digital service withdrawal right for customers in the European Union and the United Kingdom. Refunds are issued in full to your original payment method through Stripe, including any tax collected at checkout.

## After 14 days and cancellations

After the 14-day window passes, payments are non-refundable and we do not issue pro-rata refunds for unused days or months.

You can cancel your subscription anytime from **Settings → Account → Manage billing** (Stripe Customer Portal). Cancelling stops the next renewal and keeps all Pro features active until the end of the billing period you already paid for.

When Pro ends, your hosted routes remain in entitlement-only mode for 7 days so you can renew without losing your setup, and your most recently used remote computer stays usable on Free over LAN or SSH.

## Deleting your account while on Pro

An active Pro subscription never blocks you from deleting your account. When you delete your account from Settings or \`compozy account delete\`:

- Compozy cancels your Pro subscription immediately in Stripe;
- if your last payment falls within the **14-day refund window**, Compozy refunds it in full automatically;
- if your last payment is older than 14 days, no refund applies and Pro ends immediately;
- all local sessions, projects, and files on your computers stay untouched.

## Switching between monthly and annual

You can switch between monthly (USD 9.99/month) and annual (USD 79.99/year) plans anytime inside the Stripe Customer Portal (**Manage billing**). Stripe automatically calculates and applies the prorated credit or charge for the remainder of your current billing period.

## How to request a refund

Email [support@compozy.com](mailto:support@compozy.com) from the email address on your Compozy account (or include your Stripe receipt number). Once processed, Stripe returns the funds to your original payment method within 5 to 10 business days depending on your bank.`,
  },
};
