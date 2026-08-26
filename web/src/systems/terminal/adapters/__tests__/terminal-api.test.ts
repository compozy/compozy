import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DEV_SERVER_TERMINAL } from "../../mocks/terminal-fixtures";
import {
  TerminalApiError,
  answerTerminalInputRequest,
  closeTerminal,
  controlTerminalRecording,
  createTerminal,
  fetchTerminal,
  fetchTerminalInputRequests,
  fetchTerminalJournal,
  fetchTerminalRecording,
  fetchTerminals,
  mintTerminalAttachTicket,
  readTerminal,
  rejectTerminalInputRequest,
  signalTerminal,
  terminalScopeQuery,
  terminalStreamPath,
} from "../terminal-api";

const WORKSPACE_ID = "ws/atlas";
const TERMINAL_ID = "term/one";
const PROFILE = { profile: "work profile" } as const;

function respond(body: unknown, status = 200, contentType = "application/json") {
  vi.mocked(fetch).mockResolvedValueOnce(
    new Response(contentType === "application/json" ? JSON.stringify(body) : String(body), {
      status,
      headers: { "Content-Type": contentType },
    })
  );
}

function lastRequest() {
  const call = vi.mocked(fetch).mock.calls.at(-1);
  if (!call) throw new Error("expected a fetch call");
  return { url: String(call[0]), init: call[1] };
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("terminalScopeQuery", () => {
  it("Should encode exactly one profile selector", () => {
    expect(terminalScopeQuery(PROFILE).toString()).toBe("profile=work+profile");
    expect(terminalScopeQuery({ all_profiles: true }).toString()).toBe("all_profiles=true");
  });
});

describe("terminal REST requests", () => {
  it("Should encode catalog and detail reads with their owning scope", async () => {
    respond({ terminals: [DEV_SERVER_TERMINAL] });
    await expect(fetchTerminals(WORKSPACE_ID, { all_profiles: true })).resolves.toEqual([
      DEV_SERVER_TERMINAL,
    ]);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/api/workspaces/ws%2Fatlas/terminals?all_profiles=true"),
      init: expect.objectContaining({ method: "GET" }),
    });

    respond({ terminal: DEV_SERVER_TERMINAL });
    await expect(fetchTerminal(WORKSPACE_ID, TERMINAL_ID, PROFILE)).resolves.toEqual(
      DEV_SERVER_TERMINAL
    );
    expect(lastRequest().url).toContain("/term%2Fone?profile=work+profile");
  });

  it("Should preserve method and body for terminal mutations", async () => {
    respond({ terminal: DEV_SERVER_TERMINAL });
    await createTerminal(WORKSPACE_ID, { title: "release", cwd: "/repo" }, PROFILE);
    expect(lastRequest()).toMatchObject({
      init: expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ title: "release", cwd: "/repo" }),
      }),
    });

    respond({ exit: { cause: "signaled", code: null, signal: "TERM", at: "2026-08-25" } });
    await closeTerminal(WORKSPACE_ID, TERMINAL_ID, PROFILE, "TERM");
    expect(lastRequest().init).toEqual(
      expect.objectContaining({ method: "DELETE", body: JSON.stringify({ signal: "TERM" }) })
    );

    respond({ delivered: true });
    await signalTerminal(WORKSPACE_ID, TERMINAL_ID, "INT", PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/term%2Fone/signal?profile=work+profile"),
      init: expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ signal: "INT" }),
      }),
    });
  });

  it("Should encode attach and bounded-read parameters", async () => {
    respond({ ticket: "tkt-1", expires_at: "2026-08-25T12:00:30Z" });
    await mintTerminalAttachTicket(WORKSPACE_ID, TERMINAL_ID, "write", PROFILE, {
      id: "client:web",
      attachmentToken: "attachment-token",
    });
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/attach-ticket?profile=work+profile"),
      init: expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ mode: "write", client_id: "client:web" }),
        headers: expect.objectContaining({ "X-Compozy-Client-Token": "attachment-token" }),
      }),
    });

    respond({ content: "ok", seq: 12, truncated: false, busy: false, untrusted: true });
    await readTerminal(
      WORKSPACE_ID,
      TERMINAL_ID,
      { view: "lines", maxBytes: 512, sinceSeq: 7, from: 2, to: 4, grep: "ok" },
      PROFILE
    );
    expect(lastRequest().url).toContain(
      "/read?profile=work+profile&view=lines&max_bytes=512&since_seq=7&from=2&to=4&grep=ok"
    );
  });

  it("Should keep input-request identity and action bodies in the path", async () => {
    respond({ requests: [] });
    await fetchTerminalInputRequests(WORKSPACE_ID, { all_profiles: true }, TERMINAL_ID);
    expect(lastRequest().url).toContain("/input-requests?all_profiles=true&terminal_id=term%2Fone");

    respond({ outcome: "answered", resolved_at: "2026-08-25T12:00:00Z" });
    await answerTerminalInputRequest(WORKSPACE_ID, TERMINAL_ID, "request/one", "yes", PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/request%2Fone/answer?profile=work+profile"),
      init: expect.objectContaining({ body: JSON.stringify({ input: "yes" }) }),
    });

    respond({ outcome: "rejected", resolved_at: "2026-08-25T12:00:01Z" });
    await rejectTerminalInputRequest(WORKSPACE_ID, TERMINAL_ID, "request/one", "not now", PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/request%2Fone/reject?profile=work+profile"),
      init: expect.objectContaining({ body: JSON.stringify({ reason: "not now" }) }),
    });
  });

  it("Should encode recording control and journal filters", async () => {
    respond({ recording: { id: "rec-1", state: "recording" } });
    await controlTerminalRecording(WORKSPACE_ID, TERMINAL_ID, "start", PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/recording?profile=work+profile"),
      init: expect.objectContaining({ body: JSON.stringify({ action: "start" }) }),
    });

    respond({ entries: [], next: "cursor-2" });
    await expect(
      fetchTerminalJournal(
        WORKSPACE_ID,
        { actor: "agent", since: "2026-08-25", failed: true, terminalId: TERMINAL_ID, limit: 25 },
        { all_profiles: true },
        "cursor-1"
      )
    ).resolves.toEqual({ entries: [], next: "cursor-2" });
    expect(lastRequest().url).toContain(
      "/journal?all_profiles=true&actor=agent&since=2026-08-25&failed=true&terminal_id=term%2Fone&limit=25&cursor=cursor-1"
    );
  });

  it("Should return recording artifacts as text", async () => {
    respond('{"version":2}\n[0,"o","ok"]', 200, "text/plain");

    await expect(fetchTerminalRecording(WORKSPACE_ID, "rec/one", PROFILE)).resolves.toBe(
      '{"version":2}\n[0,"o","ok"]'
    );
    expect(lastRequest().url).toContain("/recordings/rec%2Fone?profile=work+profile");
  });
});

