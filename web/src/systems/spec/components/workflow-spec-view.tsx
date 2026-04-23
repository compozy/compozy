import { useEffect, useState, type ReactElement } from "react";

import {
  EmptyState,
  Markdown,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
  cn,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { MarkdownDocument, WorkflowSpecDocument } from "../types";

export interface WorkflowSpecViewProps {
  spec: WorkflowSpecDocument;
  isRefreshing: boolean;
}

type SpecTabKey = "prd" | "techspec" | "adrs";

interface SpecTab {
  key: SpecTabKey;
  label: string;
  testId: string;
  present: boolean;
  badge?: string;
}

export function WorkflowSpecView(props: WorkflowSpecViewProps): ReactElement {
  const { spec, isRefreshing } = props;
  const { workflow, workspace, prd, techspec, adrs } = spec;
  const tabs = buildTabs(spec);
  const [active, setActive] = useState<SpecTabKey>(initialTab(tabs));

  useEffect(() => {
    setActive(initialTab(tabs));
  }, [workflow.slug]);

  useEffect(() => {
    if (!tabs.some(tab => tab.key === active && tab.present)) {
      setActive(initialTab(tabs));
    }
  }, [active, tabs]);

  return (
    <div className="space-y-6" data-testid="workflow-spec-view">
      <SectionHeading
        description={
          <span>
            <Link
              className="underline-offset-4 hover:underline"
              data-testid="workflow-spec-back"
              to="/workflows"
            >
              Back to workflows
            </Link>
            {" · "}
            {workspace.name} · updated {formatTimestamp(latestUpdate(spec))}
          </span>
        }
        eyebrow={`Workflow · ${workflow.slug}`}
        title={<span className="flex items-center gap-3">{workflow.slug}</span>}
      />

      <div
        className="flex flex-wrap items-center gap-1 border-b border-border"
        data-testid="workflow-spec-tabs"
        role="tablist"
      >
        {tabs.map(tab => (
          <button
            aria-selected={active === tab.key}
            aria-controls={`workflow-spec-panel-${tab.key}`}
            className={cn(
              "-mb-px flex items-center gap-2 border-b-2 px-3 py-2 text-sm transition-colors",
              tab.present
                ? "text-muted-foreground hover:text-foreground"
                : "cursor-not-allowed text-muted-foreground/60",
              active === tab.key && tab.present
                ? "border-[color:var(--color-primary)] text-foreground"
                : "border-transparent"
            )}
            data-testid={tab.testId}
            disabled={!tab.present}
            id={`workflow-spec-tab-${tab.key}`}
            key={tab.key}
            onClick={() => {
              if (tab.present) {
                setActive(tab.key);
              }
            }}
            role="tab"
            type="button"
          >
            <span>{tab.label}</span>
            {tab.badge ? <span className="eyebrow text-muted-foreground">{tab.badge}</span> : null}
          </button>
        ))}
      </div>

      {active === "prd" ? (
        <div aria-labelledby="workflow-spec-tab-prd" id="workflow-spec-panel-prd" role="tabpanel">
          <DocumentCard document={prd} kind="PRD" testId="workflow-spec-prd" />
        </div>
      ) : null}
      {active === "techspec" ? (
        <div
          aria-labelledby="workflow-spec-tab-techspec"
          id="workflow-spec-panel-techspec"
          role="tabpanel"
        >
          <DocumentCard document={techspec} kind="TechSpec" testId="workflow-spec-techspec" />
        </div>
      ) : null}
      {active === "adrs" ? (
        <div aria-labelledby="workflow-spec-tab-adrs" id="workflow-spec-panel-adrs" role="tabpanel">
          <AdrList adrs={adrs ?? []} />
        </div>
      ) : null}

      {isRefreshing ? (
        <p className="text-xs text-muted-foreground" data-testid="workflow-spec-refreshing">
          refreshing…
        </p>
      ) : null}
    </div>
  );
}

function DocumentCard({
  document,
  kind,
  testId,
}: {
  document: MarkdownDocument | undefined;
  kind: string;
  testId: string;
}): ReactElement {
  if (!document) {
    return (
      <SurfaceCard data-testid={`${testId}-missing`}>
        <SurfaceCardHeader>
          <div>
            <SurfaceCardEyebrow>{kind}</SurfaceCardEyebrow>
            <SurfaceCardTitle>Document not found</SurfaceCardTitle>
            <SurfaceCardDescription>
              The daemon reports no {kind} document on disk for this workflow yet.
            </SurfaceCardDescription>
          </div>
        </SurfaceCardHeader>
      </SurfaceCard>
    );
  }
  const markdown = document.markdown?.trim() ?? "";
  return (
    <SurfaceCard data-testid={testId}>
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>{kind}</SurfaceCardEyebrow>
          <SurfaceCardTitle>{document.title}</SurfaceCardTitle>
          <SurfaceCardDescription>
            Updated {formatTimestamp(document.updated_at)}
          </SurfaceCardDescription>
        </div>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {markdown.length === 0 ? (
          <EmptyState data-testid={`${testId}-empty`} title="Document body is empty" />
        ) : (
          <div
            className="max-h-[min(72dvh,820px)] overflow-auto rounded-[var(--radius-lg)] border border-border-subtle bg-[color:var(--surface-inset)] px-5 py-4 shadow-[var(--shadow-xs)]"
            data-testid={`${testId}-body`}
          >
            <Markdown>{markdown}</Markdown>
          </div>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function AdrList({ adrs }: { adrs: MarkdownDocument[] }): ReactElement {
  if (adrs.length === 0) {
    return (
      <EmptyState
        data-testid="workflow-spec-adrs-empty"
        description="This workflow does not have any ADRs on disk yet."
        title="No ADRs yet"
      />
    );
  }
  return (
    <div className="space-y-3" data-testid="workflow-spec-adrs-list">
      {adrs.map(adr => (
        <SurfaceCard data-testid={`workflow-spec-adr-${adr.id}`} key={adr.id}>
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>{adr.kind}</SurfaceCardEyebrow>
              <SurfaceCardTitle>{adr.title}</SurfaceCardTitle>
              <SurfaceCardDescription>
                Updated {formatTimestamp(adr.updated_at)}
              </SurfaceCardDescription>
            </div>
            <StatusBadge tone="info">{adr.id}</StatusBadge>
          </SurfaceCardHeader>
          <SurfaceCardBody>
            <div
              className="max-h-[min(62dvh,620px)] overflow-auto rounded-[var(--radius-lg)] border border-border-subtle bg-[color:var(--surface-inset)] px-5 py-4 shadow-[var(--shadow-xs)]"
              data-testid={`workflow-spec-adr-body-${adr.id}`}
            >
              {adr.markdown?.trim().length ? (
                <Markdown>{adr.markdown.trim()}</Markdown>
              ) : (
                <EmptyState className="py-5" title="Document body is empty" />
              )}
            </div>
          </SurfaceCardBody>
        </SurfaceCard>
      ))}
    </div>
  );
}

function buildTabs(spec: WorkflowSpecDocument): SpecTab[] {
  const adrs = spec.adrs ?? [];
  return [
    {
      key: "prd",
      label: "PRD",
      testId: "workflow-spec-tab-prd",
      present: Boolean(spec.prd),
    },
    {
      key: "techspec",
      label: "TechSpec",
      testId: "workflow-spec-tab-techspec",
      present: Boolean(spec.techspec),
    },
    {
      key: "adrs",
      label: "ADRs",
      testId: "workflow-spec-tab-adrs",
      present: true,
      badge: adrs.length > 0 ? String(adrs.length) : undefined,
    },
  ];
}

function initialTab(tabs: SpecTab[]): SpecTabKey {
  const first = tabs.find(tab => tab.present);
  return (first?.key ?? "prd") as SpecTabKey;
}

function latestUpdate(spec: WorkflowSpecDocument): string | undefined {
  const candidates = [spec.prd?.updated_at, spec.techspec?.updated_at];
  for (const adr of spec.adrs ?? []) {
    candidates.push(adr.updated_at);
  }
  const populated = candidates.filter((x): x is string => Boolean(x));
  if (populated.length === 0) {
    return undefined;
  }
  populated.sort();
  return populated[populated.length - 1];
}

function formatTimestamp(raw: string | undefined): string {
  if (!raw) {
    return "unknown";
  }
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
