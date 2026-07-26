import type { DesignSystem, Page, SlideMeta } from "@open-slide/core";

export const design: DesignSystem = {
  palette: { bg: "#141312", text: "#E5E5E7", accent: "#E8572A" },
  fonts: {
    display: '"Playfair Display", "Inter Variable", Georgia, serif',
    body: '"Inter Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif',
  },
  typeScale: { hero: 168, body: 36 },
  radius: 8,
};

const surface = "#1E1C1B";
const border = "#3C3A39";
const textSoft = "#8E8E93";
const label = "#98989D";
const mono = '"JetBrains Mono", "SF Mono", ui-monospace, Menlo, monospace';

const styles = `
  @import url("https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@500;600&family=Playfair+Display:wght@400&display=swap");

  @keyframes aghRise {
    from { opacity: 0; transform: translateY(16px); }
    to   { opacity: 1; transform: translateY(0); }
  }

  .agh-rise { opacity: 1; transform: translateY(0); }

  @media (prefers-reduced-motion: no-preference) {
    .agh-rise   { animation: aghRise 200ms cubic-bezier(0.2, 0, 0, 1) both; }
    .agh-rise-1 { animation-delay: 0ms;   }
    .agh-rise-2 { animation-delay: 60ms;  }
    .agh-rise-3 { animation-delay: 120ms; }
    .agh-rise-4 { animation-delay: 180ms; }
  }
`;

const fill = {
  width: "100%",
  height: "100%",
  fontFamily: "var(--osd-font-body)",
  position: "relative",
} as const;

const Eyebrow = ({
  children,
  tone = "accent",
}: {
  children: React.ReactNode;
  tone?: "accent" | "label";
}) => (
  <div
    style={{
      fontFamily: mono,
      fontSize: 22,
      fontWeight: 600,
      letterSpacing: "0.16em",
      textTransform: "uppercase",
      color: tone === "accent" ? "var(--osd-accent)" : label,
    }}
  >
    {children}
  </div>
);

const Footer = ({ pageNum, total }: { pageNum: number; total: number }) => (
  <div
    style={{
      position: "absolute",
      left: 120,
      right: 120,
      bottom: 60,
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
      fontFamily: mono,
      fontSize: 22,
      fontWeight: 600,
      letterSpacing: "0.16em",
      textTransform: "uppercase",
      color: label,
    }}
  >
    <span style={{ display: "inline-flex", alignItems: "center", gap: 16 }}>
      <span
        style={{
          width: 8,
          height: 8,
          borderRadius: 9999,
          background: "var(--osd-accent)",
        }}
      />
      AGH · 2026
    </span>
    <span>
      {String(pageNum).padStart(2, "0")} / {String(total).padStart(2, "0")}
    </span>
  </div>
);

const TOTAL = 4;

const Cover: Page = () => (
  <>
    <style>{styles}</style>
    <div
      style={{
        ...fill,
        background: "var(--osd-bg)",
        color: "var(--osd-text)",
        padding: 120,
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
      }}
    >
      <div className="agh-rise agh-rise-1" style={{ marginBottom: 80 }}>
        <Eyebrow>AGH · Network · Differentiator</Eyebrow>
      </div>

      <h1
        className="agh-rise agh-rise-2"
        style={{
          fontFamily: "var(--osd-font-display)",
          fontSize: "var(--osd-size-hero)",
          fontWeight: 400,
          lineHeight: 0.96,
          letterSpacing: "-0.035em",
          margin: 0,
        }}
      >
        Built-in network.
        <br />
        <span style={{ color: "var(--osd-accent)" }}>Delegate. Deliver.</span> Done.
      </h1>

      <p
        className="agh-rise agh-rise-3"
        style={{
          fontFamily: "var(--osd-font-body)",
          fontSize: "var(--osd-size-body)",
          fontWeight: 400,
          lineHeight: 1.5,
          color: textSoft,
          maxWidth: 1280,
          margin: "40px 0 0",
        }}
      >
        Open agent network protocol — implemented in the alpha runtime. Agents discover each other,
        share capabilities, and close work with receipts.
      </p>

      <Footer pageNum={1} total={TOTAL} />
    </div>
  </>
);

const Definition: Page = () => (
  <>
    <style>{styles}</style>
    <div
      style={{
        ...fill,
        background: "var(--osd-bg)",
        color: "var(--osd-text)",
        padding: 120,
        display: "flex",
        flexDirection: "column",
      }}
    >
      <div className="agh-rise agh-rise-1" style={{ marginBottom: 80 }}>
        <Eyebrow tone="label">What it is</Eyebrow>
      </div>

      <h2
        className="agh-rise agh-rise-2"
        style={{
          fontFamily: "var(--osd-font-display)",
          fontSize: 96,
          fontWeight: 400,
          lineHeight: 1.02,
          letterSpacing: "-0.03em",
          margin: 0,
        }}
      >
        <span
          style={{
            fontFamily: mono,
            fontSize: 80,
            color: "var(--osd-accent)",
            letterSpacing: "-0.02em",
          }}
        >
          agh-network/v0
        </span>
        <br />
        Open agent coordination protocol.
      </h2>

      <p
        className="agh-rise agh-rise-3"
        style={{
          fontFamily: "var(--osd-font-body)",
          fontSize: "var(--osd-size-body)",
          fontWeight: 400,
          lineHeight: 1.5,
          color: textSoft,
          maxWidth: 1280,
          margin: "64px 0 0",
        }}
      >
        Start two explicit Live sessions on one daemon. Accepted messages commit to SQLite before an
        addressed or mentioned recipient is notified in-process, while finite wake, depth, token,
        and wall-time bounds keep collaboration accountable.
      </p>

      <div
        className="agh-rise agh-rise-4"
        style={{
          display: "flex",
          gap: 16,
          marginTop: 64,
          flexWrap: "wrap",
        }}
      >
        {["Open protocol", "Commit-first runtime", "Bounded Live opt-in"].map(t => (
          <span
            key={t}
            style={{
              fontFamily: mono,
              fontSize: 22,
              fontWeight: 600,
              letterSpacing: "0.06em",
              textTransform: "uppercase",
              color: label,
              padding: "12px 20px",
              border: `1px solid ${border}`,
              borderRadius: "var(--osd-radius)",
              background: surface,
            }}
          >
            {t}
          </span>
        ))}
      </div>

      <Footer pageNum={2} total={TOTAL} />
    </div>
  </>
);

