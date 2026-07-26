"use client";

import { useEffect, useReducer, useRef } from "react";
import { ArrowLeftRight, Pause, Play } from "lucide-react";
import { Button, Eyebrow, Pill } from "@agh/ui";
import { cn } from "@agh/ui/lib/utils";
import { AnimatedDiagram } from "./primitives/animated-diagram";
import { KIND_MEANING, type NetworkKind } from "./primitives/network-kinds";

type Lane = "A" | "NET" | "B";
type Direction = "->" | "<-" | "..";

type Step = {
  from: Lane;
  to: Lane;
  kind: NetworkKind;
  direction: Direction;
  payload: string;
  hint: string;
};

const STEPS: Step[] = [
  {
    from: "A",
    to: "NET",
    kind: "greet",
    direction: "->",
    payload: `{ peer_card: { peer_id: "coder.session-a", profiles_supported: ["agh-network/v0"], capabilities: ["code","review"], artifacts_supported: [], trust_modes_supported: ["unverified"] } }`,
    hint: "A Live coder announces itself inside the current daemon.",
  },
  {
    from: "B",
    to: "A",
    kind: "greet",
    direction: "<-",
    payload: `{ peer_card: { peer_id: "reviewer.session-b", profiles_supported: ["agh-network/v0"], capabilities: ["review"], artifacts_supported: [], trust_modes_supported: ["unverified"] } }`,
    hint: "A Live reviewer advertises its peer card; the daemon routes the committed greet to local members.",
  },
  {
    from: "A",
    to: "NET",
    kind: "whois",
    direction: "->",
    payload: `{ type: "request", query: "review release" }`,
    hint: "Coder asks for a local session that can review the release.",
  },
  {
    from: "B",
    to: "A",
    kind: "whois",
    direction: "<-",
    payload: `{ reply_to: "msg_whois_001", body: { type: "response", peer_card: { peer_id: "reviewer.session-b", profiles_supported: ["agh-network/v0"], capabilities: ["review"], artifacts_supported: [], trust_modes_supported: ["unverified"] } } }`,
    hint: "The matched Live reviewer owns the response; the daemon commits and routes it in process.",
  },
  {
    from: "A",
    to: "B",
    kind: "say",
    direction: "->",
    payload: `{ id: "msg_review_001", surface: "direct", direct_id: "direct_...", to: "reviewer.session-b", work_id: "work_release_review", body: { text: "review release" } }`,
    hint: "The direct message commits before the daemon wakes the addressed Live session.",
  },
  {
    from: "B",
    to: "A",
    kind: "trace",
    direction: "..",
    payload: `{ surface: "direct", direct_id: "direct_...", work_id: "work_release_review", body: { state: "working", message: "step 2 of 4" } }`,
    hint: "Reviewer records progress without turning conversation into task authority.",
  },
  {
    from: "B",
    to: "A",
    kind: "receipt",
    direction: "<-",
    payload: `{ surface: "direct", direct_id: "direct_...", work_id: "work_release_review", body: { for_id: "msg_review_001", status: "accepted" } }`,
    hint: "The collaboration records a durable receipt; provider usage is stored separately.",
  },
];

const STEP_DURATION_MS = 1600;

type State = { step: number; playing: boolean };
type Action =
  | { type: "next" }
  | { type: "prev" }
  | { type: "seek"; step: number }
  | { type: "play" }
  | { type: "pause" };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "next":
      return { ...state, step: (state.step + 1) % STEPS.length };
    case "prev":
      return { ...state, step: (state.step - 1 + STEPS.length) % STEPS.length };
    case "seek":
      return { ...state, step: action.step, playing: false };
    case "play":
      return { ...state, playing: true };
    case "pause":
      return { ...state, playing: false };
  }
}

function fromLabel(lane: Lane) {
  if (lane === "A") return "Coder · session-a";
  if (lane === "B") return "Reviewer · session-b";
  return "AGH daemon";
}

