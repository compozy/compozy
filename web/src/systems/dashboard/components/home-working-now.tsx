import { Link } from "@tanstack/react-router";
import { AlertTriangle, ChevronRight } from "lucide-react";

import { Button, Empty, OwnerAvatar, Pill, Section, SkeletonRows, Surface } from "@agh/ui";

import { useSessionCreate } from "@/systems/session";

import { useElapsedNowSeconds } from "../hooks/use-elapsed-ticker";
import { formatHomeDurationSeconds } from "../lib/home-formatters";
import type { HomeRunCardModel, HomeWorkingNowStatus } from "../types";

export interface HomeWorkingNowProps {
  cards: HomeRunCardModel[];
  total: number;
  status: HomeWorkingNowStatus;
  errorMessage?: string | null;
}

function HomeRunCard({ card, nowSeconds }: { card: HomeRunCardModel; nowSeconds: number }) {
  const ticked =
    card.elapsedBaseSeconds + Math.max(0, nowSeconds - Math.floor(card.baseAtMs / 1000));

  const body = (
    <Surface
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 transition-colors duration-base hover:bg-canvas-tint"
      data-slot="home-run-card"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2 text-small-body font-medium text-fg-strong">
          <Pill.Dot pulse tone="accent" />
          <OwnerAvatar name={card.agentName} ownerId={card.agentName} ownerKind="agent" size="sm" />
          <span className="truncate">{card.title}</span>
          {card.kind === "task_run" ? (
            <Pill size="xs" tone="neutral">
              task run
            </Pill>
          ) : null}
        </div>
        <p className="mt-1 truncate text-small-body text-muted">{card.subtitle}</p>
      </div>
      <span className="font-mono text-mono-id tabular-nums text-muted">
        {formatHomeDurationSeconds(ticked, "clock")}
      </span>
    </Surface>
  );

  if (card.sessionLink) {
    return (
      <Link
        className="block min-w-0 rounded-lg focus-visible:shadow-focus-ring focus-visible:outline-none"
        params={{ name: card.sessionLink.agentName, id: card.sessionLink.sessionId }}
        to="/agents/$name/sessions/$id"
      >
        {body}
      </Link>
    );
  }
  if (card.runLink) {
    return (
      <Link
        className="block min-w-0 rounded-lg focus-visible:shadow-focus-ring focus-visible:outline-none"
        params={{ id: card.runLink.taskId, runId: card.runLink.runId }}
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
export function HomeWorkingNow({ cards, total, status, errorMessage }: HomeWorkingNowProps) {
  const nowSeconds = useElapsedNowSeconds();
  const sessionCreate = useSessionCreate();

  return (
    <Section
      className="flex min-h-full flex-col"
      bodyClassName="flex flex-1 flex-col gap-2.5"
      count={total}
      label="Working now"
      right={
        <Button render={<Link to="/agents" />} size="sm" variant="ghost">
          View agents
          <ChevronRight aria-hidden="true" />
        </Button>
      }
    >
      {status === "error" ? (
        <Empty
          data-slot="home-working-now-error"
          description={errorMessage ?? "The daemon did not return active work."}
          icon={AlertTriangle}
          title="Unable to load live work"
        />
      ) : status === "loading" && cards.length === 0 ? (
        <Surface data-slot="home-working-now-loading">
          <SkeletonRows count={2} rowClassName="gap-1.5 py-1" />
        </Surface>
      ) : cards.length === 0 ? (
        <HomeWorkingNowEmpty onStartSession={() => sessionCreate.openForAgent("")} />
      ) : (
        <>
          {status === "partial" ? <HomeWorkingNowPartialNote /> : null}
          {cards.map(card => (
            <HomeRunCard card={card} key={card.key} nowSeconds={nowSeconds} />
          ))}
        </>
      )}
    </Section>
  );
}
