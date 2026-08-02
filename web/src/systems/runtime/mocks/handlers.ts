import type { HttpHandler } from "msw";
import { HttpResponse } from "msw";

import { compozyApiMock, type CompozyApiOkJsonResponseFor } from "@/storybook/openapi-msw";

const encoder = new TextEncoder();

const runtimeProvidersFixture = {
  providers: [
    {
      auth_status: {
        env_policy: "inherit",
        home_policy: "operator",
        last_probe_at: "2026-04-17T18:00:00Z",
        login: {
          configured: true,
          executable: "claude",
          presence: "present",
          source: "auth_login_command",
        },
        message: "Native CLI session is available.",
        mode: "native_cli",
        state: "authenticated",
        status_command: "claude status",
      },
      default: true,
      display_name: "Claude Code",
      name: "claude",
    },
    {
      auth_status: {
        env_policy: "inherit",
        home_policy: "operator",
        last_probe_at: "2026-04-17T18:00:00Z",
        login: {
          configured: true,
          executable: "codex",
          presence: "present",
          source: "auth_login_command",
        },
        message: "Native CLI session is available.",
        mode: "native_cli",
        state: "authenticated",
        status_command: "codex status",
      },
      default: false,
      display_name: "Codex",
      name: "codex",
    },
    {
      auth_status: {
        env_policy: "inherit",
        home_policy: "operator",
        last_probe_at: null,
        login: {
          configured: false,
          presence: "unknown",
          recommended_action: "inspect",
        },
        message: "Provider is installed but has not been probed yet.",
        mode: "native_cli",
        state: "unknown",
      },
      default: false,
      display_name: "Gemini CLI",
      name: "gemini",
    },
  ],
} satisfies CompozyApiOkJsonResponseFor<"get", "/api/providers">;

const directoryHome = "/Users/pedronauck";
const directoryEntriesByPath = new Map<
  string,
  CompozyApiOkJsonResponseFor<"get", "/api/fs/browse">
>([
  [
    directoryHome,
    {
      entries: [
        { is_dir: true, name: "Dev", path: "/Users/pedronauck/Dev" },
        { is_dir: true, name: "Documents", path: "/Users/pedronauck/Documents" },
        { is_dir: true, name: "Downloads", path: "/Users/pedronauck/Downloads" },
      ],
      home: directoryHome,
      parent: "/Users",
      path: directoryHome,
    },
  ],
  [
    "/Users/pedronauck/Dev",
    {
      entries: [
        { is_dir: true, name: "compozy", path: "/Users/pedronauck/Dev/compozy" },
        { is_dir: true, name: "dash", path: "/Users/pedronauck/Dev/dash" },
      ],
      home: directoryHome,
      parent: directoryHome,
      path: "/Users/pedronauck/Dev",
    },
  ],
  [
    "/Users/pedronauck/Dev/compozy",
    {
      entries: [{ is_dir: true, name: "compozy", path: "/Users/pedronauck/Dev/compozy/compozy" }],
      home: directoryHome,
      parent: "/Users/pedronauck/Dev",
      path: "/Users/pedronauck/Dev/compozy",
    },
  ],
]);

function parentPath(path: string): string | undefined {
  const trimmed = path.replace(/\/+$/, "");
  const parent = trimmed.slice(0, Math.max(0, trimmed.lastIndexOf("/")));

  return parent.length > 0 ? parent : undefined;
}

function directoryBrowseFixtureFor(
  request: Request
): CompozyApiOkJsonResponseFor<"get", "/api/fs/browse"> {
  const url = new URL(request.url);
  const requestedPath = url.searchParams.get("path")?.trim() || directoryHome;
  const fixture = directoryEntriesByPath.get(requestedPath);

  if (fixture) {
    return fixture;
  }

  return {
    entries: [],
    home: directoryHome,
    parent: parentPath(requestedPath),
    path: requestedPath,
  };
}

function createRuntimeLogStreamResponse(): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(": storybook runtime log stream\n\n"));
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

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/providers", () => HttpResponse.json(runtimeProvidersFixture)),
  compozyApiMock.get("/api/fs/browse", ({ request }) =>
    HttpResponse.json(directoryBrowseFixtureFor(request))
  ),
  compozyApiMock.get("/api/logs/stream", ({ response }) =>
    response.untyped(createRuntimeLogStreamResponse())
  ),
];
