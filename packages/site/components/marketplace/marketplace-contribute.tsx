import { GitPullRequest } from "lucide-react";
import { siteConfig } from "@/lib/site-config";

const CONTRIBUTION_GUIDE_URL = `${siteConfig.repoUrl}/tree/main/catalog`;

/** Closes every browse surface, as in the reference marketplace. */
export function MarketplaceContribute() {
  return (
    <section
      aria-labelledby="contribute"
      className="flex flex-col gap-5 rounded-lg border border-line bg-canvas-soft p-6 sm:flex-row sm:items-center sm:justify-between sm:gap-8"
    >
      <div className="max-w-[60ch]">
        <h2 id="contribute" className="text-lg font-semibold tracking-[-0.01em] text-fg">
          Add an entry to the catalog
        </h2>
        <p className="mt-2 text-small-body leading-relaxed text-muted">
          The catalog is a set of JSON feeds in the Compozy repository. Package your artifact with{" "}
          <code>compozy-catalog package</code>, validate it with{" "}
          <code>compozy-catalog validate</code>, and open a pull request against{" "}
          <code>catalog/</code> — a future site build renders that snapshot, while a daemon picks it
          up after its configured catalog source refreshes.
        </p>
      </div>
      <a
        href={CONTRIBUTION_GUIDE_URL}
        target="_blank"
        rel="noreferrer noopener"
        className="inline-flex shrink-0 items-center gap-2 rounded-lg border border-line px-4 py-2 text-small-body font-medium text-fg transition-colors hover:border-accent/35 hover:text-accent"
      >
        <GitPullRequest aria-hidden className="size-4" />
        Read the contribution guide
      </a>
    </section>
  );
}
