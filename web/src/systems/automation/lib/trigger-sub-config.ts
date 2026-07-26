/** Pure form contract for event-family-specific Trigger configuration. */
export interface SubConfigValues {
  hookName: string;
  extExt: string;
  extEvent: string;
  endpointSlug: string;
  webhookId: string;
  webhookSecret: string;
}
