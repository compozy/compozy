---
id: RT-site-pricing
area: RT
title: Pricing page, legal links, browser Pro checkout with promotion code, and Why sign in docs
persona: Eva
journey: J-buy-pro-from-site
expected: Visitors can inspect /pricing across desktop and mobile viewports (dark theme pinned on compozy.com; OS light/dark preference checked), switch between annual and monthly billing, open /legal/terms, /legal/privacy, and /legal/refunds from the header/footer/FAQ, complete a Stripe test-mode purchase with a promotion code through Get Pro to cloud.compozy.com/billing/upgrade, land on the Pro success page, sign in to the app showing Pro, and follow U0's Why sign in? link to /docs/account/sign-in.
entry_points: https://compozy.com/; https://compozy.com/pricing; https://compozy.com/legal/terms; https://compozy.com/legal/privacy; https://compozy.com/legal/refunds; https://compozy.com/docs/account/sign-in; https://cloud.compozy.com/billing/upgrade; desktop/web U0 sign-in screen
qa_status: untested
bug_ids:
fix_status: pending
retest_status: pending
fix_commits:
evidence:
last_report:
overlaps: RT-computers-billing; RT-sign-in-first-run
---

## Preconditions

- `packages/site` deployed to staging/preview or running locally with generated content.
- Staging Compozy Cloud configured with Stripe test mode, Google/GitHub test OAuth apps, and an
  active Stripe promotion code (for example a 3-month launch/migration coupon).
- A clean release-gated CompozyOS app installation (`pre_account_install = false`) ready for first
  sign-in.

## Walkthrough steps

1. **Landing & navigation:**
   - Open `/` on desktop and on a narrow phone viewport (390×844), verifying both OS light and dark
     color-scheme preferences (`compozy.com` pins dark styling without contrast regressions).
   - Confirm **Pricing** is visible in the site header and footer, and **Terms**, **Privacy**, and
     **Refunds** are visible in the footer.
   - Scroll to **Your agents, on all your computers** and switch across **Computers**, **Run on**,
     and **Phone access**. Confirm **Read the Computers docs** leads to `/docs/computers` and
     **See pricing** leads to `/pricing`.
   - Scroll to the comparison table and confirm **Remote computers and phone access** lists
     CompozyOS as `Free + Pro` alongside Synara's `Paid plan`, and the final row reads
     `Runs on your computers. Open source (MIT).`
   - Scroll to the install section and confirm **Desktop**, **Installer**, **Homebrew**, and **npm**
     are offered (no **Go** / `go install`), and Step 2 is **Sign in** (`compozy account login`).

2. **`/pricing` states:**
   - Open `/pricing` on desktop and on a phone viewport (confirm Pro stacks above Free on narrow
     viewports).
   - Verify Annual is selected by default, showing `USD 6.67` `/month, billed yearly`, `USD 79.99/year`,
     and `Save 33%`, with **Get Pro** pointing to
     `https://cloud.compozy.com/billing/upgrade?interval=year`.
   - Confirm the tax line reads `Prices in USD. Plus applicable taxes, calculated at checkout.`
   - Toggle to **Monthly** and verify `USD 9.99` `/month` and **Get Pro** updating to
     `https://cloud.compozy.com/billing/upgrade?interval=month`. Toggle back to **Annual**.
   - Expand FAQ **Why does CompozyOS ask me to sign in?** and follow its link to
     `/docs/account/sign-in`.
   - Expand FAQ **What is the refund policy?** and follow its link to `/legal/refunds`.

3. **Legal pages & mobile TOC:**
   - Open `/legal/terms`, `/legal/privacy`, and `/legal/refunds` via the footer links and the legal
     document switcher tabs.
   - On `/legal/privacy`, verify `#sub-processors` lists all 8 sub-processors (Cloudflare, Neon,
     Stripe, Resend, Google, GitHub, Sentry, Better Stack) and the hosted-route encryption note.
   - On a phone viewport on `/legal/terms`, open the **On this page** mobile table-of-contents
     disclosure and jump to a section.
   - Verify no draft review banner appears in the page body.

4. **Browser Pro purchase with promotion code:**
   - Click **Get Pro** on `/pricing` (annual selected) to open
     `https://cloud.compozy.com/billing/upgrade?interval=year`.
   - Sign in with Google or GitHub on the staging cloud page; confirm annual is preselected.
   - Click **Continue to checkout**, apply the test promotion code in Stripe Checkout, and complete
     test payment.
   - Confirm return to the cloud success page (`You're on Pro. Open Compozy to use your computers.`).

5. **App sign-in & U0 Why sign in? link:**
   - Open the fresh release-gated CompozyOS installation on U0.
   - Click **Why sign in?** and confirm `/docs/account/sign-in` opens with what is stored, what is
     never stored, offline behavior after first sign-in, CI account tokens, and account deletion.
   - Click the Terms and Privacy Policy links on U0 and confirm they resolve to `/legal/terms` and
     `/legal/privacy`.
   - Sign in with the same account that completed checkout in step 4; open **Settings → Account**
     and confirm the plan shows **Pro**.
