import { useEffect } from "react";

import { useMCPAuthorize } from "./use-mcp-authorize";
import { useSettingsMCPServer } from "./use-settings-collections";

/** Poll the authorizing definition, even when another same-name row or workspace is selected. */
export function useMCPDefinitionAuthorization() {
  const authorize = useMCPAuthorize();
  const filter = authorize.filter;
  const query = useSettingsMCPServer(authorize.server ?? "", filter ?? {}, {
    enabled: filter !== null && authorize.server !== null,
    refetchInterval: 2000,
  });
  const server = filter ? (query.data?.server ?? null) : null;
  const status = server?.auth_status;
  const { acknowledgeStatus } = authorize;
  useEffect(() => {
    if (status) acknowledgeStatus(status.status, status.token_present);
  }, [acknowledgeStatus, status]);
  return { authorize, server, scope: filter?.scope ?? "user", query };
}
