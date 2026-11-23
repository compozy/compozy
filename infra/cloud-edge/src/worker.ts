import { Container } from "@cloudflare/containers";

export class CloudContainer extends Container {
  override defaultPort = 8080;
  override sleepAfter = "24h";
  override enableInternet = true;
}

interface Env {
  CLOUD: DurableObjectNamespace<CloudContainer>;
}

const POOL_SIZE = 2;
const HEALTH_URL = "http://container/healthz";

function computerId(request: Request): string | undefined {
  const url = new URL(request.url);
  if (
    (request.method === "GET" &&
      url.pathname === "/v1/computers/control" &&
      request.headers.get("upgrade")?.toLowerCase() === "websocket") ||
    (request.method === "POST" && url.pathname === "/v1/browser/token")
  ) {
    return url.searchParams.get("computer_id") ?? undefined;
  }

  const match = /^\/v1\/computers\/(cmp_[^/]+)(?:\/(revalidate|tunnel))?$/.exec(url.pathname);
  if (match && request.method === (match[2] ? "POST" : "PATCH")) {
    return match[1];
  }
}

function firstInstance(id: string | undefined): number {
  if (!id) return Math.floor(Math.random() * POOL_SIZE);

  // FNV-1a over the ASCII computer ID.
  let hash = 0x811c9dc5;
  for (let index = 0; index < id.length; index += 1) {
    hash = Math.imul(hash ^ id.charCodeAt(index), 0x01000193);
  }
  return (hash >>> 0) % POOL_SIZE;
}

function forwardedRequest(request: Request): Request {
  const headers = new Headers(request.headers);
  headers.delete("cf-container-target-port");

  const clientIp = request.headers.get("CF-Connecting-IP");
  const location = [request.cf?.city, request.cf?.region, request.cf?.country]
    .filter((part): part is string => typeof part === "string" && part.length > 0)
    .map(encodeURIComponent)
    .join(", ");
  for (const [name, value] of [
    ["X-Compozy-Client-IP", clientIp],
    ["X-Compozy-Location", location],
  ] as const) {
    if (value) headers.set(name, value);
    else headers.delete(name);
  }

  return new Request(request, { headers, redirect: "manual" });
}

async function isReady(container: DurableObjectStub<CloudContainer>, timeoutMs: number) {
  try {
    const response = await container.fetch(
      new Request(HEALTH_URL, {
        signal: AbortSignal.timeout(timeoutMs),
        redirect: "manual",
      })
    );
    await response.body?.cancel();
    return response.status === 200;
  } catch {
    return false;
  }
}

function unavailable(): Response {
  return Response.json(
    { code: "cloud_unavailable" },
    { status: 503, headers: { "Retry-After": "5", "Cache-Control": "no-store" } }
  );
}

export default {
  async fetch(request, env) {
    const first = firstInstance(computerId(request));
    for (let offset = 0; offset < POOL_SIZE; offset += 1) {
      const index = (first + offset) % POOL_SIZE;
      const container = env.CLOUD.getByName(`cloud-${index}`);
      if (!(await isReady(container, 2_000))) continue;

      // Retry only before dispatch. An interrupted write may already have committed.
      try {
        return await container.fetch(forwardedRequest(request));
      } catch {
        return unavailable();
      }
    }

    return unavailable();
  },

  async scheduled(_controller, env) {
    await Promise.all(
      Array.from({ length: POOL_SIZE }, async (_, index) => {
        const name = `cloud-${index}`;
        if (!(await isReady(env.CLOUD.getByName(name), 10_000))) {
          console.warn("Cloud container failed its scheduled health check", { instance: name });
        }
      })
    );
  },
} satisfies ExportedHandler<Env>;
