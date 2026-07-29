import { BookOpen, User } from "lucide-react";
import Link from "next/link";

import { LLMCopyButton, OpenWithAI, ViewOptions } from "@/components/docs/page-actions";
import type { DocMastheadCrumb } from "@/lib/doc-masthead-meta";

interface DocPageMastheadProps {
  product: string;
  audience: string;
  crumbs: DocMastheadCrumb[];
  title: string;
  description?: string;
  sectionPageCount?: number | null;
  markdownUrl?: string;
  pageUrl?: string;
  githubUrl?: string;
}

function CrumbSep() {
  return <span aria-hidden className="site-doc-masthead__crumb-sep" />;
}

export function DocPageMasthead({
  product,
  audience,
  crumbs,
  title,
  description,
  sectionPageCount,
  markdownUrl,
  pageUrl,
  githubUrl,
}: DocPageMastheadProps) {
  const showActions = Boolean(markdownUrl && pageUrl && githubUrl);
  const showMeta = Boolean(audience) || (sectionPageCount != null && sectionPageCount > 0);

  return (
    <header className="site-doc-masthead not-prose">
      <div className="site-doc-masthead__context">
        <nav aria-label="Breadcrumb" className="site-doc-masthead__crumbs">
          <span className="site-doc-masthead__product">{product}</span>
          {crumbs.map(crumb => (
            <span key={`${crumb.name}-${crumb.href ?? "leaf"}`} className="contents">
              <CrumbSep />
              {crumb.href ? (
                <Link href={crumb.href}>{crumb.name}</Link>
              ) : (
                <span aria-current="page" className="site-doc-masthead__leaf">
                  {crumb.name}
                </span>
              )}
            </span>
          ))}
        </nav>

        {showActions && markdownUrl && pageUrl && githubUrl ? (
          <div className="site-doc-masthead__actions">
            <LLMCopyButton markdownUrl={markdownUrl} />
            <OpenWithAI pageUrl={pageUrl} />
            <ViewOptions githubUrl={githubUrl} markdownUrl={markdownUrl} />
          </div>
        ) : null}
      </div>

      <h1 className="site-doc-masthead__title">{title}</h1>

      {description ? <p className="site-doc-masthead__lead">{description}</p> : null}

      {showMeta ? (
        <div className="site-doc-masthead__meta">
          {audience ? (
            <span className="site-doc-masthead__meta-item">
              <User aria-hidden className="site-doc-masthead__meta-icon" />
              For <strong>{audience}</strong>
            </span>
          ) : null}
          {sectionPageCount != null && sectionPageCount > 0 ? (
            <span className="site-doc-masthead__meta-item">
              <BookOpen aria-hidden className="site-doc-masthead__meta-icon" />
              <strong>
                {sectionPageCount} {sectionPageCount === 1 ? "page" : "pages"}
              </strong>{" "}
              in this section
            </span>
          ) : null}
        </div>
      ) : null}
    </header>
  );
}
