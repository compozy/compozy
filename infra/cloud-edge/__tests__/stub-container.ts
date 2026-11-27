// SPDX-License-Identifier: FSL-1.1-MIT

import { DurableObject } from "cloudflare:workers";
import * as edge from "../src/worker";

export { default } from "../src/worker";

export class StubContainer extends DurableObject {
  #ready = true;
  #status = 200;
  #probes = 0;
  #requests = 0;

  reset(ready: boolean, status: number): void {
    this.#ready = ready;
    this.#status = status;
    this.#probes = 0;
    this.#requests = 0;
  }

  counts() {
    return { probes: this.#probes, requests: this.#requests };
  }

  edgeExports() {
    return { module: Object.keys(edge).sort(), handler: Object.keys(edge.default).sort() };
  }

  override async fetch(request: Request): Promise<Response> {
    if (new URL(request.url).pathname === "/healthz") {
      this.#probes += 1;
      return new Response(null, { status: this.#ready ? 200 : 503 });
    }

    this.#requests += 1;
    const instance = this.ctx.id.name;

    if (request.headers.get("upgrade")?.toLowerCase() === "websocket") {
      const { 0: client, 1: server } = new WebSocketPair();
      server.accept();
      server.addEventListener("message", () => server.send(instance ?? ""));
      return new Response(null, { status: 101, webSocket: client });
    }

    return Response.json(
      {
        instance,
        method: request.method,
        body: await request.text(),
        headers: Object.fromEntries(request.headers),
      },
      { status: this.#status }
    );
  }
}
