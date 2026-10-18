# @compozy/client

A zero-dependency TypeScript client for applications built on CompozyOS. Supports Node 18+, Bun,
Deno, browsers, and edge runtimes. The daemon remains a separate service.

> Experimental in 0.4: names and shapes may change in 0.5 without a deprecation window. Pin the exact version.

```ts
import { Compozy } from "@compozy/client";

const compozy = await Compozy.connect();
const { workspace } = await compozy.workspaces.resolveWorkspace({ body: { path: process.cwd() } });
const { session } = await compozy.sessions.createSession({
  body: { workspace: workspace.id, agent_name: "claude" },
});
const reply = await compozy.prompt({
  workspaceId: workspace.id,
  sessionId: session.id,
  message: "Summarize this repository.",
});
if (reply.kind === "stream") {
  for await (const chunk of reply.chunks) {
    if (chunk.type === "text-delta") process.stdout.write(chunk.delta);
  }
}
```

Node discovery checks `baseUrl`, `COMPOZY_URL`, the `daemon.json` in `home`/`COMPOZY_HOME`/`~/.compozy`,
then `http://127.0.0.1:2123`. Browsers default to `http://localhost:2123`. Pass `token` for an app token;
Node also reads `COMPOZY_APP_TOKEN`. All operations accept an abort signal.

Use `promptResponse()` in a server route to forward the AI SDK UI message stream unchanged. Read
other SSE operations through `events(operationId, options)`, which reconnects with `Last-Event-ID`
after network or server failures. Client errors stop immediately; 20 consecutive failures stop the
stream. Aborting a prompt stops reading without canceling the daemon's turn.

The Node-only `@compozy/client/node` entry adds `Compozy.start({ home })` and
`close({ stopDaemon: true })`. It only stops the daemon this instance started while the same home
still names the owned process. The main browser entry contains no Node modules.

Generated resource methods cover JSON operations. Exported `paths`, `operations`, `components`, and
`Operation*` helpers also work with other HTTP clients. There is no dependency on React or `ai`.

See [Build an app](https://compozy.com/docs/apps), [chat UI](https://compozy.com/docs/apps/chat-ui), and
[browser access](https://compozy.com/docs/apps/browser) for tutorials and API reference samples.
