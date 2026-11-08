// Stationery: business card (front and back), A4 letterhead, DL envelope and the email signature.
// Paper pieces sit on the paper band (--b-*), so they read the same tokens as the manual's paper.
import { BarsDiagram } from "../../art/diagrams/bars-diagram";
import { LanesDiagram } from "../../art/diagrams/lanes-diagram";
import { Visual } from "../../art/visual";
import { cq, display, gradientFill, ink, Mark, monoUp, Scrim, scrim } from "../parts";

const paper = "band-paper absolute inset-0 overflow-hidden text-(--b-fg)";

/** Business card front, 85×55 mm: lockup, name block, quiet Cone atmosphere. */
export function BusinessCardFront({
  name = "Name Surname",
  role = "Founding engineer",
  email = "name@compozy.com",
  phone = "+1 000 000 0000",
  url = "compozy.com",
}: {
  name?: string;
  role?: string;
  email?: string;
  phone?: string;
  url?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="cone" x={0.9} y={0.3} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [48, 0.9],
          [78, 0],
        ])}
      />
      <div className="absolute flex flex-col justify-between" style={{ inset: cq(8) }}>
        <Mark size={5.4} className="self-start" />
        <div>
          <p
            data-field="name"
            className={`${display} tracking-[-0.025em]`}
            style={{ fontSize: "5.6cqi", lineHeight: 1.05 }}
          >
            {name}
          </p>
          <p
            data-field="role"
            className="m-0 text-muted-foreground"
            style={{ fontSize: "3.1cqi", marginTop: "1cqi" }}
          >
            {role}
          </p>
          <div
            className="font-mono"
            style={{
              fontSize: "2.6cqi",
              letterSpacing: ".02em",
              marginTop: "5cqi",
              lineHeight: 1.6,
            }}
          >
            <p data-field="email" className="m-0">
              {email}
            </p>
            <p data-field="phone" className="m-0 text-muted-foreground">
              {phone}
            </p>
            <p data-field="url" className="m-0 text-brand-cyan">
              {url}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Business card back: ink with a Swarm crop and one mono line. */
export function BusinessCardBack({
  line = "Keep every agent running",
  url = "compozy.com",
}: {
  line?: string;
  url?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="swarm" x={0.95} y={0.5} />
      <Scrim
        style={scrim("90deg", [
          [0, 1],
          [38, 0.85],
          [70, 0],
        ])}
      />
      <div className="absolute flex flex-col justify-between" style={{ inset: cq(8) }}>
        <Mark size={5.4} className="self-start" />
        <div className="flex flex-col items-start" style={{ gap: "1.2cqi" }}>
          <p
            data-field="line"
            className={monoUp}
            style={{ fontSize: "2.6cqi", letterSpacing: ".12em" }}
          >
            {line}
          </p>
          <p data-field="url" className="m-0 font-mono" style={{ fontSize: "2.6cqi" }}>
            {url}
          </p>
        </div>
      </div>
    </div>
  );
}

