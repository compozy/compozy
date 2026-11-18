import { RUNNER, RUNNER_AGENTS, RunnerEngine, type RunnerMood } from "./runner-engine";

/** Build-time faces: `assets/faces/<agent>-<mood>-<theme>.svg`, rendered with the product generator. */
export function faceSource(agent: string, mood: RunnerMood, theme: "dark" | "light"): string {
  return `assets/faces/${agent}-${mood}-${theme}.svg`;
}

function element<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className: string
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  node.className = className;
  return node;
}

/**
 * Mounts the waiting game into `host`: an always-on agent's face runs the point line in the boot
 * window's empty ink. Positions are written through the CSSOM (allowed by the page's CSP, unlike
 * inline style attributes). Reduced motion keeps the game off and the face still.
 */
export class BootRunner {
  readonly #host: HTMLElement;
  readonly #engine: RunnerEngine;
  readonly #still: boolean;
  readonly #face = element("img", "boot__runner-face");
  readonly #agent = element("span", "boot__runner-agent");
  readonly #name = element("span", "boot__runner-name");
  readonly #hint = element("i", "boot__runner-hint");
  readonly #score = element("span", "boot__runner-score");
  readonly #ground = element("span", "boot__runner-ground-dots");
  readonly #blocks: HTMLSpanElement[] = [];
  #frame = 0;
  #last = 0;
  #shown = "";

  constructor(host: HTMLElement, options: { autoplay: boolean }) {
    this.#host = host;
    this.#still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    this.#engine = new RunnerEngine({ autoplay: options.autoplay && !this.#still });
    this.#build();
  }

  #build(): void {
    const hud = element("p", "boot__runner-hud");
    const label = element("span", "");
    label.append(this.#name, this.#hint);
    hud.append(label, this.#score);
    const track = element("button", "boot__runner-track");
    track.type = "button";
    track.setAttribute("aria-label", "Jump");
    track.addEventListener("click", () => this.#engine.press(performance.now()));
    this.#face.alt = "";
    this.#face.width = RUNNER.size;
    this.#face.height = RUNNER.size;
    this.#agent.append(this.#face);
    track.append(this.#agent);
    for (let index = 0; index < RUNNER.obstacles; index += 1) {
      const block = element("span", "boot__runner-block");
      this.#blocks.push(block);
      track.append(block);
    }
    const ground = element("span", "boot__runner-ground");
    ground.append(this.#ground);
    track.append(ground);
    this.#host.replaceChildren(hud, track);
    this.#host.dataset.still = String(this.#still);
  }

  #onKey = (event: KeyboardEvent): void => {
    if (event.code !== "Space" && event.code !== "ArrowUp") return;
    if (event.target instanceof HTMLElement && event.target.closest("button, a, summary")) return;
    event.preventDefault();
    this.#engine.press(performance.now());
  };

  start(): void {
    window.addEventListener("keydown", this.#onKey);
    this.#paint();
    if (this.#still) return;
    this.#last = performance.now();
    const tick = (now: number) => {
      this.#frame = requestAnimationFrame(tick);
      this.#engine.step((now - this.#last) / 1000, now);
      this.#last = now;
      this.#paint();
    };
    this.#frame = requestAnimationFrame(tick);
  }

  stop(): void {
    cancelAnimationFrame(this.#frame);
    window.removeEventListener("keydown", this.#onKey);
  }

  #paint(): void {
    const engine = this.#engine;
    const agent = RUNNER_AGENTS[engine.agent] ?? RUNNER_AGENTS[0];
    const mood: RunnerMood = this.#still ? "paused" : engine.mood;
    const theme = window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    const face = faceSource(agent, mood, theme);
    if (face !== this.#shown) {
      this.#face.src = face;
      this.#shown = face;
    }
    this.#host.dataset.mood = mood;
    this.#name.textContent = agent;
    this.#hint.textContent = this.#still
      ? ""
      : engine.running
        ? "Space to jump"
        : "Press Space to play";
    this.#score.textContent = String(engine.score).padStart(3, "0");
    this.#agent.style.transform = `translateY(${-engine.y}px)`;
    this.#ground.style.transform = `translateX(${-(engine.travelled % 8)}px)`;
    engine.obstacles.forEach((obstacle, index) => {
      const block = this.#blocks[index];
      if (!block) return;
      block.style.transform = `translateX(${obstacle.x}px)`;
      block.style.width = `${obstacle.cols * 8}px`;
      block.style.height = `${obstacle.rows * 8}px`;
    });
  }
}