describe("terminal REST failures", () => {
  it("Should preserve the daemon's machine code and message", async () => {
    respond(
      { error: { code: "terminal_limit_reached", message: "Project terminal limit reached" } },
      409
    );

    const error = await fetchTerminals(WORKSPACE_ID, PROFILE).catch(cause => cause);

    expect(error).toBeInstanceOf(TerminalApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "terminal_limit_reached",
      message: "Project terminal limit reached",
    });
  });

  it("Should reject unreadable JSON instead of inventing a payload", async () => {
    respond("not-json", 200, "application/octet-stream");

    await expect(fetchTerminals(WORKSPACE_ID, PROFILE)).rejects.toMatchObject({
      name: "TerminalApiError",
      message: "The daemon returned an unreadable response.",
    });
  });
});

describe("terminalStreamPath", () => {
  it("Should encode every upgrade parameter without adding a profile selector", () => {
    expect(
      terminalStreamPath(WORKSPACE_ID, TERMINAL_ID, {
        ticket: "ticket/one",
        mode: "write",
        flow: "ack",
        cols: 120,
        rows: 40,
        afterSeq: 512,
      })
    ).toBe(
      "/api/workspaces/ws%2Fatlas/terminals/term%2Fone/stream?ticket=ticket%2Fone&mode=write&flow=ack&cols=120&rows=40&after_seq=512"
    );
  });
});
