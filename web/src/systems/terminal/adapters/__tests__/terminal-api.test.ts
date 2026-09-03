import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  parseTerminalCatalogEvent,
  TerminalCatalogProtocolError,
} from "../../lib/terminal-catalog-stream";
import { DEV_SERVER_TERMINAL, JOURNAL_FIXTURES } from "../../mocks/terminal-fixtures";
import {
  TerminalApiError,
  TerminalProtocolError,
  answerTerminalInputRequest,
  closeTerminal,
  controlTerminalRecording,
  createTerminal,
  fetchTerminal,
  fetchTerminalInputRequestProjection,
  fetchTerminalJournal,
  fetchTerminalRecording,
  fetchTerminals,
  mintTerminalAttachTicket,
  readTerminal,
  rejectTerminalInputRequest,
  signalTerminal,
  terminalScopeQuery,
  terminalStreamPath,
  waitTerminal,
} from "../terminal-api";

const WORKSPACE_ID = "ws/atlas";
const TERMINAL_ID = "term/one";
const PROFILE = { profile: "work profile" } as const;
const VIEWER = { id: "client:web", attachmentToken: "attachment-token" } as const;

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
    await createTerminal(WORKSPACE_ID, { title: "release", cwd: "/repo" }, PROFILE, VIEWER);
    expect(lastRequest()).toMatchObject({
      init: expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ title: "release", cwd: "/repo", client_id: "client:web" }),
        headers: expect.objectContaining({ "X-Compozy-Client-Token": "attachment-token" }),
      }),
    });

    respond({
      exit: { cause: "signaled", code: null, signal: "TERM", at: "2026-08-25T12:00:00Z" },
    });
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

    respond({ reason: "exit", screen: "", untrusted: true, exit_code: 0 });
    await waitTerminal(WORKSPACE_ID, TERMINAL_ID, { until: "exit" }, PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/term%2Fone/wait?profile=work+profile"),
      init: expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ until: "exit" }),
      }),
    });
  });

  it("Should encode attach and bounded-read parameters", async () => {
    respond({ ticket: "tkt-1", expires_at: "2026-08-25T12:00:30Z" });
    await mintTerminalAttachTicket(WORKSPACE_ID, TERMINAL_ID, "write", PROFILE, VIEWER);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/attach-ticket?profile=work+profile"),
      init: expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ mode: "write", client_id: "client:web" }),
        headers: expect.objectContaining({ "X-Compozy-Client-Token": "attachment-token" }),
      }),
    });

    respond({ content: "ok", seq: "12", truncated: false, busy: false, untrusted: true });
    await expect(
      readTerminal(
        WORKSPACE_ID,
        TERMINAL_ID,
        { view: "lines", maxBytes: 512, sinceSeq: 7n, from: 2, to: 4, grep: "ok" },
        PROFILE
      )
    ).resolves.toMatchObject({ seq: 12n });
    expect(lastRequest().url).toContain(
      "/read?profile=work+profile&view=lines&max_bytes=512&since_seq=7&from=2&to=4&grep=ok"
    );
  });

  it("Should keep input-request identity and action bodies in the path", async () => {
    respond({ pending: [], resolved: [] });
    await fetchTerminalInputRequestProjection(WORKSPACE_ID, { all_profiles: true }, TERMINAL_ID);
    expect(lastRequest().url).toContain("/input-requests?all_profiles=true&terminal_id=term%2Fone");

    respond({ delivered_bytes: 4, redacted: false });
    await answerTerminalInputRequest(WORKSPACE_ID, TERMINAL_ID, "request/one", "yes", PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/request%2Fone/answer?profile=work+profile"),
      init: expect.objectContaining({ body: JSON.stringify({ input: "yes" }) }),
    });

    respond({ outcome: "rejected" });
    await rejectTerminalInputRequest(WORKSPACE_ID, TERMINAL_ID, "request/one", "not now", PROFILE);
    expect(lastRequest()).toMatchObject({
      url: expect.stringContaining("/request%2Fone/reject?profile=work+profile"),
      init: expect.objectContaining({ body: JSON.stringify({ reason: "not now" }) }),
    });
  });

  it("Should encode recording control and journal filters", async () => {
    respond({
      recording: {
        id: "rec-1",
        state: "recording",
        terminal_id: TERMINAL_ID,
        profile_id: "profile-work",
        digest: "",
        started_at: "2026-08-25T12:00:00Z",
        stopped_at: null,
        bytes: 0,
        expires_at: "2026-08-26T12:00:00Z",
      },
    });
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
      {
        error: {
          code: "terminal_limit_reached",
          message: "Project terminal limit reached",
          details: { current: 8, max: 8 },
        },
      },
      409
    );

    const error = await fetchTerminals(WORKSPACE_ID, PROFILE).catch(cause => cause);

    expect(error).toBeInstanceOf(TerminalApiError);
    if (!(error instanceof TerminalApiError)) throw error;
    expect(error).toMatchObject({
      status: 409,
      code: "terminal_limit_reached",
      domainCode: "terminal_limit_reached",
      message: "Project terminal limit reached",
      details: { current: 8, max: 8 },
    });
    expect(Object.isFrozen(error.details)).toBe(true);
  });

  it("Should preserve the daemon envelope when a recording download fails", async () => {
    respond({ error: { code: "recording_unavailable", message: "Recording is unavailable" } }, 422);

    await expect(fetchTerminalRecording(WORKSPACE_ID, "rec-1", PROFILE)).rejects.toMatchObject({
      status: 422,
      code: "recording_unavailable",
      message: "Recording is unavailable",
    });
  });

  it("Should preserve a truthful non-domain transport code", async () => {
    respond(
      { error: { code: "service_unavailable", message: "terminal service unavailable" } },
      503
    );

    await expect(fetchTerminals(WORKSPACE_ID, PROFILE)).rejects.toMatchObject({
      status: 503,
      code: "service_unavailable",
      domainCode: undefined,
      message: "terminal service unavailable",
    });
  });

  it("Should reject malformed terminal error details without exposing them", async () => {
    const secret = "must-not-appear";
    respond(
      {
        error: {
          code: "terminal_limit_reached",
          message: "Project terminal limit reached",
          details: { current: { secret } },
        },
      },
      409
    );

    const error = await fetchTerminals(WORKSPACE_ID, PROFILE).catch(cause => cause);

    expect(error).toBeInstanceOf(TerminalProtocolError);
    if (!(error instanceof TerminalProtocolError)) throw error;
    expect(error.message).not.toContain(secret);

    respond(
      {
        error: {
          code: "terminal_limit_reached",
          message: " \n\t ",
        },
      },
      409
    );

    await expect(fetchTerminals(WORKSPACE_ID, PROFILE)).rejects.toBeInstanceOf(
      TerminalProtocolError
    );
  });

  it("Should reject unreadable JSON instead of inventing a payload", async () => {
    respond("not-json", 200, "application/octet-stream");

    await expect(fetchTerminals(WORKSPACE_ID, PROFILE)).rejects.toMatchObject({
      name: "TerminalProtocolError",
      message: "The daemon returned an invalid terminal response.",
    });
  });

  it("Should reject malformed terminal success envelopes instead of casting them", async () => {
    respond({ ticket: 42, expires_at: "never" });
    await expect(
      mintTerminalAttachTicket(WORKSPACE_ID, TERMINAL_ID, "read", PROFILE)
    ).rejects.toBeInstanceOf(TerminalProtocolError);

    respond({ pending: [{ id: "request-without-contract-fields" }], resolved: [] });
    await expect(fetchTerminalInputRequestProjection(WORKSPACE_ID, PROFILE)).rejects.toBeInstanceOf(
      TerminalProtocolError
    );

    respond({ entries: [{ command_id: "incomplete" }], next: null });
    await expect(fetchTerminalJournal(WORKSPACE_ID, {}, PROFILE)).rejects.toBeInstanceOf(
      TerminalProtocolError
    );

    respond({ content: "missing read metadata" });
    await expect(
      readTerminal(WORKSPACE_ID, TERMINAL_ID, { view: "screen" }, PROFILE)
    ).rejects.toBeInstanceOf(TerminalProtocolError);

    respond({ recording: { id: "incomplete" } });
    await expect(
      controlTerminalRecording(WORKSPACE_ID, TERMINAL_ID, "start", PROFILE)
    ).rejects.toBeInstanceOf(TerminalProtocolError);
  });

  it("Should keep answer input out of protocol diagnostics", async () => {
    const secret = "password-that-must-not-appear";
    respond({ delivered_bytes: "not-a-number", redacted: false });

    const error = await answerTerminalInputRequest(
      WORKSPACE_ID,
      TERMINAL_ID,
      "request-1",
      secret,
      PROFILE
    ).catch(cause => cause);

    expect(error).toBeInstanceOf(TerminalProtocolError);
    expect(error.message).toBe("The daemon returned an invalid terminal response.");
    expect(error.message).not.toContain(secret);
  });

  it("Should reject fractional and negative terminal counters", async () => {
    const validRead = {
      content: "ok",
      seq: "12",
      truncated: false,
      busy: false,
      untrusted: true,
      spill: { artifact_id: "artifact-1", bytes: 24 },
    };
    const validRecording = {
      id: "rec-1",
      state: "recording",
      terminal_id: TERMINAL_ID,
      profile_id: "profile-work",
      digest: "",
      started_at: "2026-08-25T12:00:00Z",
      stopped_at: null,
      bytes: 24,
      expires_at: "2026-08-26T12:00:00Z",
    };

    for (const invalid of [-1, 1.5]) {
      const cases = [
        {
          body: { terminals: [{ ...DEV_SERVER_TERMINAL, viewers: invalid }] },
          request: () => fetchTerminals(WORKSPACE_ID, PROFILE),
        },
        {
          body: {
            terminals: [
              {
                ...DEV_SERVER_TERMINAL,
                bound_run: { session_id: "session-1", run_id: "run-1", generation: invalid },
              },
            ],
          },
          request: () => fetchTerminals(WORKSPACE_ID, PROFILE),
        },
        {
          body: { ...validRead, seq: invalid },
          request: () => readTerminal(WORKSPACE_ID, TERMINAL_ID, { view: "screen" }, PROFILE),
        },
        {
          body: { ...validRead, spill: { artifact_id: "artifact-1", bytes: invalid } },
          request: () => readTerminal(WORKSPACE_ID, TERMINAL_ID, { view: "screen" }, PROFILE),
        },
        {
          body: { delivered_bytes: invalid, redacted: false },
          request: () =>
            answerTerminalInputRequest(WORKSPACE_ID, TERMINAL_ID, "request-1", "answer", PROFILE),
        },
        {
          body: { recording: { ...validRecording, bytes: invalid } },
          request: () => controlTerminalRecording(WORKSPACE_ID, TERMINAL_ID, "start", PROFILE),
        },
        {
          body: {
            entries: [{ ...JOURNAL_FIXTURES[0], duration_ms: invalid }],
            next: null,
          },
          request: () => fetchTerminalJournal(WORKSPACE_ID, {}, PROFILE),
        },
        {
          body: {
            entries: [{ ...JOURNAL_FIXTURES[0], output_bytes: invalid }],
            next: null,
          },
          request: () => fetchTerminalJournal(WORKSPACE_ID, {}, PROFILE),
        },
      ];

      for (const testCase of cases) {
        respond(testCase.body);
        await expect(testCase.request()).rejects.toBeInstanceOf(TerminalProtocolError);
      }
    }
  });
});

