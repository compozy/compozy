/**
 * The brand's point field: `cols × rows` points on the 8 px pitch. With a known fraction it fills
 * column by column in grey and lights only the column being worked on; without one, a band of light
 * keeps crossing it (each point's sweep is delayed by its column).
 */
export class PointField {
  readonly #host: HTMLElement;
  readonly #cols: number;
  readonly #rows: number;
  readonly #points: HTMLSpanElement[] = [];
  #painted = -2;

  constructor(host: HTMLElement, cols: number, rows: number) {
    this.#host = host;
    this.#cols = cols;
    this.#rows = rows;
    host.style.setProperty("--rows", String(rows));
    const sweep = 2400;
    for (let index = 0; index < cols * rows; index += 1) {
      const point = document.createElement("span");
      point.className = "boot__point";
      const column = Math.floor(index / rows);
      point.style.animationDelay = `${Math.round((column * sweep) / cols) - sweep}ms`;
      this.#points.push(point);
    }
    host.replaceChildren(...this.#points);
  }

  /** `fraction` in [0, 1], or null for the unknown-total sweep. */
  paint(fraction: number | null): void {
    const total = this.#cols * this.#rows;
    const done = fraction === null ? -1 : Math.floor(Math.min(1, Math.max(0, fraction)) * total);
    if (done === this.#painted) return;
    this.#painted = done;
    this.#host.dataset.mode = fraction === null ? "sweep" : "fill";
    const liveColumn = Math.floor(done / this.#rows);
    this.#points.forEach((point, index) => {
      const column = Math.floor(index / this.#rows);
      let on: string | undefined;
      if (fraction !== null) {
        if (done >= total) on = "done";
        else if (column === liveColumn && index <= done) on = "live";
        else if (index < done) on = "done";
      }
      if (on) point.dataset.on = on;
      else delete point.dataset.on;
    });
  }
}
