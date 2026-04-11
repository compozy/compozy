import { EOFError, type Message, type Transport } from "../transport.js";

type PendingReader = {
  reject: (error: unknown) => void;
  resolve: (message: Message) => void;
};

export class MockTransport implements Transport {
  private readonly incoming: Message[] = [];
  private readonly readers: PendingReader[] = [];
  private peer?: MockTransport;
  private closed = false;
  private peerClosed = false;

  connect(peer: MockTransport): void {
    this.peer = peer;
  }

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

  async writeMessage(message: Message): Promise<void> {
    if (this.closed || this.peerClosed || this.peer?.closed) {
      throw new EOFError();
    }
    this.peer?.pushIncoming(message);
  }

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

  async receive(): Promise<Message> {
    return this.readMessage();
  }

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

export function createMockTransportPair(): [MockTransport, MockTransport] {
  const left = new MockTransport();
  const right = new MockTransport();
  left.connect(right);
  right.connect(left);
  return [left, right];
}
