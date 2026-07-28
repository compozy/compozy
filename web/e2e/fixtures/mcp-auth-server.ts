import { createServer, type IncomingMessage, type Server } from "node:http";

const DEFAULT_HOST = "127.0.0.1";

/**
 * Minimal auto-approve OAuth 2.1 + PKCE Authorization Server for the MCP
 * authorize E2E journey. It matches the contract the daemon exercises against the
 * task-04 Go `httptest` fake AS (metadata discovery, S256 PKCE, authorization_code
 * grant), but runs in-process as Node so it can be reached across the real-daemon
 * process boundary of a Playwright run. Reuse is behavioral, not literal.
 */
export interface MCPAuthTestServer {
  baseURL: string;
  server: Server;
  /** Requests observed, for optional assertions. */
  readonly requests: string[];
}

export async function startMCPAuthServer(): Promise<MCPAuthTestServer> {
  const requests: string[] = [];
  let baseURL = "";

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? "/", `http://${DEFAULT_HOST}`);
    requests.push(`${request.method} ${url.pathname}`);

    if (request.method === "GET" && url.pathname === "/.well-known/oauth-authorization-server") {
      response.statusCode = 200;
      response.setHeader("content-type", "application/json");
      response.end(
        JSON.stringify({
          issuer: baseURL,
          authorization_endpoint: `${baseURL}/authorize`,
          token_endpoint: `${baseURL}/token`,
          revocation_endpoint: `${baseURL}/revoke`,
          code_challenge_methods_supported: ["S256"],
          response_types_supported: ["code"],
          grant_types_supported: ["authorization_code", "refresh_token"],
        })
      );
      return;
    }

    // Auto-approve: redirect straight back to the daemon callback with a code.
    if (request.method === "GET" && url.pathname === "/authorize") {
      const redirectUri = url.searchParams.get("redirect_uri");
      const state = url.searchParams.get("state") ?? "";
      if (!redirectUri) {
        response.statusCode = 400;
        response.end("missing redirect_uri");
        return;
      }
      const location = new URL(redirectUri);
      location.searchParams.set("code", "auth-code");
      location.searchParams.set("state", state);
      response.statusCode = 302;
      response.setHeader("location", location.toString());
      response.end();
      return;
    }

    if (request.method === "POST" && url.pathname === "/token") {
      drainBody(request, () => {
        response.statusCode = 200;
        response.setHeader("content-type", "application/json");
        response.end(
          JSON.stringify({
            access_token: "e2e-access-token",
            refresh_token: "e2e-refresh-token",
            token_type: "Bearer",
            expires_in: 3600,
            scope: "read write",
          })
        );
      });
      return;
    }

    if (request.method === "POST" && url.pathname === "/revoke") {
      drainBody(request, () => {
        response.statusCode = 200;
        response.end();
      });
      return;
    }

    response.statusCode = 404;
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({ error: "not_found" }));
  });

  await listen(server);
  const address = server.address();
  if (address === null || typeof address === "string") {
    await closeMCPAuthServer(server);
    throw new Error("failed to resolve MCP auth server address");
  }
  baseURL = `http://${DEFAULT_HOST}:${address.port}`;
  return { baseURL, server, requests };
}

function drainBody(request: IncomingMessage, done: () => void) {
  request.on("data", () => undefined);
  request.on("end", done);
}

async function listen(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    const cleanup = () => {
      server.off("error", handleError);
      server.off("listening", handleListening);
    };
    const handleError = (error: Error) => {
      cleanup();
      reject(error);
    };
    const handleListening = () => {
      cleanup();
      resolve();
    };
    server.once("error", handleError);
    server.once("listening", handleListening);
    server.listen(0, DEFAULT_HOST);
  });
}

export async function closeMCPAuthServer(server: Server | undefined): Promise<void> {
  if (server === undefined || !server.listening) return;
  await new Promise<void>((resolve, reject) => {
    server.close(error => (error ? reject(error) : resolve()));
  });
}
