import { appendFileSync } from "node:fs";

import { CAPABILITIES, Extension } from "@compozy/extension-sdk";
import type { FetchRequest, ResolveIssuesRequest, ReviewItem } from "@compozy/extension-sdk";

type RecordEntry = {
  kind: string;
  payload?: Record<string, unknown>;
};

export function createReviewProviderExtension(
  name = "__EXTENSION_NAME__",
  version = "__EXTENSION_VERSION__"
): Extension {
  const extension = new Extension(name, version);
  const providerName = `${name}-review`;
  const mode = process.env.COMPOZY_TS_REVIEW_MODE?.trim() ?? "";

  if (mode !== "missing_capability") {
    extension.withCapabilities(CAPABILITIES.providersRegister);
  }

  if (mode !== "missing_registration") {
    extension.registerReviewProvider(providerName, {
      async fetchReviews(context, request: FetchRequest): Promise<ReviewItem[]> {
        record("fetch_reviews", {
          provider: context.provider,
          pr: request.pr,
          include_nitpicks: request.include_nitpicks ?? false,
        });
        return [
          {
            title: `Fetched review for ${request.pr}`,
            file: "README.md",
            body: `Handled by ${context.provider}`,
            provider_ref: "thread-ts-1",
          },
        ];
      },

      async resolveIssues(context, request: ResolveIssuesRequest): Promise<void> {
        record("resolve_issues", {
          provider: context.provider,
          pr: request.pr,
          issues: request.issues ?? [],
        });
      },
    });
  }

  return extension;
}

function record(kind: string, payload: Record<string, unknown>): void {
  const path = process.env.COMPOZY_TS_RECORD_PATH?.trim();
  if (path === undefined || path === "") {
    return;
  }

  const entry: RecordEntry = { kind, payload };
  appendFileSync(path, `${JSON.stringify(entry)}\n`, "utf8");
}