/** Letterhead, A4: paper, lockup, address column, one thin run-lanes strip at the foot. */
export function Letterhead({
  date = "7 November 2026",
  to = "Recipient name · Company · City",
  subject = "Design partner programme",
  body = "Letter text sits in Figtree 10.5 pt on a 15 pt line, left aligned, ragged right. Paragraphs are separated by one empty line, never indented. The text column is 120 mm wide so a line stays under 75 characters. Keep letters to one page; a second page repeats only the footer.",
  sender = "Name Surname",
  role = "Founding engineer",
  address = "Compozy · compozy.com · hello@compozy.com",
}: {
  date?: string;
  to?: string;
  subject?: string;
  body?: string;
  sender?: string;
  role?: string;
  address?: string;
}) {
  return (
    <div className={paper}>
      <div className="absolute" style={{ right: 0, bottom: 0, width: cq(46), height: cq(9) }}>
        <LanesDiagram labels={false} lanes={["a", "b", "c", "d"]} />
      </div>
      <div className="absolute" style={{ left: cq(9.5), top: cq(8) }}>
        <Mark size={3.2} variant="light" />
      </div>
      <p
        className="absolute m-0 text-right font-mono text-(--b-muted)"
        style={{
          right: cq(9.5),
          top: cq(8.2),
          fontSize: "1.05cqi",
          lineHeight: 1.7,
          letterSpacing: ".04em",
        }}
      >
        compozy.com
        <br />
        hello@compozy.com
        <br />
        github.com/compozy
      </p>
      <div
        className="absolute bg-(--b-accent)"
        style={{ left: cq(9.5), top: cq(14), width: cq(6), height: ".18cqi" }}
      />
      <div
        className="absolute"
        style={{ left: cq(9.5), top: cq(30), width: cq(57), fontSize: "1.76cqi", lineHeight: 1.45 }}
      >
        <p
          data-field="date"
          className="m-0 font-mono text-(--b-muted)"
          style={{ fontSize: "1.1cqi", letterSpacing: ".06em" }}
        >
          {date}
        </p>
        <p data-field="to" className="m-0" style={{ marginTop: "4cqi" }}>
          {to}
        </p>
        <p
          data-field="subject"
          className="m-0 font-display tracking-[-0.02em]"
          style={{ marginTop: "5cqi", fontSize: "2.2cqi" }}
        >
          {subject}
        </p>
        <p data-field="body" className="m-0" style={{ marginTop: "2.4cqi" }}>
          {body}
        </p>
        <p className="m-0" style={{ marginTop: "2cqi" }}>
          The accent rule under the lockup is the only colour on the page besides the run lanes at
          the foot. Signatures go in ink, never blue.
        </p>
        <p data-field="sender" className="m-0 font-medium" style={{ marginTop: "6cqi" }}>
          {sender}
        </p>
        <p data-field="role" className="m-0 text-(--b-muted)">
          {role}
        </p>
      </div>
      <p
        data-field="address"
        className="absolute m-0 font-mono text-(--b-muted)"
        style={{ left: cq(9.5), bottom: cq(6), fontSize: "1.05cqi", letterSpacing: ".06em" }}
      >
        {address}
      </p>
    </div>
  );
}

/** Envelope DL, 220×110 mm: return address, the address window, an activity strip on the flap. */
export function Envelope({ returnAddress = "Compozy · compozy.com" }: { returnAddress?: string }) {
  return (
    <div className={paper}>
      <div className="absolute" style={{ right: 0, top: 0, width: cq(24), height: cq(12) }}>
        <BarsDiagram axis={false} n={70} />
      </div>
      <div className="absolute" style={{ left: cq(6.8), top: cq(6.8) }}>
        <Mark size={2.4} variant="light" />
      </div>
      <p
        data-field="returnAddress"
        className="absolute m-0 font-mono text-(--b-muted)"
        style={{ left: cq(6.8), top: cq(11), fontSize: "1.15cqi", letterSpacing: ".06em" }}
      >
        {returnAddress}
      </p>
      <div
        className="absolute border border-dashed border-(--b-line)"
        style={{
          left: cq(9.1),
          bottom: cq(6.8),
          width: cq(40.9),
          height: cq(20.5),
          borderRadius: ".6cqi",
        }}
      >
        <p
          className={`${monoUp} text-(--b-muted)`}
          style={{ padding: "1.4cqi", fontSize: ".85cqi", letterSpacing: ".12em" }}
        >
          address window · 90 × 45 mm
        </p>
      </div>
    </div>
  );
}

/** Email signature, 600×160 on white: name, role, one mono line, the lockup. */
export function EmailSignature({
  name = "Name Surname",
  role = "Founding engineer · Compozy",
  email = "name@compozy.com",
  url = "compozy.com",
}: {
  name?: string;
  role?: string;
  email?: string;
  url?: string;
}) {
  return (
    // .band-paper is unlayered CSS, so the white ground goes inline to win over its background.
    <div className={paper} style={{ background: "var(--color-fg-strong)" }}>
      <div
        className="absolute"
        style={{ left: cq(4), top: cq(4), bottom: cq(4), width: ".5cqi", ...gradientFill }}
      />
      <div className="absolute" style={{ left: cq(8), top: cq(5.4) }}>
        <p
          data-field="name"
          className="m-0 font-display font-medium tracking-[-0.02em]"
          style={{ fontSize: "4.2cqi", lineHeight: 1.2 }}
        >
          {name}
        </p>
        <p
          data-field="role"
          className="m-0 text-(--b-muted)"
          style={{ fontSize: "2.6cqi", marginTop: ".6cqi" }}
        >
          {role}
        </p>
        <p className="m-0 font-mono" style={{ fontSize: "2.3cqi", marginTop: "3.2cqi" }}>
          <span data-field="email">{email}</span> <span className="text-(--b-muted)">·</span>{" "}
          <span data-field="url" className="text-(--b-accent)">
            {url}
          </span>
        </p>
      </div>
      <div className="absolute" style={{ right: cq(4), top: cq(5.4) }}>
        <Mark size={4.2} variant="light" />
      </div>
    </div>
  );
}
