import type { SettingsMCPServerEntry } from "@/systems/settings/types";

/** The first project server waits for approval, as a project file declared it. */
export function withWaiting(servers: readonly SettingsMCPServerEntry[]): SettingsMCPServerEntry[] {
  let marked = false;
  return servers.map(server => {
    if (marked || server.scope !== "workspace") return server;
    marked = true;
    return {
      ...server,
      runtime_status: null,
      callable_in_sessions: false,
      approval: { required: true, state: "waiting", definition_digest: "sha256:3f9c" },
    };
  });
}
