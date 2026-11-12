/** The always-on agents that take turns running the boot window's point line. */
export const RUNNER_AGENTS = [
  "checkout-lead",
  "qa-runner",
  "docs-steward",
  "release-manager",
  "payments-codex",
  "test-writer",
  "platform-engineer",
] as const;

/** Product agent states the runner wears: Working while it runs, Failed on a miss, Paused at rest. */
export type RunnerMood = "working" | "failed" | "paused";

export const RUNNER = {
  track: 300,
  x: 28,
  size: 32,
  obstacles: 3,
  jump: 330,
  gravity: 1100,
  speed: { start: 150, max: 230, gain: 2 },
  /** How long a key press keeps the autopilot off. */
  manualHoldMs: 6000,
  /** How long the Failed face shows before the next agent takes the run. */
  handoffMs: 1100,
} as const;

export interface Obstacle {
  x: number;
  /** Size in points on the 8 px pitch. */
  cols: number;
  rows: number;
  cleared: boolean;
}

export type RandomSource = () => number;

function spawn(after: number, random: RandomSource): Obstacle {
  return {
    x: after + 190 + random() * 150,
    cols: random() < 0.3 ? 2 : 1,
    rows: random() < 0.5 ? 3 : 2,
    cleared: false,
  };
}

/**
 * The waiting game's rules, free of the DOM: an agent runs at a growing speed and hops point stacks.
 * The autopilot jumps every stack on time, so the game plays itself; a key press hands control to
 * the person for a few seconds, and a miss hands the run to the next agent.
 */
export class RunnerEngine {
  readonly #random: RandomSource;
  obstacles: Obstacle[];
  y = 0;
  velocity = 0;
  speed: number = RUNNER.speed.start;
  travelled = 0;
  score = 0;
  agent = 0;
  mood: RunnerMood;
  running: boolean;
  #manualUntil = 0;
  #crashedAt = 0;

  constructor(options: { autoplay: boolean; random?: RandomSource }) {
    this.#random = options.random ?? Math.random;
    this.running = options.autoplay;
    this.mood = options.autoplay ? "working" : "paused";
    this.obstacles = this.#field();
  }

  #field(): Obstacle[] {
    const obstacles: Obstacle[] = [];
    let after = RUNNER.track - 190;
    for (let index = 0; index < RUNNER.obstacles; index += 1) {
      const next = spawn(after, this.#random);
      obstacles.push(next);
      after = next.x;
    }
    return obstacles;
  }

  /** Space, the up arrow or a click: start a paused run, take over for a while and jump. */
  press(now: number): void {
    this.#manualUntil = now + RUNNER.manualHoldMs;
    if (!this.running) {
      this.running = true;
      this.mood = "working";
    }
    this.#leap();
  }

  #leap(): void {
    if (this.y === 0 && this.#crashedAt === 0) this.velocity = RUNNER.jump;
  }

  /** Advances the run by `seconds` (capped so a background tab never teleports the runner). */
  step(seconds: number, now: number): void {
    const delta = Math.min(0.05, seconds);
    if (this.#crashedAt > 0) {
      if (now - this.#crashedAt < RUNNER.handoffMs) return;
      this.#handoff();
    }
    if (!this.running) return;
    this.speed = Math.min(RUNNER.speed.max, this.speed + RUNNER.speed.gain * delta);
    this.travelled += this.speed * delta;
    this.velocity -= RUNNER.gravity * delta;
    this.y = Math.max(0, this.y + this.velocity * delta);
    if (this.y === 0) this.velocity = 0;
    let farthest = Math.max(...this.obstacles.map(obstacle => obstacle.x));
    for (const obstacle of this.obstacles) {
      obstacle.x -= this.speed * delta;
      const width = obstacle.cols * 8;
      const ahead = obstacle.x + width / 2 - (RUNNER.x + RUNNER.size / 2);
      if (now > this.#manualUntil && ahead > 0 && ahead < this.speed * 0.3) this.#leap();
      const overlaps = obstacle.x < RUNNER.x + RUNNER.size - 7 && obstacle.x + width > RUNNER.x + 7;
      if (overlaps && this.y < obstacle.rows * 8 - 5 && this.#crashedAt === 0) {
        this.#crashedAt = now;
        this.mood = "failed";
      }
      if (!obstacle.cleared && obstacle.x + width < RUNNER.x) {
        obstacle.cleared = true;
        this.score += 1;
      }
      if (obstacle.x < -24) {
        Object.assign(obstacle, spawn(Math.max(farthest, RUNNER.track - 190), this.#random));
        farthest = obstacle.x;
      }
    }
  }

  #handoff(): void {
    this.#crashedAt = 0;
    this.obstacles = this.#field();
    this.speed = RUNNER.speed.start;
    this.score = 0;
    this.y = 0;
    this.velocity = 0;
    this.agent = (this.agent + 1) % RUNNER_AGENTS.length;
    this.mood = "working";
  }
}
