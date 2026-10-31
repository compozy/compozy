import { Link } from "@tanstack/react-router";
import { AlertTriangle, ChevronRight } from "@compozy/ui/icons";

import {
  Button,
  Empty,
  OwnerAvatar,
  Panel,
  Pill,
  Section,
  SkeletonRegion,
  StateGlyph,
  Surface,
} from "@compozy/ui";

import { useElapsedNowSeconds } from "../hooks/use-elapsed-ticker";
import { formatHomeDurationSeconds } from "../lib/home-formatters";
import type { HomeRunCardModel, HomeWorkingNowStatus } from "../types";
import {
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
} from "./home-zone-shape";
import { HomeWorkingNowSkeleton } from "./home-zone-skeletons";
import { useSessionCreateActions } from "@/systems/session";
import { WorkspaceBadge } from "@/systems/workspace";

export interface HomeWorkingNowProps {
  cards: HomeRunCardModel[];
  total: number;
  status: HomeWorkingNowStatus;
  errorMessage?: string | null;
  liveEnabled?: boolean;
}

function HomeRunCard({ card, nowSeconds }: { card: HomeRunCardModel; nowSeconds: number }) {
  const ticked =
    card.elapsedBaseSeconds + Math.max(0, nowSeconds - Math.floor(card.baseAtMs / 1000));

  // One row of the shared live-work card: rows divide, the card carries the edge.
  const body = (
    <div
      className={`${HOME_RUN_ROW_CLASS} transition-colors duration-base hover:bg-surface-2`}
      data-slot="home-run-card"
    >
      <div className="min-w-0">
        <div className={HOME_RUN_TITLE_CLASS}>
          <StateGlyph state="running" />
          <OwnerAvatar name={card.agentName} ownerId={card.agentName} ownerKind="agent" size="sm" />
          <span className="truncate">{card.title}</span>
          {card.kind === "task_run" ? (
            <Pill size="xs" tone="neutral">
              task run
            </Pill>
          ) : null}
        </div>
        <p className={HOME_RUN_SUBTITLE_CLASS}>
          {card.project ? (
            <WorkspaceBadge
              className="max-w-40 shrink-[1000]"
              data-testid={`home-run-card-project-${card.key}`}
              identity={card.project.badge}
            />
          ) : null}
          <span className="min-w-0 truncate">{card.subtitle}</span>
        </p>
      </div>
      <span className="text-meta tabular-nums text-muted">
        {formatHomeDurationSeconds(ticked, "clock")}
      </span>
    </div>
  );

  if (card.sessionLink) {
    return (
      <Link
        className="block min-w-0 focus-visible:shadow-focus-inset focus-visible:outline-none"
        params={{ name: card.sessionLink.agentName, id: card.sessionLink.sessionId }}
        search={card.project ? { workspace: card.project.binding } : undefined}
        to="/agents/$name/sessions/$id"
      >
        {body}
      </Link>
    );
  }
  if (card.runLink) {
    return (
      <Link
        className="block min-w-0 focus-visible:shadow-focus-inset focus-visible:outline-none"
        params={{ id: card.runLink.taskId, runId: card.runLink.runId }}
        search={card.project ? { workspace: card.project.binding } : undefined}
        to="/tasks/$id/runs/$runId"
      >
        {body}
      </Link>
    );
  }
  return body;
}

function HomeWorkingNowEmpty({ onStartSession }: { onStartSession: () => void }) {
  return (
    <Surface className="flex items-center justify-between gap-3" data-slot="home-working-now-empty">
      <p className="text-small-body text-subtle">No agents working right now.</p>
      <Button onClick={onStartSession} size="sm" variant="neutral">
        Start a session
      </Button>
    </Surface>
  );
}

// A source failed while others returned cards: the list is shown but flagged
// incomplete so a dropped session/run is never passed off as "all of it".
function HomeWorkingNowPartialNote() {
  return (
    <div
      className="flex items-center gap-2 px-1 text-small-body text-warning"
      data-slot="home-working-now-partial"
    >
      <AlertTriangle aria-hidden="true" className="size-3.5 shrink-0" />
      <span>Some live work could not be loaded — this list may be incomplete.</span>
    </div>
  );
}

/**
 * Zone 3a — live run cards for active sessions and task runs; a single shared
 * ticker keeps every elapsed timer in lockstep. A failed read renders an error
 * (not a false "nothing here"), and a partial read shows what loaded but flags
 * the gap.
 */
export function HomeWorkingNow({
  cards,
  total,
  status,
  errorMessage,
  liveEnabled = true,
}: HomeWorkingNowProps) {
  const nowSeconds = useElapsedNowSeconds(liveEnabled);
  const sessionCreate = useSessionCreateActions();

  // Its own read is still out: the zone in its loading shape, not a count of zero.
  if (status === "loading" && cards.length === 0) {
    return (
      <SkeletonRegion label="Loading live work">
        <HomeWorkingNowSkeleton />
      </SkeletonRegion>
    );
  }

  return (
    <Section
      className="flex min-h-full flex-col"
      bodyClassName="flex flex-1 flex-col gap-2.5"
      count={total}
      label="Working now"
      right={
        <Button nativeButton={false} render={<Link to="/agents" />} size="sm" variant="link">
          View agents
          <ChevronRight aria-hidden="true" />
        </Button>
      }
    >
      {status === "error" ? (
        <Empty
          data-slot="home-working-now-error"
          description={errorMessage ?? "Try again in a moment."}
          icon={AlertTriangle}
          title="Couldn't load live work"
        />
      ) : cards.length === 0 ? (
        <HomeWorkingNowEmpty onStartSession={() => sessionCreate.openForAgent("")} />
      ) : (
        <>
          {status === "partial" ? <HomeWorkingNowPartialNote /> : null}
          <Panel bodyClassName="p-0" className="overflow-hidden">
            <div className="divide-y divide-line-soft">
              {cards.map(card => (
                <HomeRunCard card={card} key={card.key} nowSeconds={nowSeconds} />
              ))}
            </div>
          </Panel>
        </>
      )}
    </Section>
  );
}
