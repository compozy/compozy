// Diagram · Task stack: one task in three steps (task, diff, checks) stepped down into the band.
// Status marks are the product's own StateGlyph; colours are band variables over @compozy/ui
// tokens (ADR-005 §3).
import { StateGlyph, type StateGlyphState } from "@compozy/ui";
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

type DiffKind = "+" | "-" | " ";

interface StackDiagramProps {
  x: number;
  y: number;
  id?: string;
  title?: string;
  agent?: string;
  worktree?: string;
  loop?: string;
  file?: string;
  diff?: [DiffKind, string][];
  checks?: [string, "passed" | "running"][];
}

const DEFAULT_DIFF: [DiffKind, string][] = [
  ["-", "const ttl = 3600"],
  ["+", "const ttl = config.session.ttl"],
  ["+", "if (isExpired(token)) return rotate(token)"],
  [" ", "return token"],
  ["+", "audit.log('token.rotated', id)"],
];
const DEFAULT_CHECKS: [string, "passed" | "running"][] = [
  ["typecheck", "passed"],
  ["unit · 212", "passed"],
  ["e2e · auth", "running"],
];
const GLYPH: Record<"passed" | "running", StateGlyphState> = { passed: "done", running: "running" };

const muted = "text-(--b-muted,var(--color-muted-foreground))";
const line = "border-(--b-line,var(--color-line))";
const live = "text-(--b-accent,var(--color-brand-cyan))";

function Plate({ i, x, y, children }: { i: number; x: number; y: number; children: ReactNode }) {
  return (
    <div
      className={cn(
        "absolute h-[23cqw] w-[40cqw] rounded-[0.9cqw] border bg-(--b-card,var(--color-canvas)) px-[1.5cqw] py-[1.3cqw]",
        "shadow-[0_2.6cqw_5cqw_color-mix(in_oklab,var(--color-rail)_55%,transparent)]",
        "[mask-image:linear-gradient(180deg,black_35%,transparent_100%)]",
        line
      )}
      style={{
        left: `calc(${x * 100}cqw - 32cqw + ${i * 12.5}cqw)`,
        top: `calc(${y * 100}cqh - 26cqw + ${i * 14.5}cqw)`,
        transform: "perspective(160cqw) rotateX(48deg) rotateZ(-30deg)",
      }}
    >
      {children}
    </div>
  );
}

function Row({ k, children }: { k: string; children: ReactNode }) {
  return (
    <div
      className={cn(
        "flex justify-between border-b font-mono text-[0.8cqw] leading-[2.3]",
        line,
        muted
      )}
    >
      <span>{k}</span>
      {children}
    </div>
  );
}

const label = cn("m-0 font-mono text-[0.7cqw] uppercase tracking-[0.06em]", muted);

export function StackDiagram({
  x,
  y,
  id = "AUTH-142",
  title = "Rotate session tokens on refresh",
  agent = "claude-code",
  worktree = "feat/auth",
  loop = "2 of 5",
  file = "src/session/token.ts",
  diff = DEFAULT_DIFF,
  checks = DEFAULT_CHECKS,
}: StackDiagramProps) {
  return (
    <div data-diagram="stack" className="absolute inset-0 text-(--b-fg,var(--color-fg))">
      <Plate i={0} x={x} y={y}>
        <p className={label}>Task · {id}</p>
        <p className="mt-[0.8cqw] mb-[1cqw] text-[1.6cqw] font-medium tracking-[-0.02em]">
          {title}
        </p>
        <Row k="status">
          <span className={cn("inline-flex items-center gap-[0.4cqw]", live)}>
            <StateGlyph state="running" size="sm" still /> running
          </span>
        </Row>
        <Row k="agent">
          <span>{agent}</span>
        </Row>
        <Row k="worktree">
          <span>{worktree}</span>
        </Row>
        <Row k="loop">
          <span>{loop}</span>
        </Row>
      </Plate>
      <Plate i={1} x={x} y={y}>
        <p className={label}>Diff · {file}</p>
        <div
          className={cn("mt-[0.8cqw] font-mono text-[0.8cqw] leading-[1.95] whitespace-pre", muted)}
        >
          {diff.map(([k, l], n) => (
            <div
              key={n}
              className={cn(
                k === "+" &&
                  cn(live, "bg-[color-mix(in_oklab,var(--color-brand-cyan)_7%,transparent)]"),
                k === "-" && "line-through opacity-55"
              )}
            >
              {k} {l}
            </div>
          ))}
        </div>
      </Plate>
      <Plate i={2} x={x} y={y}>
        <p className={label}>Checks</p>
        {checks.map(([n, s]) => (
          <Row key={n} k={n}>
            <span className={cn("inline-flex items-center gap-[0.4cqw]", live)}>
              <StateGlyph state={GLYPH[s]} size="sm" still /> {s}
            </span>
          </Row>
        ))}
      </Plate>
    </div>
  );
}
