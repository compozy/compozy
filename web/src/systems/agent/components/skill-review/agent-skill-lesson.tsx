/** S2: the lesson a skill proposal carries, with links to the sessions that show it. */
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Eyebrow, Pill } from "@compozy/ui";

import { SessionNotFoundError, sessionOwnerOptions } from "@/systems/session";

import type { AgentSkillLessonPayload } from "../../types";

const CONFIDENCE_LABEL: Readonly<Record<string, string>> = {
  low: "Low confidence",
  medium: "Medium confidence",
  high: "High confidence",
};

/** A session the daemon no longer knows reads as unavailable; any other failure keeps the link. */
function EvidenceSession({ id }: { id: string }) {
  const navigate = useNavigate();
  const owner = useQuery({ ...sessionOwnerOptions(id), refetchOnWindowFocus: false });
  if (owner.error instanceof SessionNotFoundError) {
    return (
      <span className="font-mono text-small-body text-muted-foreground">
        {id} <span className="font-sans">· Unavailable</span>
      </span>
    );
  }
  return (
    <a
      className="rounded-xs font-mono text-small-body text-fg underline outline-none focus-visible:shadow-focus-ring"
      href={`/session/${encodeURIComponent(id)}`}
      onClick={event => {
        event.preventDefault();
        void navigate({ to: "/session/$id", params: { id } });
      }}
    >
      {id}
    </a>
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
}: {
  lesson: AgentSkillLessonPayload | null | undefined;
  evidenceSessionIds: readonly string[];
}) {
  if (!lesson) {
    return (
      <p className="text-small-body text-muted-foreground" data-testid="skill-review-no-lesson">
        No lesson recorded
      </p>
    );
  }
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
                  <EvidenceSession id={id} />
                </li>
              ))}
            </ul>
          </LessonRow>
        ) : null}
      </dl>
    </section>
  );
}
