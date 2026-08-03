import { Eyebrow } from "@compozy/ui";
import Link from "next/link";

export default function ChangelogReleaseNotFound() {
  return (
    <main className="px-4 py-24">
      <div className="mx-auto max-w-2xl rounded-xl border border-line bg-canvas-soft p-8">
        <Eyebrow className="text-warning">Release not found</Eyebrow>
        <h1 className="mt-4 font-sans text-3xl font-semibold tracking-tight text-fg">
          This version is not in the public changelog.
        </h1>
        <p className="mt-4 text-muted">
          The site lists published GitHub releases from v0.3.0-beta.1 onward.
        </p>
        <Link href="/changelog" className="mt-7 inline-flex text-sm font-medium text-accent">
          Browse all releases →
        </Link>
      </div>
    </main>
  );
}
