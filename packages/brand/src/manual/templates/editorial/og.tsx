// Link images, 1200×630: default, blog, docs (paper, like the docs site) and changelog.
import { Visual } from "../../art/visual";
import { cq, display, ink, Mark, monoUp, Scrim, scrim, Shot } from "../parts";

const fade = (solid: number, end: number, surface?: string) =>
  scrim(
    "90deg",
    [
      [0, 1],
      [solid, 0.9],
      [end, 0],
    ],
    surface
  );

function Eyebrow({ label }: { label: string }) {
  return (
    <div className="flex items-center" style={{ gap: "2cqi" }}>
      <Mark size={2.6} />
      <span
        className={`${monoUp} text-brand-cyan`}
        style={{ fontSize: "1.3cqi", letterSpacing: ".12em" }}
      >
        {label}
      </span>
    </div>
  );
}

const column = "absolute flex flex-col";
const columnBox = (w: number) => ({ top: cq(6), bottom: cq(6), left: cq(6), width: cq(w) });

/** Default link image: positioning line, Atmosphere converge right, domain. */
export function OgDefault({
  title = "The autonomous control plane for your agents.",
  meta = "compozy.com",
}: {
  title?: string;
  meta?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="converge" x={0.85} y={0.5} />
      <Scrim style={fade(42, 70)} />
      <div className={column} style={columnBox(56)}>
        <Mark size={3} className="self-start" />
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "5.8cqi", lineHeight: 1, marginTop: "auto" }}
        >
          {title}
        </p>
        <p
          data-field="meta"
          className="m-0 font-mono text-muted-foreground"
          style={{ fontSize: "1.5cqi", letterSpacing: ".06em", marginTop: "3cqi" }}
        >
          {meta}
        </p>
      </div>
    </div>
  );
}

/** Blog post link image: eyebrow, title (≤ 70 characters), author and date, swarm right. */
export function OgBlog({
  title = "Why agent work needs a definition of done",
  author = "Pedro Nauck",
  date = "Nov 12, 2026",
}: {
  title?: string;
  author?: string;
  date?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="swarm" x={0.85} y={0.5} />
      <Scrim style={fade(50, 78)} />
      <div className={column} style={columnBox(62)}>
        <Eyebrow label="Blog" />
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "5.4cqi", lineHeight: 1.02, marginTop: "auto" }}
        >
          {title}
        </p>
        <p
          className={`${monoUp} text-muted-foreground`}
          style={{ fontSize: "1.35cqi", letterSpacing: ".1em", marginTop: "3cqi" }}
        >
          <span data-field="author">{author}</span> · <span data-field="date">{date}</span>
        </p>
      </div>
    </div>
  );
}

/** Docs link image: paper like the docs site; section, page title, path, the desktop in light. */
export function OgDocs({
  section = "Always-on agents",
  title = "Set when an agent checks in",
  path = "docs.compozy.com/agents/check-ins",
}: {
  section?: string;
  title?: string;
  path?: string;
}) {
  return (
    <div className="band-paper absolute inset-0 overflow-hidden text-(--b-fg)">
      <Shot
        name="desktop"
        theme="light"
        w={92}
        className="border-(--b-line)"
        style={{ left: cq(42), top: cq(14) }}
      />
      <Scrim
        style={scrim(
          "90deg",
          [
            [0, 1],
            [46, 0.94],
            [62, 0],
          ],
          "--b-surface"
        )}
      />
      <div className={column} style={columnBox(46)}>
        <div className="flex items-center" style={{ gap: "2cqi" }}>
          <Mark size={2.6} variant="light" />
          <span
            className={`${monoUp} border border-(--b-line) text-(--b-accent)`}
            style={{
              fontSize: "1.2cqi",
              letterSpacing: ".12em",
              padding: ".35cqi .8cqi",
              borderRadius: ".5cqi",
            }}
          >
            Docs
          </span>
        </div>
        <p
          data-field="section"
          className={`${monoUp} text-(--b-muted)`}
          style={{ fontSize: "1.4cqi", letterSpacing: ".12em", marginTop: "auto" }}
        >
          {section}
        </p>
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "4.6cqi", lineHeight: 1.02, marginTop: "1.4cqi" }}
        >
          {title}
        </p>
        <p
          data-field="path"
          className="m-0 font-mono text-(--b-muted)"
          style={{ fontSize: "1.3cqi", marginTop: "3cqi" }}
        >
          {path}
        </p>
      </div>
    </div>
  );
}

/** Changelog link image: the version is the headline, three highlights, the sessions board. */
export function OgChangelog({
  version = "v0.6.0",
  date = "Nov 18, 2026",
  h1 = "Orchestrator hands work to other harnesses",
  h2 = "Board by who has the next step",
  h3 = "Always-on agents, with faces",
}: {
  version?: string;
  date?: string;
  h1?: string;
  h2?: string;
  h3?: string;
}) {
  const highlights: [field: string, text: string][] = [
    ["h1", h1],
    ["h2", h2],
    ["h3", h3],
  ];
  return (
    <div className={ink}>
      <Shot name="board" w={110} style={{ left: cq(44), top: cq(12) }} />
      <Scrim style={fade(46, 62)} />
      <div className={column} style={columnBox(46)}>
        <Eyebrow label="Changelog" />
        <p
          data-field="date"
          className={`${monoUp} text-muted-foreground`}
          style={{ fontSize: "1.35cqi", letterSpacing: ".1em", marginTop: "auto" }}
        >
          {date}
        </p>
        <p
          data-field="version"
          className={display}
          style={{ fontSize: "10cqi", lineHeight: 0.95, marginTop: ".8cqi" }}
        >
          {version}
        </p>
        <ul
          className="m-0 list-none p-0 font-mono"
          style={{ fontSize: "1.5cqi", lineHeight: 1.7, marginTop: "2.6cqi" }}
        >
          {highlights.map(([field, text]) => (
            <li key={field}>
              <span className="text-brand-cyan">+ </span>
              <span data-field={field}>{text}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
