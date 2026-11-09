// Diagram · Activity: thin bars across a day, the live window in the live gradient (ADR-005 §3).
import { LiveGradient } from "./live-gradient";
import { seeded } from "./seeded";

interface BarsDiagramProps {
  n?: number;
  seed?: number;
  window?: [number, number];
  axis?: boolean;
}

const L = 7;
const W = 86;
const BASE = 84;

export function BarsDiagram({
  n = 150,
  seed = 7,
  window = [0.62, 0.8],
  axis = true,
}: BarsDiagramProps) {
  const rnd = seeded(seed);
  const bars = Array.from({ length: n }, (_, i) => {
    const t = i / n;
    const v = 0.25 + 0.6 * Math.sin(t * 3.1 + 0.4) ** 2 * (0.6 + 0.4 * rnd());
    return { t, h: v * 62, o: 0.1 + 0.2 * rnd() };
  });
  return (
    <div data-diagram="bars" className="absolute inset-0 text-(--b-fg,var(--color-fg))">
      <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="absolute inset-0 size-full">
        <LiveGradient id="dg-bars" />
        {bars.map((b, i) => {
          const hot = b.t > window[0] && b.t < window[1];
          return (
            <rect
              key={i}
              x={L + (i * W) / n}
              y={BASE - b.h}
              width={(W / n) * 0.55}
              height={b.h}
              fill={hot ? "url(#dg-bars)" : "currentColor"}
              fillOpacity={hot ? 1 : b.o}
            />
          );
        })}
        {axis && (
          <line
            x1={L}
            x2={L + W}
            y1={BASE + 1.5}
            y2={BASE + 1.5}
            stroke="currentColor"
            strokeOpacity={0.15}
            vectorEffect="non-scaling-stroke"
          />
        )}
      </svg>
      {axis &&
        Array.from({ length: 9 }, (_, i) => (
          <span
            key={i}
            className="absolute font-mono text-[0.75cqw] text-(--b-muted,var(--color-muted-foreground)) opacity-70"
            style={{ left: `${L + (i * W) / 8}%`, top: `${BASE + 3.5}%` }}
          >
            {String(i * 3).padStart(2, "0")}:00
          </span>
        ))}
    </div>
  );
}
