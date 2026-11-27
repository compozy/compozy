import type { LegalDocument } from "./legal-content";

export const ABUSE_REPORT: Omit<LegalDocument, "id"> = {
  navLabel: "Report abuse",
  title: "Report hosted-route abuse",
  route: "/abuse",
  summary: "Report phishing, unlawful content, or other misuse of a Compozy Cloud hosted route.",
  lastUpdated: "November 27, 2026",
  sections: [
    {
      id: "what-to-include",
      title: "What to include",
      body: `Email [abuse@compozy.com](mailto:abuse@compozy.com) with the details of the incident.

- The full hosted-route URL on a \`*.compozy.run\` address.
- The date, time, and time zone when you observed the abuse.
- A description of what happened and any relevant screenshots or log excerpts. Remove passwords, tokens, and other private data before sharing.`,
    },
    {
      id: "what-happens-next",
      title: "What happens next",
      body: `The Compozy Cloud team reviews reports and may reply by email to ask for more details. Compozy, LLC may take down routes that violate the [Terms of Service](/legal/terms#acceptable-use-and-suspension).`,
    },
  ],
};
