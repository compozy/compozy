/** S2: the lesson a skill proposal carries, with links to the sessions that show it. */
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Eyebrow, Pill } from "@compozy/ui";

import type { AgentSkillLessonPayload } from "../../types";

const NO_UNAVAILABLE_SESSIONS: readonly string[] = [];

const CONFIDENCE_LABEL: Readonly<Record<string, string>> = {
  low: "Low confidence",
  medium: "Medium confidence",
  high: "High confidence",
};

/** A session the daemon no longer knows reads as unavailable; any other session stays a link. */
function EvidenceSession({ id, unavailable }: { id: string; unavailable: boolean }) {
  if (unavailable) {
    return (
      <span className="font-mono text-small-body text-muted-foreground">
        {id} <span className="font-sans">· Unavailable</span>
      </span>
    );
  }
  return (
    <Link
      className="rounded-xs font-mono text-small-body text-fg underline outline-none focus-visible:shadow-focus-ring"
      params={{ id }}
      to="/session/$id"
    >
      {id}
    </Link>
  );
}

function LessonRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[6rem_minmax(0,1fr)] gap-3 max-[760px]:grid-cols-[minmax(0,1fr)] max-[760px]:gap-0.5">
      <dt className="text-small-body text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-body break-words text-fg">{children}</dd>
    </div>
  );
}

export function AgentSkillLesson({
  lesson,
  evidenceSessionIds,
  unavailableSessionIds = NO_UNAVAILABLE_SESSIONS,
}: {
  lesson: AgentSkillLessonPayload | null | undefined;
  evidenceSessionIds: readonly string[];
  /** Evidence sessions the daemon no longer knows; the container resolves them. */
  unavailableSessionIds?: readonly string[];
}) {
  if (!lesson) {
    return (
      <p className="text-small-body text-muted-foreground" data-testid="skill-review-no-lesson">
        No lesson recorded
      </p>
    );
  }
  const unavailable = new Set(unavailableSessionIds);
  return (
    <section aria-label="Lesson" className="flex flex-col gap-2" data-testid="skill-review-lesson">
      <div className="flex items-center gap-2">
        <Eyebrow className="text-subtle">Lesson</Eyebrow>
        <Pill size="sm">
          {CONFIDENCE_LABEL[lesson.confidence ?? "medium"] ?? lesson.confidence}
        </Pill>
      </div>
      <dl className="flex flex-col gap-1.5">
        <LessonRow label="When">{lesson.cue}</LessonRow>
        <LessonRow label="Lesson">{lesson.lesson}</LessonRow>
        <LessonRow label="Evidence">{lesson.evidence}</LessonRow>
        {evidenceSessionIds.length > 0 ? (
          <LessonRow label="Sessions">
            <ul className="flex flex-wrap gap-x-3 gap-y-1">
              {evidenceSessionIds.map(id => (
                <li key={id}>
                  <EvidenceSession id={id} unavailable={unavailable.has(id)} />
                </li>
              ))}
            </ul>
          </LessonRow>
        ) : null}
      </dl>
    </section>
  );
}
