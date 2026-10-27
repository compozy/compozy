import { mkdir, writeFile, appendFile } from "node:fs/promises";
import { join, resolve } from "node:path";

import { expect } from "@playwright/test";

import { runCommand } from "./process";

interface Session {
  id: string;
  workspace_id: string;
  state: string;
  badge: string;
  runtime: { acp_session_id?: string };
  stop_reason?: string;
  failure?: unknown;
}
interface SessionEvent {
  type: string;
  content: { synthetic?: { kind?: string; subagent_ids?: string[] } };
}
interface Subagent {
  subagent_id: string;
  child_session_id: string;
  status: string;
  delivery: string;
  wake_message_id?: string;
}
export interface UpdateWork {
  origin: string;
  sessions: Session[];
  stopped: Session;
  child: Subagent;
}

export async function prepareUpdateWork(home: string): Promise<void> {
  const driver = join(home, "acpmock-driver");
  const built = await runCommand(
    "go",
    ["build", "-o", driver, "./internal/testutil/acpmock/cmd/acpmock-driver"],
    { cwd: resolve("..") }
  );
  expect(built.exitCode, built.stderr).toBe(0);
  const fixture = join(home, "update-work.json");
  const block = { kind: "driver_control", driver_control: { action: "block_until_cancel" } };
  const agents = ["update-parent", "update-worker"].map(name => ({
    name,
    provider: "acpmock",
    permissions: "approve-all",
    turns: [
      {
        name: "in-flight",
        match: { user_text: "update work" },
        steps: [
          ...(name === "update-parent"
            ? [
                {
                  kind: "native_tool_call",
                  tool_id: "compozy__subagent_delegate",
                  tool_call_id: "update-delegation",
                  raw_input: {
                    task: "update work",
                    title: "Work across update",
                    target: { agent: "update-worker" },
                  },
                },
              ]
            : []),
          { kind: "assistant", text: "Partial work before update." },
          block,
        ],
      },
      {
        name: "continued",
        match: { user_text_contains: "The daemon restarted while your previous turn was running." },
        steps: [{ kind: "assistant", text: "Work finished after update." }],
      },
      {
        name: "wake",
        match: { turn_source: "synthetic" },
        steps: [{ kind: "assistant", text: "Child result received." }],
      },
    ],
  }));
  await writeFile(fixture, JSON.stringify({ version: 2, agents }));
  for (const agent of agents) {
    const directory = join(home, "agents", agent.name);
    await mkdir(directory, { recursive: true });
    const command = [
      driver,
      "--fixture",
      fixture,
      "--agent",
      agent.name,
      "--diagnostics",
      join(home, `${agent.name}.jsonl`),
    ]
      .map(value => `'${value.replaceAll("'", "'\\''")}'`)
      .join(" ");
    await writeFile(
      join(directory, "AGENT.md"),
      `---\nname: ${agent.name}\nprovider: acpmock\ncommand: ${JSON.stringify(command)}\npermissions: approve-all\n---\nComplete the requested work.\n`
    );
  }
  await appendFile(
    join(home, "config.toml"),
    `
[defaults]
agent = "update-worker"
provider = "acpmock"
[daemon]
socket = ${JSON.stringify(join(home, "update-runtime.sock"))}
shutdown_grace = "1ms"
[daemon.load]
admission_threshold = 0
[roles.auto_title]
enabled = false
[provider_usage]
enabled = false
[automation]
enabled = false
[model_catalog.sources.models_dev]
enabled = false
[providers.acpmock]
command = ${JSON.stringify(driver)}
harness = "acp"
auth_mode = "none"
none_security = "local_transport"
${["claude", "codex", "gemini", "opencode"].map(name => `[providers.${name}.models.discovery]\nenabled = false`).join("\n")}
`
  );
  await mkdir(join(home, "project"));
}

