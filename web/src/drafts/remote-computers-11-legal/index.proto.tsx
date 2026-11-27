import "../landing/landing.css";
import "../docs/docs.css";
import "../remote-computers-08-pricing/site-dark-pin.css";

import { definePrototype, type PrototypeProps } from "@protohq/cli";
import { LazyMotion, MotionConfig, domMax } from "motion/react";

import { LegalPage } from "./legal-page";
import { WhySignInDocsPage } from "./why-sign-in-page";

export const meta = definePrototype({
  title: "Legal and sign-in pages",
  spec: "remote-computers",
  surface: "U11",
  route: "/legal/terms",
  states: [
    { id: "terms", label: "Terms of Service (/legal/terms)" },
    { id: "privacy", label: "Privacy Policy (/legal/privacy)" },
    { id: "refunds", label: "Refund Policy (/legal/refunds)" },
    { id: "why-sign-in", label: "Why sign in (/docs/account/sign-in)" },
    { id: "terms-mobile-toc", label: "Terms · Mobile table of contents open" },
    { id: "privacy-subprocessors", label: "Privacy · Sub-processors table" },
  ],
});

function renderState(state: string) {
  switch (state) {
    case "privacy":
      return <LegalPage initialDocId="privacy" key={state} />;
    case "privacy-subprocessors":
      return <LegalPage initialDocId="privacy" key={state} scrollToSectionId="sub-processors" />;
    case "refunds":
      return <LegalPage initialDocId="refunds" key={state} />;
    case "terms-mobile-toc":
      return <LegalPage initialDocId="terms" key={state} mobileTocOpen />;
    case "why-sign-in":
      return <WhySignInDocsPage key={state} />;
    default:
      return <LegalPage initialDocId="terms" key={state} />;
  }
}

export default function RemoteComputersLegalDraft({ state }: PrototypeProps) {
  return (
    <LazyMotion features={domMax}>
      <MotionConfig reducedMotion="always">{renderState(state)}</MotionConfig>
    </LazyMotion>
  );
}
