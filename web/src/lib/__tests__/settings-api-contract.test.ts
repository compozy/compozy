import { describe, expectTypeOf, it } from "vitest";

import type {
  OperationPath,
  OperationQuery,
  OperationRequestBody,
  OperationResponse,
} from "@/lib/api-contract";

type UpdateSettingsGeneralBody = OperationRequestBody<"updateSettingsGeneral">;
type GetSettingsGeneralResponse = OperationResponse<"getSettingsGeneral", 200>;
type PutSettingsMCPServerQuery = OperationQuery<"putSettingsMCPServer">;
type PutSettingsMCPServerBody = OperationRequestBody<"putSettingsMCPServer">;
type ListSettingsMCPServersResponse = OperationResponse<"listSettingsMCPServers", 200>;
type PutSettingsProviderBody = OperationRequestBody<"putSettingsProvider">;
type ListSettingsProvidersResponse = OperationResponse<"listSettingsProviders", 200>;
type GetSettingsObservabilityResponse = OperationResponse<"getSettingsObservability", 200>;
type TriggerSettingsRestartResponse = OperationResponse<"triggerSettingsRestart", 202>;
type GetSettingsRestartStatusPath = OperationPath<"getSettingsRestartStatus">;
type GetSettingsRestartStatusResponse = OperationResponse<"getSettingsRestartStatus", 200>;
type GetSettingsRolesResponse = OperationResponse<"getSettingsRoles", 200>;
type UpdateSettingsRolesBody = OperationRequestBody<"updateSettingsRoles">;

