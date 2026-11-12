import { useEffect, useRef, useState } from "react";

/** One always-on agent runs along the point line; a miss hands the run to the next one. */
export const RUNNERS = [
  "checkout-lead",
  "qa-runner",
  "docs-steward",
  "release-manager",
  "payments-codex",
  "test-writer",
  "platform-engineer",
] as const;

export type RunnerMood = "working" | "failed" | "paused";

const TRACK = 300;
const RUNNER_X = 28;
const RUNNER_SIZE = 32;
const OBSTACLES = 3;
const JUMP = 330;
const GRAVITY = 1100;
const SPEED = { start: 150, max: 230, gain: 2 };
/** How long a key press keeps the autopilot off. */
const MANUAL_HOLD = 6000;
const HANDOFF = 1100;

interface Obstacle {
  x: number;
  /** Size in points on the 8 px pitch. */
  cols: number;
  rows: number;
  cleared: boolean;
}

function spawn(after: number): Obstacle {
  return {
    x: after + 190 + Math.random() * 150,
    cols: Math.random() < 0.3 ? 2 : 1,
    rows: Math.random() < 0.5 ? 3 : 2,
    cleared: false,
  };
}

function field(): Obstacle[] {
  const obstacles: Obstacle[] = [];
  let after = TRACK - 190;
  for (let index = 0; index < OBSTACLES; index += 1) {
    const next = spawn(after);
    obstacles.push(next);
    after = next.x;
  }
  return obstacles;
}

/**
 * The boot window's waiting game. It plays itself (the autopilot never misses), and Space or a
 * click takes over for a few seconds. Positions are written straight to the elements, so a frame
 * never re-renders React; only the mood, the score and the runner change state.
 */
export function useBootRunner({ autoplay }: { autoplay: boolean }) {
  const runner = useRef<HTMLSpanElement>(null);
  const ground = useRef<HTMLSpanElement>(null);
  const blocks = useRef<(HTMLSpanElement | null)[]>([]);
  const jump = useRef<() => void>(() => {});
  const [still] = useState(() => window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  const [mood, setMood] = useState<RunnerMood>(autoplay && !still ? "working" : "paused");
  const [score, setScore] = useState(0);
  const [agent, setAgent] = useState(0);

  useEffect(() => {
    if (still) return;
    let obstacles = field();
    let running = autoplay;
    let y = 0;
    let velocity = 0;
    let speed = SPEED.start;
    let travelled = 0;
    let manualUntil = 0;
    let crashedAt = 0;
    let cleared = 0;
    let last = performance.now();
    let frame = 0;

    const leap = () => {
      if (y === 0 && crashedAt === 0) velocity = JUMP;
    };
    jump.current = () => {
      manualUntil = performance.now() + MANUAL_HOLD;
      if (!running) {
        running = true;
        setMood("working");
        last = performance.now();
      }
      leap();
    };

    const paint = () => {
      if (runner.current) runner.current.style.transform = `translateY(${-y}px)`;
      if (ground.current) ground.current.style.transform = `translateX(${-(travelled % 8)}px)`;
      obstacles.forEach((obstacle, index) => {
        const element = blocks.current[index];
        if (!element) return;
        element.style.transform = `translateX(${obstacle.x}px)`;
        element.style.width = `${obstacle.cols * 8}px`;
        element.style.height = `${obstacle.rows * 8}px`;
      });
    };

    const tick = (now: number) => {
      frame = requestAnimationFrame(tick);
      const delta = Math.min(0.05, (now - last) / 1000);
      last = now;
      if (crashedAt > 0) {
        if (now - crashedAt < HANDOFF) return;
        crashedAt = 0;
        obstacles = field();
        speed = SPEED.start;
        cleared = 0;
        y = 0;
        velocity = 0;
        setScore(0);
        setAgent(value => (value + 1) % RUNNERS.length);
        setMood("working");
      }
      if (!running) {
        paint();
        return;
      }
      speed = Math.min(SPEED.max, speed + SPEED.gain * delta);
      travelled += speed * delta;
      velocity -= GRAVITY * delta;
      y = Math.max(0, y + velocity * delta);
      if (y === 0) velocity = 0;
      let farthest = Math.max(...obstacles.map(obstacle => obstacle.x));
      for (const obstacle of obstacles) {
        obstacle.x -= speed * delta;
        const width = obstacle.cols * 8;
        const center = obstacle.x + width / 2 - (RUNNER_X + RUNNER_SIZE / 2);
        if (now > manualUntil && center > 0 && center < speed * 0.3) leap();
        const overlaps =
          obstacle.x < RUNNER_X + RUNNER_SIZE - 7 && obstacle.x + width > RUNNER_X + 7;
        if (overlaps && y < obstacle.rows * 8 - 5) {
          crashedAt = now;
          setMood("failed");
        }
        if (!obstacle.cleared && obstacle.x + width < RUNNER_X) {
          obstacle.cleared = true;
          cleared += 1;
          setScore(cleared);
        }
        if (obstacle.x < -24) {
          Object.assign(obstacle, spawn(Math.max(farthest, TRACK - 190)));
          farthest = obstacle.x;
        }
      }
      paint();
    };

    const onKey = (event: KeyboardEvent) => {
      if (event.code !== "Space" && event.code !== "ArrowUp") return;
      if (event.target instanceof HTMLElement && event.target.closest("button, a, summary")) return;
      event.preventDefault();
      jump.current();
    };
    window.addEventListener("keydown", onKey);
    paint();
    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("keydown", onKey);
    };
  }, [autoplay, still]);

  const onJump = () => jump.current();
  return {
    runner,
    ground,
    blocks,
    onJump,
    still,
    mood,
    score,
    agent: RUNNERS[agent],
    slots: OBSTACLES,
  };
}