async function request<T>(origin: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(new URL(path, origin), {
    method: body === undefined ? "GET" : "POST",
    headers: { "content-type": "application/json", origin },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    signal: AbortSignal.timeout(10_000),
  });
  expect(response.ok, `${path}: ${response.status} ${response.statusText}`).toBe(true);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

function path(session: Session): string {
  return `/api/workspaces/${session.workspace_id}/sessions/${session.id}`;
}
async function events(origin: string, session: Session): Promise<SessionEvent[]> {
  return (await request<{ events: SessionEvent[] }>(origin, `${path(session)}/events?limit=200`))
    .events;
}

export async function startUpdateWork(home: string, origin: string): Promise<UpdateWork> {
  const { workspace } = await request<{ workspace: { id: string } }>(
    origin,
    "/api/workspaces/resolve",
    { path: join(home, "project") }
  );
  const sessions: Session[] = [];
  for (const agent of ["update-parent", "update-worker", "update-worker"]) {
    const { session } = await request<{ session: Session }>(origin, "/api/sessions", {
      workspace: workspace.id,
      agent_name: agent,
    });
    sessions.push(session);
    const response = await fetch(new URL(`${path(session)}/prompt`, origin), {
      method: "POST",
      headers: { "content-type": "application/json", origin },
      body: JSON.stringify({
        message: "update work",
        message_id: `work-${session.id}`,
        idempotency_key: `work-${session.id}`,
      }),
      signal: AbortSignal.timeout(10_000),
    });
    expect(response.ok).toBe(true);
    await response.body?.cancel();
    await expect
      .poll(async () => JSON.stringify(await events(origin, session)))
      .toContain("Partial work before update.");
  }
  const stopped = sessions.pop()!;
  await request(origin, `${path(stopped)}/stop`, {});
  let child: Subagent | undefined;
  await expect
    .poll(async () => {
      const result = await request<{ subagents: Subagent[] }>(
        origin,
        `${path(sessions[0]!)}/subagents`
      );
      child = result.subagents[0];
      return child?.status;
    })
    .toBe("running");
  const childSession = (
    await request<{ session: Session }>(
      origin,
      `/api/workspaces/${workspace.id}/sessions/${child!.child_session_id}`
    )
  ).session;
  sessions.push(childSession);
  await expect
    .poll(async () => JSON.stringify(await events(origin, childSession)))
    .toContain("Partial work before update.");
  for (let i = 0; i < sessions.length; i++) {
    const current = (await request<{ session: Session }>(origin, path(sessions[i]!))).session;
    expect(current.runtime.acp_session_id).toBeTruthy();
    sessions[i] = current;
  }
  return { origin, sessions, stopped, child: child! };
}

export async function expectUpdateWorkRecovered(work: UpdateWork): Promise<void> {
  const { origin } = work;
  for (const session of work.sessions) {
    await expect
      .poll(async () => JSON.stringify(await events(origin, session)))
      .toContain("Work finished after update.");
    const history = await events(origin, session);
    expect(JSON.stringify(history)).toContain("Partial work before update.");
    expect(history.filter(event => event.type === "session.restart_resumed")).toHaveLength(1);
    expect(JSON.stringify(history)).not.toContain("daemon stopped by user");
    const current = (await request<{ session: Session }>(origin, path(session))).session;
    expect(current.runtime.acp_session_id).toBe(session.runtime.acp_session_id);
    expect(current.failure).toBeFalsy();
    expect(current.badge).not.toBe("failed");
    expect(current.stop_reason).not.toBe("user_canceled");
  }
  await expect
    .poll(async () => {
      const { subagents } = await request<{ subagents: Subagent[] }>(
        origin,
        `${path(work.sessions[0]!)}/subagents`
      );
      return subagents.find(row => row.subagent_id === work.child.subagent_id);
    })
    .toMatchObject({ status: "completed", delivery: "delivered" });
  for (const session of work.sessions) {
    await expect
      .poll(async () => request(origin, `${path(session)}/status`))
      .toMatchObject({ active_prompt: false, queue: { entries: 0 } });
  }
  const parentHistory = await events(origin, work.sessions[0]!);
  const childResults = parentHistory.filter(
    event =>
      event.type === "synthetic_reentry" &&
      event.content.synthetic?.kind === "subagent_wake" &&
      event.content.synthetic.subagent_ids?.includes(work.child.subagent_id)
  );
  expect(
    childResults,
    JSON.stringify(parentHistory.filter(event => event.type === "synthetic_reentry"))
  ).toHaveLength(1);
  const stopped = (await request<{ session: Session }>(origin, path(work.stopped))).session;
  expect(stopped.stop_reason).toBe("user_canceled");
  expect(
    (await events(origin, stopped)).filter(event => event.type === "session.restart_resumed")
  ).toHaveLength(0);
}

export async function restartUpdatedRuntime(work: UpdateWork): Promise<void> {
  const action = await request<{ operation_id: string }>(
    work.origin,
    "/api/settings/actions/restart",
    {}
  );
  await expect
    .poll(async () => {
      try {
        const operation = await request<{ status: string; old_pid: number; new_pid: number }>(
          work.origin,
          `/api/settings/actions/restart/${action.operation_id}`
        );
        return operation.status === "ready" && operation.new_pid !== operation.old_pid;
      } catch {
        return false;
      }
    })
    .toBe(true);
}
