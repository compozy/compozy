import { Avatar, AvatarFallback, AvatarGroup, AvatarImage, Eyebrow } from "@compozy/ui";
import { ArrowUpRight, Download } from "lucide-react";
import type { ChangelogPullRequestsResult, ChangelogRelease } from "@/lib/changelog/types";
import { formatBytes } from "./format";

function initial(login: string): string {
  return login.charAt(0).toUpperCase();
}

export function ReleaseEvidence({
  release,
  evidence,
}: {
  release: ChangelogRelease;
  evidence: ChangelogPullRequestsResult;
}) {
  const readyEvidence = evidence.status === "ready" ? evidence : null;
  return (
    <div className="mt-14 grid gap-8 border-t border-line pt-10 md:grid-cols-2">
      <section aria-labelledby="contributors-heading">
        <Eyebrow id="contributors-heading" className="text-muted">
          Contributors
        </Eyebrow>
        {readyEvidence && readyEvidence.contributors.length > 0 ? (
          <AvatarGroup className="mt-4">
            {readyEvidence.contributors.map(contributor => (
              <a
                key={contributor.login}
                href={contributor.url}
                target="_blank"
                rel="noreferrer noopener"
                aria-label={`${contributor.login} on GitHub`}
              >
                <Avatar>
                  <AvatarImage src={contributor.avatarUrl} alt="" />
                  <AvatarFallback>{initial(contributor.login)}</AvatarFallback>
                </Avatar>
              </a>
            ))}
          </AvatarGroup>
        ) : (
          <p className="mt-3 text-sm leading-6 text-muted">
            {evidence.status === "unavailable"
              ? "Contribution data is temporarily unavailable."
              : "No linked human contributors were found."}
          </p>
        )}
        {readyEvidence && readyEvidence.pullRequests.length > 0 && (
          <ul className="mt-6 flex flex-col gap-3">
            {readyEvidence.pullRequests.map(pullRequest => (
              <li key={pullRequest.number}>
                <a
                  href={pullRequest.url}
                  target="_blank"
                  rel="noreferrer noopener"
                  className="group flex gap-3 text-sm leading-6 text-muted hover:text-fg"
                >
                  <span className="font-mono text-subtle">#{pullRequest.number}</span>
                  <span>{pullRequest.title}</span>
                  <ArrowUpRight
                    className="mt-1 size-3 shrink-0 opacity-0 group-hover:opacity-100"
                    aria-hidden
                  />
                </a>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section aria-labelledby="assets-heading">
        <Eyebrow id="assets-heading" className="text-muted">
          Release assets
        </Eyebrow>
        <p className="mt-3 text-sm leading-6 text-muted">
          {release.assets.length} files published by the GitHub release workflow.
        </p>
        <details className="mt-4 rounded-xl border border-line bg-canvas-soft p-4">
          <summary className="cursor-pointer text-sm font-medium text-fg">Browse downloads</summary>
          <ul className="mt-4 flex max-h-80 flex-col gap-3 overflow-y-auto">
            {release.assets.map(asset => (
              <li key={asset.name} className="flex items-start gap-2">
                <Download className="mt-1 size-3.5 shrink-0 text-subtle" aria-hidden />
                <a href={asset.url} className="min-w-0 text-sm text-muted hover:text-fg">
                  <span className="block break-all font-mono text-xs text-fg">{asset.name}</span>
                  <span>
                    {formatBytes(asset.size)} · {asset.downloadCount} downloads
                  </span>
                </a>
              </li>
            ))}
            <li>
              <a href={release.sourceArchiveUrl} className="text-sm text-muted hover:text-fg">
                Source code (tar.gz)
              </a>
            </li>
            <li>
              <a href={release.sourceZipUrl} className="text-sm text-muted hover:text-fg">
                Source code (zip)
              </a>
            </li>
          </ul>
        </details>
      </section>
    </div>
  );
}
