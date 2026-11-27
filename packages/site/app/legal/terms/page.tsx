import type { Metadata } from "next";

import { LEGAL_DOCUMENTS } from "@/components/legal/legal-content";
import { LegalPage } from "@/components/legal/legal-page";
import { createPageMetadata } from "@/lib/site-config";

const doc = LEGAL_DOCUMENTS.terms;

export const metadata: Metadata = createPageMetadata({
  title: doc.title,
  description: doc.summary,
  path: doc.route,
});

export default function TermsPage() {
  return <LegalPage docId="terms" />;
}
