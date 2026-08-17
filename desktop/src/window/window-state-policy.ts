export interface Rectangle {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export interface PersistedWindowState extends Rectangle {
  readonly maximized: boolean;
}

const DEFAULT_BOUNDS: Rectangle = { x: 0, y: 0, width: 1280, height: 800 };
export const MINIMUM_WINDOW_SIZE = { width: 900, height: 600 } as const;
const REQUIRED_VISIBLE_EDGE = 48;

function defaultBounds(displays: readonly Rectangle[]): Rectangle {
  const display = displays[0];
  if (!display) return DEFAULT_BOUNDS;
  const width = Math.min(DEFAULT_BOUNDS.width, display.width);
  const height = Math.min(DEFAULT_BOUNDS.height, display.height);
  return {
    x: display.x + Math.floor((display.width - width) / 2),
    y: display.y + Math.floor((display.height - height) / 2),
    width,
    height,
  };
}

function finiteInteger(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? Math.round(value) : null;
}

export function parseWindowState(value: unknown): PersistedWindowState | null {
  if (!value || typeof value !== "object") return null;
  const record = value as Record<string, unknown>;
  const x = finiteInteger(record.x);
  const y = finiteInteger(record.y);
  const width = finiteInteger(record.width);
  const height = finiteInteger(record.height);
  if (
    x === null ||
    y === null ||
    width === null ||
    height === null ||
    typeof record.maximized !== "boolean"
  ) {
    return null;
  }
  return { x, y, width, height, maximized: record.maximized };
}

function overlap(left: Rectangle, right: Rectangle): { width: number; height: number } {
  return {
    width: Math.max(
      0,
      Math.min(left.x + left.width, right.x + right.width) - Math.max(left.x, right.x)
    ),
    height: Math.max(
      0,
      Math.min(left.y + left.height, right.y + right.height) - Math.max(left.y, right.y)
    ),
  };
}

export function restoreWindowBounds(
  state: PersistedWindowState | null,
  displays: readonly Rectangle[]
): Rectangle {
  if (
    !state ||
    state.width < MINIMUM_WINDOW_SIZE.width ||
    state.height < MINIMUM_WINDOW_SIZE.height
  ) {
    return defaultBounds(displays);
  }
  const visible = displays.some(display => {
    const intersection = overlap(state, display);
    return (
      intersection.width >= REQUIRED_VISIBLE_EDGE && intersection.height >= REQUIRED_VISIBLE_EDGE
    );
  });
  return visible ? { ...state } : defaultBounds(displays);
}