describe("settings openapi contract", () => {
  it("keeps generated settings operation types aligned with the API surface", () => {
    expectTypeOf<UpdateSettingsGeneralBody["config"]["session_timeout"]>().toEqualTypeOf<string>();
    expectTypeOf<UpdateSettingsGeneralBody["config"]["permissions"]["mode"]>().toEqualTypeOf<
      "approve-all" | "approve-reads" | "deny-all"
    >();

    expectTypeOf<GetSettingsGeneralResponse["section"]>().toEqualTypeOf<
      | "general"
      | "memory"
      | "roles"
      | "skills"
      | "automation"
      | "network"
      | "observability"
      | "hooks-extensions"
      | "window-manager"
    >();
    expectTypeOf<GetSettingsGeneralResponse["scope"]>().toEqualTypeOf<"global">();
    expectTypeOf<
      GetSettingsGeneralResponse["available_scopes"][number]
    >().toEqualTypeOf<"global">();
    expectTypeOf<GetSettingsGeneralResponse["config_paths"]["log_file"]>().toEqualTypeOf<string>();
    expectTypeOf<GetSettingsGeneralResponse["config"]["session_timeout"]>().toEqualTypeOf<string>();
    expectTypeOf<GetSettingsGeneralResponse["runtime"]["available"]>().toEqualTypeOf<boolean>();
    expectTypeOf<GetSettingsGeneralResponse["runtime"]["started_at"]>().toEqualTypeOf<
      string | null | undefined
    >();
    expectTypeOf<
      GetSettingsGeneralResponse["actions"]["restart"]["name"]
    >().toEqualTypeOf<string>();
    expectTypeOf<
      GetSettingsGeneralResponse["actions"]["restart"]["available"]
    >().toEqualTypeOf<boolean>();
    expectTypeOf<GetSettingsGeneralResponse["actions"]["restart"]["behavior"]>().toEqualTypeOf<
      "action_trigger" | "applied_now" | "restart_required"
    >();

    expectTypeOf<GetSettingsRolesResponse["scope"]>().toEqualTypeOf<"global">();
    expectTypeOf<keyof NonNullable<UpdateSettingsRolesBody["config"]>>().toEqualTypeOf<
      | "auto_title"
      | "checkpoint_summary"
      | "coordinator"
      | "dream"
      | "memory_controller"
      | "memory_extractor"
    >();

    expectTypeOf<PutSettingsMCPServerQuery["scope"]>().toEqualTypeOf<
      "global" | "workspace" | undefined
    >();
    expectTypeOf<PutSettingsMCPServerQuery["workspace_id"]>().toEqualTypeOf<string | undefined>();
    expectTypeOf<PutSettingsMCPServerQuery["target"]>().toEqualTypeOf<
      "auto" | "config" | "sidecar" | undefined
    >();
    expectTypeOf<PutSettingsMCPServerBody["server"]["name"]>().toEqualTypeOf<string>();
    expectTypeOf<PutSettingsMCPServerBody["server"]["transport"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<PutSettingsMCPServerBody["server"]["command"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<PutSettingsMCPServerBody["server"]["url"]>().toEqualTypeOf<string | undefined>();
    expectTypeOf<PutSettingsMCPServerBody["server"]["auth"]>().toEqualTypeOf<
      | {
          authorization_url?: string;
          client_id?: string;
          client_secret_ref?: string;
          issuer_url?: string;
          metadata_url?: string;
          revocation_url?: string;
          scopes?: string[];
          token_url?: string;
          type?: string;
        }
      | null
      | undefined
    >();
    expectTypeOf<PutSettingsMCPServerBody["server"]["args"]>().toEqualTypeOf<
      string[] | undefined
    >();
    expectTypeOf<PutSettingsMCPServerBody["server"]["env"]>().toEqualTypeOf<
      Record<string, string> | undefined
    >();

    expectTypeOf<ListSettingsMCPServersResponse["collection"]>().toEqualTypeOf<
      "providers" | "mcp-servers" | "sandboxes" | "hooks"
    >();
    expectTypeOf<ListSettingsMCPServersResponse["scope"]>().toEqualTypeOf<"global" | "workspace">();
    expectTypeOf<ListSettingsMCPServersResponse["workspace_id"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<
      ListSettingsMCPServersResponse["mcp_servers"][number]["name"]
    >().toEqualTypeOf<string>();
    expectTypeOf<
      ListSettingsMCPServersResponse["mcp_servers"][number]["transport"]
    >().toEqualTypeOf<string>();
    expectTypeOf<
      ListSettingsMCPServersResponse["mcp_servers"][number]["auth_status"]
    >().toEqualTypeOf<
      | {
          auth_type?: string;
          authorization_url?: string;
          client_id?: string;
          diagnostic?: string;
          expires_at?: string | null;
          issuer?: string;
          refreshable: boolean;
          remote_url?: string;
          revocation_url?: string;
          scope: string;
          scopes?: string[];
          server_name: string;
          status: string;
          token_present: boolean;
          updated_at?: string | null;
          workspace_id?: string;
        }
      | null
      | undefined
    >();
    expectTypeOf<ListSettingsMCPServersResponse["mcp_servers"][number]["scope"]>().toEqualTypeOf<
      "global" | "workspace" | "agent"
    >();
    expectTypeOf<
      ListSettingsMCPServersResponse["mcp_servers"][number]["source_metadata"]["effective_source"]["kind"]
    >().toEqualTypeOf<
      | "builtin-provider"
      | "global-config"
      | "workspace-config"
      | "global-mcp-sidecar"
      | "workspace-mcp-sidecar"
      | "global-agent-file"
      | "workspace-agent-file"
    >();
    expectTypeOf<
      ListSettingsMCPServersResponse["mcp_servers"][number]["source_metadata"]["available_targets"][number]
    >().toEqualTypeOf<
      | "global-config"
      | "workspace-config"
      | "global-mcp-sidecar"
      | "workspace-mcp-sidecar"
      | "global-agent-file"
      | "workspace-agent-file"
    >();
    expectTypeOf<
      ListSettingsMCPServersResponse["mcp_servers"][number]["source_metadata"]["shadowed_sources"]
    >().toEqualTypeOf<
      | {
          agent_name?: string;
          kind:
            | "builtin-provider"
            | "global-config"
            | "workspace-config"
            | "global-mcp-sidecar"
            | "workspace-mcp-sidecar"
            | "global-agent-file"
            | "workspace-agent-file";
          scope: "global" | "workspace" | "agent";
          workspace_id?: string;
        }[]
      | undefined
    >();

    expectTypeOf<
      ListSettingsProvidersResponse["providers"][number]["settings"]["auth_mode"]
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<
      ListSettingsProvidersResponse["providers"][number]["settings"]["env_policy"]
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<
      ListSettingsProvidersResponse["providers"][number]["settings"]["home_policy"]
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<
      ListSettingsProvidersResponse["providers"][number]["settings"]["auth_status_command"]
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<
      ListSettingsProvidersResponse["providers"][number]["settings"]["auth_login_command"]
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<ListSettingsProvidersResponse["providers"][number]["auth_status"]>().toEqualTypeOf<
      | {
          code?: string;
          env_policy: string;
          home_policy: string;
          login_command?: string;
          login_env?: string[];
          message?: string;
          mode: string;
          native_cli?: {
            command?: string;
            error?: string;
            path?: string;
            present: boolean;
            source?: string;
          } | null;
          state: string;
          status_command?: string;
        }
      | null
      | undefined
    >();
    expectTypeOf<PutSettingsProviderBody["settings"]["auth_mode"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<PutSettingsProviderBody["settings"]["env_policy"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<PutSettingsProviderBody["settings"]["home_policy"]>().toEqualTypeOf<
      string | undefined
    >();

    expectTypeOf<
      GetSettingsObservabilityResponse["log_tail"]["available"]
    >().toEqualTypeOf<boolean>();
    expectTypeOf<GetSettingsObservabilityResponse["log_tail"]["stream_url"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<GetSettingsObservabilityResponse["log_tail"]["transport"]>().toEqualTypeOf<
      "sse" | undefined
    >();

    expectTypeOf<TriggerSettingsRestartResponse["operation_id"]>().toEqualTypeOf<string>();
    expectTypeOf<TriggerSettingsRestartResponse["status"]>().toEqualTypeOf<
      "failed" | "pending" | "ready" | "starting" | "stopping" | "waiting_release"
    >();
    expectTypeOf<TriggerSettingsRestartResponse["status_url"]>().toEqualTypeOf<string>();
    expectTypeOf<TriggerSettingsRestartResponse["active_session_count"]>().toEqualTypeOf<number>();

    expectTypeOf<GetSettingsRestartStatusPath["operation_id"]>().toEqualTypeOf<string>();
    expectTypeOf<GetSettingsRestartStatusResponse["old_pid"]>().toEqualTypeOf<number>();
    expectTypeOf<GetSettingsRestartStatusResponse["old_started_at"]>().toEqualTypeOf<string>();
    expectTypeOf<GetSettingsRestartStatusResponse["old_socket_path"]>().toEqualTypeOf<string>();
    expectTypeOf<GetSettingsRestartStatusResponse["new_pid"]>().toEqualTypeOf<number | undefined>();
    expectTypeOf<GetSettingsRestartStatusResponse["failure_reason"]>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<GetSettingsRestartStatusResponse["completed_at"]>().toEqualTypeOf<
      string | null | undefined
    >();
    expectTypeOf<GetSettingsRestartStatusResponse["started_at"]>().toEqualTypeOf<string>();
    expectTypeOf<GetSettingsRestartStatusResponse["updated_at"]>().toEqualTypeOf<string>();
  });
});
