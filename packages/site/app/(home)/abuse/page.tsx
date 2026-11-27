import type { Metadata } from "next";

import { ABUSE_REPORT } from "@/components/legal/abuse-content";
import { LegalPage } from "@/components/legal/legal-page";
import { createPageMetadata } from "@/lib/site-config";

export const metadata: Metadata = createPageMetadata({
  title: ABUSE_REPORT.title,
  description: ABUSE_REPORT.summary,
  path: ABUSE_REPORT.route,
});

export default function AbusePage() {
  return <LegalPage docId="abuse" />;
}
