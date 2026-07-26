const WIRE_KINDS = ["greet", "whois", "say", "capability", "receipt", "trace"] as const;

export type WireKind = (typeof WIRE_KINDS)[number];

export const WIRE_KIND_DOT_CLASS: Record<WireKind, string> = {
  greet: "bg-kind-greet",
  whois: "bg-kind-whois",
  say: "bg-kind-say",
  capability: "bg-kind-capability",
  receipt: "bg-kind-receipt",
  trace: "bg-kind-trace",
};
