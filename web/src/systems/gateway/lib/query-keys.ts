/**
 * Gateway posture is operator-global — there is no workspace scope to key on.
 * Devices are keyed separately from status so mutations can invalidate the
 * inventories they actually change; rename and revoke currently refresh both.
 */
export const gatewayKeys = {
  all: ["gateway"] as const,
  status: () => [...gatewayKeys.all, "status"] as const,
  devices: () => [...gatewayKeys.all, "devices"] as const,
  audit: () => [...gatewayKeys.all, "audit"] as const,
};
