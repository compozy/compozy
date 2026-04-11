import { EOFError, type Message, type Transport } from "../transport.js";

type PendingReader = {
  reject: (error: unknown) => void;
  resolve: (message: Message) => void;
};

/** An in-memory message transport used by SDK tests. */
export class MockTransport implements Transport {
  private readonly incoming: Message[] = [];
  private readonly readers: PendingReader[] = [];
  private peer?: MockTransport;
  private closed = false;
  private peerClosed = false;

  /** Connects this transport to its peer so messages written by one are readable by the other. */
  connect(peer: MockTransport): void {
    this.peer = peer;
  }

  /** Reads one queued message, blocking until a message is available. */
  async readMessage(): Promise<Message> {
    if (this.incoming.length > 0) {
      const message = this.incoming.shift();
      if (message === undefined) {
        throw new EOFError();
      }
      return message;
    }
    if (this.closed || this.peerClosed) {
      throw new EOFError();
    }
    return new Promise<Message>((resolve, reject) => {
      this.readers.push({ resolve, reject });
    });
  }

  /** Enqueues one message for the peer transport. */
  async writeMessage(message: Message): Promise<void> {
    if (this.closed || this.peerClosed || this.peer?.closed) {
      throw new EOFError();
    }
    this.peer?.pushIncoming(message);
  }

  /** Closes this transport endpoint. */
  async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    while (this.readers.length > 0) {
      this.readers.shift()?.reject(new EOFError());
    }
    this.peer?.markPeerClosed();
  }

  /** Reads one queued message. Alias for {@link readMessage}. */
  async receive(): Promise<Message> {
    return this.readMessage();
  }

  /** Enqueues one message for the peer transport. Alias for {@link writeMessage}. */
  async send(message: Message): Promise<void> {
    await this.writeMessage(message);
  }

  private pushIncoming(message: Message): void {
    const reader = this.readers.shift();
    if (reader !== undefined) {
      reader.resolve(message);
      return;
    }
    this.incoming.push(message);
  }

  private markPeerClosed(): void {
    this.peerClosed = true;
    while (this.readers.length > 0) {
      this.readers.shift()?.reject(new EOFError());
    }
  }
}

/** Creates a connected in-memory transport pair. */
export function createMockTransportPair(): [MockTransport, MockTransport] {
  const left = new MockTransport();
  const right = new MockTransport();
  left.connect(right);
  right.connect(left);
  return [left, right];
}
