import type { ReactNode } from "react";

/**
 * The behaviour options, drawn. Each picture keeps the accent for one thing —
 * "the window this is about" — so selecting an option never repaints the
 * drawing: the colour is information, and it means the same in every state.
 */
function Diagram({ children }: { children: ReactNode }) {
  return (
    <svg aria-hidden="true" className="h-9 w-full text-subtle" fill="none" viewBox="0 0 88 38">
      {children}
    </svg>
  );
}

function Pane({
  x,
  y,
  width,
  height,
  opacity = 0.16,
}: {
  x: number;
  y: number;
  width: number;
  height: number;
  opacity?: number;
}) {
  return (
    <rect
      fill="currentColor"
      fillOpacity={opacity}
      height={height}
      rx="2"
      stroke="currentColor"
      strokeOpacity="0.55"
      width={width}
      x={x}
      y={y}
    />
  );
}

function Subject({ x, y, width, height }: { x: number; y: number; width: number; height: number }) {
  return (
    <rect
      className="fill-accent stroke-accent"
      fillOpacity="0.32"
      height={height}
      rx="2"
      width={width}
      x={x}
      y={y}
    />
  );
}

export function DiagramFloatOnTop() {
  return (
    <Diagram>
      <Pane height={30} width={38} x={3} y={4} />
      <Pane height={30} width={40} x={45} y={4} />
      <rect className="fill-elevated stroke-accent" height="24" rx="2" width="40" x="26" y="12" />
    </Diagram>
  );
}

export function DiagramSplitBesideFocus() {
  return (
    <Diagram>
      <Pane height={30} width={38} x={3} y={4} />
      <Subject height={30} width={40} x={45} y={4} />
    </Diagram>
  );
}

export function DiagramFoldIntoStack() {
  return (
    <Diagram>
      <Pane height={26} opacity={0.1} width={50} x={6} y={8} />
      <Pane height={26} width={50} x={14} y={6} />
      <Subject height={30} width={50} x={22} y={4} />
    </Diagram>
  );
}

export function DiagramRefuse() {
  return (
    <Diagram>
      <rect
        height="30"
        rx="2"
        stroke="currentColor"
        strokeDasharray="3 3"
        strokeOpacity="0.45"
        width="80"
        x="4"
        y="4"
      />
      <path
        d="m36 12 16 14M52 12 36 26"
        stroke="currentColor"
        strokeLinecap="round"
        strokeWidth="1.6"
      />
    </Diagram>
  );
}

export function DiagramClickAndKeys() {
  return (
    <Diagram>
      <Pane height={30} width={38} x={3} y={4} />
      <Subject height={30} width={40} x={45} y={4} />
      <path className="fill-fg-strong" d="m58 16 10 9-4 1 2 5-2 1-2-5-3 3z" />
    </Diagram>
  );
}

export function DiagramKeysOnly() {
  return (
    <Diagram>
      <Pane height={30} width={38} x={3} y={4} />
      <Subject height={30} width={40} x={45} y={4} />
      <path
        className="stroke-fg-strong"
        d="M69 19h-12M57 19l4-4M57 19l4 4"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.6"
      />
    </Diagram>
  );
}

export function DiagramDragWindow() {
  return (
    <Diagram>
      <Pane height={26} width={26} x={3} y={8} />
      <Pane height={26} width={26} x={33} y={8} />
      <rect className="fill-elevated stroke-accent" height="24" rx="2" width="32" x="52" y="2" />
    </Diagram>
  );
}

export function DiagramDragGroup() {
  return (
    <Diagram>
      <Pane height={26} width={26} x={3} y={8} />
      <rect className="fill-elevated stroke-accent" height="24" rx="2" width="20" x="42" y="2" />
      <rect className="fill-elevated stroke-accent" height="24" rx="2" width="20" x="64" y="2" />
    </Diagram>
  );
}

export function DiagramSlide() {
  return (
    <Diagram>
      <Pane height={26} width={34} x={2} y={6} />
      <Subject height={26} width={34} x={50} y={6} />
      <path
        className="stroke-fg-strong"
        d="M40 19h8M44 15l4 4-4 4"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.5"
      />
    </Diagram>
  );
}

export function DiagramCrossfade() {
  return (
    <Diagram>
      <Pane height={26} opacity={0.08} width={40} x={12} y={6} />
      <rect
        className="fill-accent stroke-accent"
        fillOpacity="0.2"
        height="26"
        rx="2"
        strokeOpacity="0.7"
        width="40"
        x="36"
        y="6"
      />
    </Diagram>
  );
}

export function DiagramInstant() {
  return (
    <Diagram>
      <Pane height={26} opacity={0.08} width={40} x={2} y={6} />
      <Subject height={26} width={40} x={46} y={6} />
      <path
        className="stroke-fg-strong"
        d="M44 2v34"
        strokeDasharray="2 2"
        strokeOpacity="0.5"
        strokeWidth="1.2"
      />
    </Diagram>
  );
}