describe("terminal REST and SSE contract parity", () => {
  it("Should accept the same exact terminal projection", async () => {
    respond({ terminals: [DEV_SERVER_TERMINAL] });

    const rest = await fetchTerminals(WORKSPACE_ID, PROFILE);
    const sse = parseTerminalCatalogEvent("terminal.snapshot", {
      terminals: [DEV_SERVER_TERMINAL],
    });

    expect(sse).toEqual({ name: "terminal.snapshot", terminals: rest });
  });

  it("Should reject open enum values on both boundaries", async () => {
    const invalidTerminals = [
      { ...DEV_SERVER_TERMINAL, mode: "future" },
      { ...DEV_SERVER_TERMINAL, state: "paused" },
      { ...DEV_SERVER_TERMINAL, lease: "shared" },
      { ...DEV_SERVER_TERMINAL, controller: { kind: "operator", id: "client-1" } },
      {
        ...DEV_SERVER_TERMINAL,
        state: "exited",
        exit: {
          cause: "cancelled",
          code: null,
          signal: null,
          at: "2026-08-25T12:00:00Z",
        },
      },
    ];

    for (const terminal of invalidTerminals) {
      respond({ terminals: [terminal] });
      await expect(fetchTerminals(WORKSPACE_ID, PROFILE)).rejects.toBeInstanceOf(
        TerminalProtocolError
      );
      expect(() =>
        parseTerminalCatalogEvent("terminal.snapshot", { terminals: [terminal] })
      ).toThrow(TerminalCatalogProtocolError);
    }
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
        afterSeq: 512n,
      })
    ).toBe(
      "/api/workspaces/ws%2Fatlas/terminals/term%2Fone/stream?ticket=ticket%2Fone&mode=write&flow=ack&cols=120&rows=40&after_seq=512"
    );
  });
});
