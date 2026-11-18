// Diagram · Run lanes: one lane per agent, one bar per run, the now line at the right edge; runs
// still live at "now" carry the live gradient (ADR-005 §3).
import { LiveGradient } from "./live-gradient";
import { seeded } from "./seeded";

const DEFAULT_LANES = [
  "claude-code",
  "codex",
  "gemini",
  "claude-code",
  "codex",
  "opencode",
  "claude-code",
  "codex",
  "gemini",
  "claude-code",
  "codex",
  "claude-code",
  "opencode",
  "codex",
];

interface LanesDiagramProps {
  lanes?: string[];
  seed?: number;
  labels?: boolean;
}

const R = 97;
const T = 9;

export function LanesDiagram({
  lanes = DEFAULT_LANES,
  seed = 7,
  labels = true,
}: LanesDiagramProps) {
  const rnd = seeded(seed);
  const L = labels ? 14 : 3;
  const rh = 82 / lanes.length;
  const rows = lanes.map((name, i) => {
    const bars: { x: number; w: number; live: boolean }[] = [];
    let x = L + rnd() * 6;
    while (x < R - 2) {
      const w = 3 + rnd() * 18;
      const e = Math.min(x + w, R);
      if (e - x > 1) bars.push({ x, w: e - x, live: e >= R - 0.01 });
      x += w + 1.5 + rnd() * 7;
    }
    return { name, y: T + i * rh, bars };
  });
  const bottom = T + lanes.length * rh;
  const muted = "text-(--b-muted,var(--color-muted-foreground))";
  return (
    <div data-diagram="lanes" className="absolute inset-0 text-(--b-fg,var(--color-fg))">
      <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="absolute inset-0 size-full">
        <LiveGradient id="dg-lanes" />
        {Array.from({ length: 28 }, (_, i) => {
          const gx = L + i * ((R - L) / 27);
          return (
            <line
              key={i}
              x1={gx}
              x2={gx}
              y1={T - 3}
              y2={bottom}
              stroke="currentColor"
              strokeOpacity={0.05}
              vectorEffect="non-scaling-stroke"
            />
          );
        })}
        {rows.map((r, ri) =>
          r.bars.map((b, bi) => (
            <rect
              key={`${ri}-${bi}`}
              x={b.x}
              y={r.y + rh * 0.18}
              width={b.w}
              height={rh * 0.42}
              rx={0.25}
              fill={b.live ? "url(#dg-lanes)" : "currentColor"}
              fillOpacity={b.live ? 1 : 0.13}
            />
          ))
        )}
        <line
          x1={R}
          x2={R}
          y1={T - 5}
          y2={bottom + 1}
          className="stroke-(--b-accent,var(--color-brand-cyan))"
          strokeWidth={1.5}
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      {labels &&
        rows.map((r, i) => (
          <span
            key={i}
            className={`absolute -translate-y-1/2 font-mono text-[0.8cqw] opacity-80 ${muted}`}
            style={{ top: `${r.y + rh * 0.39}%`, right: `${100 - L + 1.2}%` }}
          >
            {r.name}
          </span>
        ))}
      <span
        className="absolute font-mono text-[0.8cqw] text-(--b-accent,var(--color-brand-cyan))"
        style={{ top: `${T - 5.5}%`, right: `${100 - R + 0.8}%` }}
      >
        now
      </span>
    </div>
  );
}
