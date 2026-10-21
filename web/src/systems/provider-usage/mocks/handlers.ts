import { HttpResponse, http, type HttpHandler } from "msw";

import type { ProviderUsageEnvelope } from "../types";

import { defaultUsageEnvelope } from "./fixtures";

// Plain `http` handlers: the stream is a raw SSE response and story overrides
// swap one envelope across every read, which the typed mock cannot express.

const encoder = new TextEncoder();

/** An SSE response that sends the connect-time `ready` frame and stays open. */
export function providerUsageStreamResponse(revision = 14, bootId = "b_storybook"): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        encoder.encode(`event: ready\ndata: ${JSON.stringify({ revision, boot_id: bootId })}\n\n`)
      );
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

/** Handlers serving one envelope for every read and refresh (story overrides swap it). */
export function providerUsageHandlers(
  envelope: () => ProviderUsageEnvelope = () => defaultUsageEnvelope(Date.now())
): HttpHandler[] {
  return [
    http.get("*/api/provider-usage/stream", () => providerUsageStreamResponse()),
    http.get("*/api/provider-usage/providers/:providerId", ({ params }) => {
      const current = envelope();
      const target = current.targets.find(entry => entry.provider_id === params.providerId);
      if (!target) {
        return HttpResponse.json(
          {
            error: `provider "${String(params.providerId)}" not found`,
            code: "provider_not_found",
          },
          { status: 404 }
        );
      }
      return HttpResponse.json({ ...current, tray: null, targets: [target] });
    }),
    http.post("*/api/provider-usage/refresh", () => HttpResponse.json(envelope())),
    http.get("*/api/provider-usage", () => HttpResponse.json(envelope())),
  ];
}

export const handlers: HttpHandler[] = providerUsageHandlers();
