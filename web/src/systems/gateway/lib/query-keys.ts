/**
 * Gateway posture is operator-global — there is no workspace scope to key on.
 * Devices are keyed separately from status so a rename or revoke can reconcile
 * the inventory without refetching provider and address state.
 */
export const gatewayKeys = {
  all: ["gateway"] as const,
  status: () => [...gatewayKeys.all, "status"] as const,
  devices: () => [...gatewayKeys.all, "devices"] as const,
  audit: () => [...gatewayKeys.all, "audit"] as const,
};