const KINDS: { name: string; role: string }[] = [
  { name: "greet", role: "Peer announces presence" },
  { name: "whois", role: "Identity lookup" },
  { name: "say", role: "Thread or direct-room message" },
  { name: "capability", role: "Offer or request" },
  { name: "receipt", role: "Proof of delivery" },
  { name: "trace", role: "Audit identifier" },
];

const Kinds: Page = () => (
  <>
    <style>{styles}</style>
    <div
      style={{
        ...fill,
        background: "var(--osd-bg)",
        color: "var(--osd-text)",
        padding: 120,
        display: "flex",
        flexDirection: "column",
      }}
    >
      <div className="agh-rise agh-rise-1" style={{ marginBottom: 80 }}>
        <Eyebrow>What you send</Eyebrow>
      </div>

      <h2
        className="agh-rise agh-rise-2"
        style={{
          fontFamily: "var(--osd-font-display)",
          fontSize: 96,
          fontWeight: 400,
          lineHeight: 1.02,
          letterSpacing: "-0.03em",
          margin: 0,
        }}
      >
        Six message kinds.
      </h2>

      <div
        className="agh-rise agh-rise-3"
        style={{
          marginTop: 64,
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: 24,
        }}
      >
        {KINDS.map(({ name, role }) => (
          <div
            key={name}
            style={{
              padding: "28px 28px",
              border: `1px solid ${border}`,
              borderRadius: "var(--osd-radius)",
              background: surface,
              display: "flex",
              flexDirection: "column",
              gap: 16,
            }}
          >
            <span
              style={{
                fontFamily: mono,
                fontSize: 32,
                fontWeight: 600,
                color: "var(--osd-accent)",
                letterSpacing: "-0.01em",
              }}
            >
              {name}
            </span>
            <span
              style={{
                fontFamily: "var(--osd-font-body)",
                fontSize: 22,
                fontWeight: 400,
                lineHeight: 1.4,
                color: textSoft,
              }}
            >
              {role}
            </span>
          </div>
        ))}
      </div>

      <p
        className="agh-rise agh-rise-4"
        style={{
          fontFamily: "var(--osd-font-body)",
          fontSize: 28,
          lineHeight: 1.5,
          color: label,
          marginTop: 56,
          maxWidth: 1280,
        }}
      >
        Discovery, conversation, delegation, audit — composable, replayable, persisted to the audit
        log.
      </p>

      <Footer pageNum={3} total={TOTAL} />
    </div>
  </>
);

const Closing: Page = () => (
  <>
    <style>{styles}</style>
    <div
      style={{
        ...fill,
        background: "var(--osd-bg)",
        color: "var(--osd-text)",
        padding: 120,
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
      }}
    >
      <div className="agh-rise agh-rise-1" style={{ marginBottom: 64 }}>
        <Eyebrow>Read the spec</Eyebrow>
      </div>

      <h2
        className="agh-rise agh-rise-2"
        style={{
          fontFamily: "var(--osd-font-display)",
          fontSize: 128,
          fontWeight: 400,
          lineHeight: 1.0,
          letterSpacing: "-0.035em",
          margin: 0,
        }}
      >
        Open. Auditable.
        <br />
        <span style={{ color: "var(--osd-accent)" }}>Yours to implement.</span>
      </h2>

      <div
        className="agh-rise agh-rise-3"
        style={{
          marginTop: 56,
          display: "inline-flex",
          alignItems: "center",
          gap: 20,
          padding: "20px 28px",
          border: `1px solid ${border}`,
          borderRadius: "var(--osd-radius)",
          background: surface,
          alignSelf: "flex-start",
        }}
      >
        <span
          style={{
            fontFamily: mono,
            fontSize: 24,
            fontWeight: 600,
            color: label,
            letterSpacing: "0.06em",
            textTransform: "uppercase",
          }}
        >
          Spec
        </span>
        <span
          style={{
            fontFamily: mono,
            fontSize: 36,
            fontWeight: 600,
            color: "var(--osd-text)",
            letterSpacing: "-0.01em",
          }}
        >
          agh.network/protocol
        </span>
      </div>

      <p
        className="agh-rise agh-rise-4"
        style={{
          fontFamily: "var(--osd-font-body)",
          fontSize: "var(--osd-size-body)",
          fontWeight: 400,
          lineHeight: 1.5,
          color: textSoft,
          maxWidth: 1280,
          margin: "40px 0 0",
        }}
      >
        Every other agent tool stops at the single-runtime boundary. agh-network/v0 is the open
        agent network protocol — implementable outside AGH.
      </p>

      <Footer pageNum={4} total={TOTAL} />
    </div>
  </>
);

export const meta: SlideMeta = { title: "AGH Network · agh-network/v0" };
export default [Cover, Definition, Kinds, Closing] satisfies Page[];
