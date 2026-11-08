// Event badge, A6 105×148 mm. One family, four roles: the surface and the art carry the role.
import type { CSSProperties } from "react";

import { LanesDiagram } from "../../art/diagrams/lanes-diagram";
import { Visual } from "../../art/visual";
import type { AtmosphereName } from "../../art/visuals";
import { cq, display, Mark, monoUp } from "../parts";

type Role = "team" | "speaker" | "partner" | "attendee";

interface Look {
  /** ground token, also the colour the art fades into */
  ground: string;
  art: AtmosphereName | "lanes";
  chip: CSSProperties;
}

const LOOK: Record<Role, Look> = {
  team: {
    ground: "--color-rail",
    art: "terrain",
    chip: { borderColor: "var(--color-brand-cyan)", color: "var(--color-brand-cyan)" },
  },
  speaker: {
    ground: "--color-rail",
    art: "swarm",
    chip: {
      background: "var(--color-brand-volt)",
      borderColor: "var(--color-brand-volt)",
      color: "var(--color-brand-ink)",
    },
  },
  partner: {
    ground: "--b-surface",
    art: "lanes",
    chip: { borderColor: "var(--b-accent)", color: "var(--b-accent)" },
  },
  attendee: {
    ground: "--color-black",
    art: "field",
    chip: { borderColor: "var(--color-faint)", color: "var(--color-fg)" },
  },
};

const isRole = (v: string): v is Role => v in LOOK;

export function EventBadge({
  variant = "attendee",
  role = "Attendee",
  name = "Name Surname",
  company = "Company",
  event = "Compozy meetup · São Paulo",
  date = "28 Nov 2026",
}: {
  variant?: string;
  role?: string;
  name?: string;
  company?: string;
  event?: string;
  date?: string;
}) {
  const kind: Role = isRole(variant) ? variant : "attendee";
  const look = LOOK[kind];
  const partner = kind === "partner";
  const muted = partner ? "text-(--b-muted)" : "text-muted-foreground";
  return (
    <div
      data-badge={kind}
      className={
        partner
          ? "band-paper absolute inset-0 overflow-hidden text-(--b-fg)"
          : "absolute inset-0 overflow-hidden text-fg"
      }
      style={{ background: `var(${look.ground})` }}
    >
      {look.art === "lanes" ? (
        <div className="absolute inset-x-0" style={{ bottom: "15%", height: "12%" }}>
          <LanesDiagram labels={false} lanes={["a", "b", "c", "d"]} />
        </div>
      ) : (
        <>
          <div className="absolute inset-x-0 bottom-0" style={{ height: "62%" }}>
            <Visual name={look.art} x={0.7} y={0.6} />
          </div>
          <div
            aria-hidden
            className="absolute inset-x-0 bottom-0"
            style={{
              height: "62%",
              backgroundImage: `linear-gradient(180deg, var(${look.ground}) 0%, transparent 40%, transparent 72%, var(${look.ground}) 96%)`,
            }}
          />
        </>
      )}
      {/* lanyard slot */}
      <div
        aria-hidden
        className="absolute left-1/2 -translate-x-1/2 rounded-full"
        style={{
          top: cq(4),
          width: cq(16),
          height: cq(3.2),
          background: partner
            ? "var(--b-line)"
            : "color-mix(in oklab, var(--color-fg) 14%, transparent)",
        }}
      />
      <div className="absolute flex flex-col" style={{ inset: `${cq(15)} ${cq(8)} ${cq(8)}` }}>
        <Mark size={5.2} variant={partner ? "light" : "dark"} className="self-start" />
        <p
          data-field="role"
          className={`${monoUp} self-start rounded-full border`}
          style={{
            marginTop: "9cqi",
            fontSize: "3cqi",
            letterSpacing: ".14em",
            padding: "1.2cqi 3.2cqi",
            ...look.chip,
          }}
        >
          {role}
        </p>
        <p
          data-field="name"
          className={display}
          style={{ marginTop: "5cqi", fontSize: "9.6cqi", lineHeight: 0.98, maxWidth: "8em" }}
        >
          {name}
        </p>
        <p
          data-field="company"
          className={`m-0 ${muted}`}
          style={{ marginTop: "3cqi", fontSize: "4.4cqi" }}
        >
          {company}
        </p>
        <div
          className={`mt-auto flex items-end justify-between font-mono ${muted}`}
          style={{ fontSize: "2.6cqi", letterSpacing: ".06em" }}
        >
          <p data-field="event" className="m-0" style={{ maxWidth: "62%" }}>
            {event}
          </p>
          <p data-field="date" className="m-0">
            {date}
          </p>
        </div>
      </div>
    </div>
  );
}
