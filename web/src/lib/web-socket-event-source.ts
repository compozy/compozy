import type { StreamEventSource } from "./ticketed-event-source";

/** One connection attempt; ticketing and reconnect belong to TicketedEventSource. */
export class WebSocketEventSource extends EventTarget implements StreamEventSource {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;
  readonly url: string;

  private readonly socket: WebSocket;
  private closed = false;
  private failed = false;
  private lastEventId = "";

  constructor(path: string) {
    super();
    const url = new URL(path, window.location.href);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    this.url = url.toString();
    this.socket = new WebSocket(this.url);
    this.socket.onopen = () => {
      const event = new Event("open");
      this.dispatchEvent(event);
      this.onopen?.(event);
    };
    this.socket.onmessage = event => this.receive(event.data);
    this.socket.onerror = () => this.fail();
    this.socket.onclose = () => this.fail();
  }

  close(): void {
    this.closed = true;
    this.socket.onopen = null;
    this.socket.onmessage = null;
    this.socket.onerror = null;
    this.socket.onclose = null;
    this.socket.close();
  }

  private fail(): void {
    if (this.closed || this.failed) return;
    this.failed = true;
    const event = new Event("error");
    this.dispatchEvent(event);
    this.onerror?.(event);
  }

  private receive(frame: unknown): void {
    if (this.closed) return;
    if (typeof frame !== "string") {
      this.fail();
      return;
    }
    // The API writes one complete, already-redacted SSE frame per text message.
    // Preserve named-event semantics: comments never become application data.
    let type = "message";
    const data: string[] = [];
    for (const line of frame.split(/\r?\n/u)) {
      const separator = line.indexOf(":");
      const field = separator < 0 ? line : line.slice(0, separator);
      const value = separator < 0 ? "" : line.slice(separator + 1).replace(/^ /u, "");
      if (field === "event") type = value || "message";
      if (field === "data") data.push(value);
      if (field === "id" && !value.includes("\0")) this.lastEventId = value;
    }
    if (data.length === 0) return;
    const event = new MessageEvent(type, {
      data: data.join("\n"),
      lastEventId: this.lastEventId,
      origin: new URL(this.url).origin,
    });
    this.dispatchEvent(event);
    if (type === "message") this.onmessage?.(event);
  }
}
