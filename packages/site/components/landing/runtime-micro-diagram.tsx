"use client";

import { cn } from "@agh/ui/lib/utils";
import { useReducedMotion } from "./primitives/use-reduced-motion";

const SUBSYSTEMS = [
  { id: "sessions", label: "Sessions", note: "event store" },
  { id: "memory", label: "Memory", note: "global + ws" },
  { id: "skills", label: "Skills", note: "catalog" },
  { id: "workspaces", label: "Workspaces", note: "overlays" },
  { id: "observe", label: "Observe", note: "events + health" },
];

/**
 * 240×320 SVG showing the AGH daemon and its five subsystems with a subtle
 * highlight cycling through them , communicates "the daemon has real internals"
 * without re-rendering the full architecture diagram.
 */
export function RuntimeMicroDiagram({ className }: { className?: string }) {
  const reducedMotion = useReducedMotion();
  const cycleDuration = SUBSYSTEMS.length * 1.2;

  return (
    <div
      className={cn(
        "relative aspect-3/4 w-full max-w-65 overflow-hidden rounded-diagram border border-line bg-rail",
        className
      )}
      aria-hidden="true"
    >
      <svg aria-hidden="true" focusable="false" viewBox="0 0 240 320" className="size-full">
        {/* Daemon box */}
        <rect
          x={20}
          y={20}
          width={200}
          height={52}
          rx={8}
          fill="var(--color-accent-tint)"
          stroke="var(--color-accent)"
          strokeWidth={1}
        />
        <text
          x={120}
          y={44}
          textAnchor="middle"
          fontFamily="var(--font-mono)"
          fontSize="10"
          fontWeight={600}
          fill="var(--color-accent)"
          letterSpacing="0.08em"
        >
          AGH DAEMON
        </text>
        <text
          x={120}
          y={60}
          textAnchor="middle"
          fontFamily="var(--font-mono)"
          fontSize="9"
          fill="var(--color-muted)"
        >
          pid 42871 · sqlite
        </text>

        {/* Subsystems */}
        {SUBSYSTEMS.map((sub, i) => {
          const y = 92 + i * 44;
          return (
            <g key={sub.id}>
              <line
                x1={120}
                y1={y - 10}
                x2={120}
                y2={y + 2}
                stroke="var(--color-line)"
                strokeWidth={1}
              />
              <rect
                className={reducedMotion ? undefined : "agh-subsystem"}
                style={
                  reducedMotion
                    ? undefined
                    : {
                        animationDelay: `${i * 1.2}s`,
                        animationDuration: `${cycleDuration}s`,
                      }
                }
                x={32}
                y={y + 2}
                width={176}
                height={30}
                rx={6}
                fill="var(--color-canvas-soft)"
                stroke="var(--color-line)"
                strokeWidth={1}
              />
              <text
                x={44}
                y={y + 22}
                fontFamily="var(--font-sans)"
                fontSize="12"
                fontWeight={500}
                fill="var(--color-fg)"
              >
                {sub.label}
              </text>
              <text
                x={196}
                y={y + 22}
                textAnchor="end"
                fontFamily="var(--font-mono)"
                fontSize="9.5"
                fill="var(--color-subtle)"
              >
                {sub.note}
              </text>
            </g>
          );
        })}
      </svg>

      {!reducedMotion && (
        <style>{`
          .agh-subsystem {
            animation-name: agh-subsystem-pulse;
            animation-iteration-count: infinite;
            animation-timing-function: var(--ease-in-out);
          }
          @media (prefers-reduced-motion: reduce) {
            .agh-subsystem {
              animation: none !important;
            }
          }
          @keyframes agh-subsystem-pulse {
            0%, 100% {
              fill: var(--color-canvas-soft);
              stroke: var(--color-line);
            }
            10%, 30% {
              fill: var(--color-accent-tint);
              stroke: var(--color-accent);
            }
          }
        `}</style>
      )}
    </div>
  );
}
