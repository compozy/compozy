import { ws } from "msw";

import {
  operatorNotificationFixture,
  sessionAttentionChangedFixture,
  sessionCatalogChangedFixture,
} from "./fixtures";

const frames = [
  ["session_catalog_changed", sessionCatalogChangedFixture],
  ["session_attention_changed", sessionAttentionChangedFixture],
  ["operator_notification", operatorNotificationFixture],
].map(([event, data]) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);

const catalogStream = ws.link("*/api/sessions/catalog-stream");

export const sessionCatalogStreamHandler = catalogStream.addEventListener(
  "connection",
  ({ client }) => {
    for (const frame of frames) client.send(frame);
  }
);

export function createSessionCatalogStreamResponse(): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(frames.join("")));
    },
  });
  return new Response(stream, {
    status: 200,
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
  });
}