function directionGlyph(d: Direction, from: Lane, to: Lane) {
  if (d === "..") return "streaming";
  if (from === "A" && to === "NET") return "Agent A → Network";
  if (from === "NET" && to === "A") return "Network → Agent A";
  if (from === "A" && to === "B") return "Agent A → Agent B";
  if (from === "B" && to === "A") return "Agent B → Agent A";
  if (from === "NET" && to === "B") return "Network → Agent B";
  if (from === "B" && to === "NET") return "Agent B → Network";
  return `${from} → ${to}`;
}

export function NetworkProtocolVisual({ className }: { className?: string }) {
  return (
    <AnimatedDiagram className={className} ariaLabel="agh-network/v0 protocol walkthrough">
      {({ active, reducedMotion }) => <Inner active={active} reducedMotion={reducedMotion} />}
    </AnimatedDiagram>
  );
}

function Inner({ active, reducedMotion }: { active: boolean; reducedMotion: boolean }) {
  const [state, dispatch] = useReducer(reducer, { step: 0, playing: true });
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Auto-advance
  useEffect(() => {
    if (reducedMotion) return;
    if (!active) return;
    if (!state.playing) return;
    const id = window.setInterval(() => {
      dispatch({ type: "next" });
    }, STEP_DURATION_MS);
    return () => window.clearInterval(id);
  }, [active, state.playing, reducedMotion]);

  // Keyboard
  useEffect(() => {
    const node = containerRef.current;
    if (!node) return;
    const handler = (event: KeyboardEvent) => {
      if (document.activeElement !== node) return;
      if (event.key === "ArrowRight") {
        event.preventDefault();
        dispatch({ type: "next" });
        dispatch({ type: "pause" });
      } else if (event.key === "ArrowLeft") {
        event.preventDefault();
        dispatch({ type: "prev" });
        dispatch({ type: "pause" });
      } else if (event.key === " ") {
        event.preventDefault();
        dispatch({ type: state.playing ? "pause" : "play" });
      }
    };
    node.addEventListener("keydown", handler);
    return () => node.removeEventListener("keydown", handler);
  }, [state.playing]);

  const showAll = reducedMotion;

  return (
    <div
      ref={containerRef}
      tabIndex={0}
      role="group"
      aria-roledescription="protocol walkthrough"
      aria-label="agh-network/v0 seven-step in-process collaboration sequence"
      className="min-w-0 max-w-full overflow-hidden rounded-diagram border border-line bg-rail outline-none focus-visible:ring-2 focus-visible:ring-accent"
    >
      {/* Header , lane labels */}
      <div className="grid grid-cols-3 gap-2 border-b border-line bg-canvas-soft p-3 sm:gap-4 sm:px-4 md:px-6">
        <LaneHeader title="Session A" subtitle="coder · Live" />
        <LaneHeader title="AGH daemon" subtitle="agh-network/v0 · in-process" accent />
        <LaneHeader title="Session B" subtitle="reviewer · Live" />
      </div>

      {/* Body */}
      <div className="relative">
        {/* Lane lines , purely decorative vertical rulers */}
        <div className="pointer-events-none absolute inset-y-0 left-0 grid w-full grid-cols-3 gap-2 px-3 sm:gap-4 sm:px-4 md:px-6">
          <div className="relative flex justify-center">
            <div className="h-full w-px bg-line" />
          </div>
          <div className="relative flex justify-center">
            <div className="h-full w-px bg-accent-dim" />
          </div>
          <div className="relative flex justify-center">
            <div className="h-full w-px bg-line" />
          </div>
        </div>

        <ol className="relative flex min-w-0 flex-col gap-3 px-3 py-6 sm:px-4 md:px-6">
          {STEPS.map((step, i) => {
            const isCurrent = !showAll && i === state.step;
            const isPast = !showAll && i < state.step;
            const visible = showAll || i <= state.step;
            return (
              <li key={`${step.kind}-${step.from}-${step.to}-${step.direction}-${step.payload}`}>
                <button
                  type="button"
                  onClick={() => dispatch({ type: "seek", step: i })}
                  aria-current={isCurrent ? "step" : undefined}
                  aria-label={`Step ${i + 1} of ${STEPS.length}: ${step.kind} ${directionGlyph(step.direction, step.from, step.to)}`}
                  className={cn(
                    "group grid w-full grid-cols-[24px_1fr] items-start gap-3 rounded-icon-well border border-transparent p-2 text-left transition-all",
                    visible ? "opacity-100" : "pointer-events-none opacity-30",
                    isCurrent && "border-accent/55 bg-accent-tint/80",
                    isPast && "opacity-60"
                  )}
                >
                  <span
                    className={cn(
                      "mt-1 inline-flex size-5 items-center justify-center rounded-full font-mono text-badge font-semibold",
                      isCurrent ? "bg-accent text-accent-ink" : "bg-elevated text-subtle"
                    )}
                  >
                    {i + 1}
                  </span>

                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <Pill
                        mono
                        size="xs"
                        tone="accent"
                        solid={isCurrent}
                        title={KIND_MEANING[step.kind]}
                      >
                        {step.kind}
                      </Pill>
                      <span
                        className={cn(
                          "inline-flex min-w-0 flex-wrap items-center gap-1 font-mono text-eyebrow tracking-mono",
                          step.direction === ".." ? "text-subtle" : "text-muted"
                        )}
                      >
                        <span className="min-w-0 wrap-anywhere sm:whitespace-nowrap">
                          {fromLabel(step.from)}
                        </span>
                        <ArrowGlyph direction={step.direction} />
                        <span className="min-w-0 wrap-anywhere sm:whitespace-nowrap">
                          {fromLabel(step.to)}
                        </span>
                      </span>
                    </div>

                    <pre className="mt-1.5 overflow-x-auto font-mono text-xs leading-6 text-fg">
                      <code>{step.payload}</code>
                    </pre>

                    {isCurrent ? (
                      <p className="mt-1 text-xs leading-relaxed text-subtle">{step.hint}</p>
                    ) : null}
                  </div>
                </button>
              </li>
            );
          })}
        </ol>
      </div>

      {/* Footer , controls + kind footnote */}
      <div className="flex min-w-0 flex-col gap-3 border-t border-line bg-canvas-soft p-3 sm:px-4 md:flex-row md:items-center md:justify-between md:px-6">
        <p className="min-w-0 font-mono text-eyebrow text-subtle">
          <span className="text-accent">capability</span> transfers full capability artifacts.{" "}
          <span className="text-accent">say</span> is free-form operator chat.
        </p>
        {!showAll ? (
          <div className="flex items-center gap-1.5">
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => dispatch({ type: "prev" })}
              aria-label="Previous step"
            >
              <ArrowLeftRight aria-hidden className="rotate-180" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => dispatch({ type: state.playing ? "pause" : "play" })}
              aria-label={state.playing ? "Pause walkthrough" : "Play walkthrough"}
              className="eyebrow"
            >
              {state.playing ? <Pause aria-hidden /> : <Play aria-hidden />}
              {state.playing ? "pause" : "play"}
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => dispatch({ type: "next" })}
              aria-label="Next step"
            >
              <ArrowLeftRight aria-hidden />
            </Button>
            <span className="ml-2 font-mono text-badge tracking-mono text-subtle">
              {state.step + 1} / {STEPS.length}
            </span>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function LaneHeader({
  title,
  subtitle,
  accent = false,
}: {
  title: string;
  subtitle: string;
  accent?: boolean;
}) {
  return (
    <div className="min-w-0 text-center">
      <Eyebrow className={cn(accent ? "text-accent" : "text-subtle")}>{title}</Eyebrow>
      <p className="mt-0.5 font-mono text-eyebrow text-muted wrap-anywhere">{subtitle}</p>
    </div>
  );
}

function ArrowGlyph({ direction }: { direction: Direction }) {
  if (direction === "->") {
    return <span aria-hidden="true">→</span>;
  }
  if (direction === "<-") {
    return <span aria-hidden="true">←</span>;
  }
  return <span aria-hidden="true">··</span>;
}
