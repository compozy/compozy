import { describe, expect, it } from "vitest";

import type { SessionProcessResult } from "../../adapters/session-processes-api";
import { primarySessionFixture } from "../../mocks/fixtures";
import { sessionProcessFixture, withSessionProcessSummary } from "../../mocks/process-fixtures";
import { processTreeResults } from "../session-process-feedback";
import {
  buildProcessTrees,
  processCommandLabel,
  processIdentity,
  processStopReport,
  sessionRunningProcesses,
} from "../session-processes";

// Suite: session process view model.
// Invariant: the count is background + Terminal rows of a supported scan, and a stop reads as
// one report from the daemon's per-target outcomes: exited counts as stopped, every other
// refusal is named with its reason.
// Trees (#1100): the count is the daemon's tree count when it reports one, rows join the daemon's
// trees, and a tree's stop reads as one outcome.
// Owning layer: session lib (pure). Canonical suite: this file.

const result = (pid: number, outcome: SessionProcessResult["outcome"], error?: string) => ({
  pid,
  started_at: "2026-10-29T12:00:00Z",
  outcome,
  ...(error ? { error } : {}),
});

const command = (entry: SessionProcessResult) => `cmd-${entry.pid}`;

describe("sessionRunningProcesses", () => {
  it("Should count background and Terminal processes of a supported scan only", () => {
    const session = withSessionProcessSummary(primarySessionFixture);
    expect(sessionRunningProcesses(session)).toBe(3);
    expect(
      sessionRunningProcesses({
        process_summary: { ...session.process_summary!, supported: false },
      })
    ).toBe(0);
    expect(sessionRunningProcesses({ process_summary: null })).toBe(0);
  });

  it("Should count trees once the daemon reports them", () => {
    const session = withSessionProcessSummary(primarySessionFixture);
    expect(
      sessionRunningProcesses({ process_summary: { ...session.process_summary!, trees: 2 } })
    ).toBe(2);
  });
});

const wrapper = sessionProcessFixture({ pid: 10, command: "bun run dev", ports: [] });
const server = sessionProcessFixture({ pid: 11, command: "node server.js", ports: [3000] });
const helper = sessionProcessFixture({ pid: 12, command: "esbuild --service", ports: [] });
const tool = sessionProcessFixture({ pid: 20, kind: "tool", command: "go test ./..." });
const target = ({ pid, started_at }: { pid: number; started_at: string }) => ({ pid, started_at });
const serverTree = {
  id: processIdentity(wrapper),
  kind: "background" as const,
  root: target(wrapper),
  lead: target(server),
  wrappers: [target(wrapper)],
  members: [target(wrapper), target(server), target(helper)],
  ports: [3000],
};

describe("buildProcessTrees", () => {
  it("Should join rows to the daemon's trees and keep unnamed rows as their own", () => {
    const [left, turn] = buildProcessTrees([tool, wrapper, server, helper], [serverTree]);
    expect(left).toMatchObject({
      id: processIdentity(wrapper),
      lead: server,
      wrappers: [wrapper],
      helpers: [helper],
      ports: [3000],
    });
    // Tool calls sort after what the session left running.
    expect(turn).toMatchObject({ id: processIdentity(tool), kind: "tool", helpers: [] });
  });

  it("Should list one row per process from a daemon that predates trees", () => {
    expect(buildProcessTrees([wrapper, server]).map(tree => tree.lead)).toHaveLength(2);
  });
});

describe("processTreeResults", () => {
  it("Should read a tree's stop as one outcome named by its lead", () => {
    const [tree] = buildProcessTrees([wrapper, server, helper], [serverTree]);
    const at = (pid: number, outcome: SessionProcessResult["outcome"]) => ({
      pid,
      started_at: wrapper.started_at,
      outcome,
    });
    const done = processTreeResults(
      [at(10, "terminated"), at(11, "terminated"), at(12, "already_exited")],
      [tree!]
    );
    expect(done).toEqual([{ result: at(11, "terminated"), tree }]);
    const refused = processTreeResults(
      [at(10, "terminated"), at(11, "unproven"), at(99, "failed")],
      [tree!]
    );
    expect(refused.map(({ result }) => [result.pid, result.outcome])).toEqual([
      [11, "unproven"],
      [99, "failed"],
    ]);
  });
});

describe("processCommandLabel", () => {
  it("Should name the program and keep paths inside the working directory relative", () => {
    expect(
      processCommandLabel({
        command:
          "/Users/dev/.local/share/mise/installs/node/24.22.1/bin/node /Users/dev/lab/server.js --port 50399",
        argv0: "/Users/dev/.local/share/mise/installs/node/24.22.1/bin/node",
        cwd: "/Users/dev/lab",
      })
    ).toBe("node server.js --port 50399");
    expect(processCommandLabel({ command: "/bin/sleep 9999", argv0: "/bin/sleep", cwd: "" })).toBe(
      "sleep 9999"
    );
    expect(processCommandLabel({ command: "", argv0: "vite", cwd: "/tmp" })).toBe("vite");
    // Paths are case-sensitive on Linux and case-sensitive APFS: no relative form across case.
    expect(
      processCommandLabel({ command: "node /work/Foo/server.js", argv0: "node", cwd: "/work/foo" })
    ).toBe("node /work/Foo/server.js");
    expect(
      processCommandLabel({ command: "bash -c 'a\nb' /etc/hosts", argv0: "bash", cwd: "/tmp" })
    ).toBe("bash -c 'a ↵ b' /etc/hosts");
  });
});

describe("processStopReport", () => {
  it("Should report every stop as one success, counting already exited as stopped", () => {
    expect(
      processStopReport([result(1, "terminated"), result(2, "already_exited")], command)
    ).toEqual({ tone: "success", title: "2 processes stopped" });
    expect(processStopReport([result(1, "already_exited")], command)).toEqual({
      tone: "info",
      title: "Process already exited",
    });
  });

  it("Should name each refused process with its reason on a partial stop", () => {
    expect(
      processStopReport(
        [
          result(1, "terminated"),
          result(2, "identity_mismatch"),
          result(3, "failed", "operation not permitted"),
          result(4, "unproven"),
        ],
        command
      )
    ).toEqual({
      tone: "error",
      title: "1 process stopped, 3 not stopped",
      description:
        "cmd-2: It changed since the list was read. Refresh and try again. · cmd-3: operation not permitted · cmd-4: Stop was sent, but its exit couldn't be confirmed.",
    });
    expect(processStopReport([result(5, "failed")], command)).toEqual({
      tone: "error",
      title: "Couldn't stop 1 process",
      description: "cmd-5: Couldn't stop it.",
    });
  });
});
