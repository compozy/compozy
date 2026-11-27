type IngressReachability = "off" | "unconfirmed" | "live" | "broken" | "reconfirmation_required";
type IngressTone = "neutral" | "warning" | "success" | "destructive";

const INGRESS_REACHABILITY_COPY: Record<
  IngressReachability,
  { label: string; tone: IngressTone; detail: string }
> = {
  off: {
    label: "Off",
    tone: "neutral",
    detail: "Public delivery ingress is turned off, so this URL would not answer.",
  },
  unconfirmed: {
    label: "Unconfirmed",
    tone: "warning",
    detail: "Confirm this subject against the current public address before publishing the URL.",
  },
  live: {
    label: "Live",
    tone: "success",
    detail: "Confirmed against the verified public address.",
  },
  broken: {
    label: "Broken",
    tone: "destructive",
    detail: "The public address is no longer reachable.",
  },
  reconfirmation_required: {
    label: "Needs confirmation",
    tone: "warning",
    detail: "The public address changed. Confirm again before senders use this URL.",
  },
};

export function ingressReachabilityCopy(reachability: string) {
  if (isIngressReachability(reachability)) {
    return INGRESS_REACHABILITY_COPY[reachability];
  }
  return {
    label: "Unknown",
    tone: "neutral" as IngressTone,
    detail: "Delivery status is unavailable.",
  };
}

function isIngressReachability(value: string): value is IngressReachability {
  return Object.prototype.hasOwnProperty.call(INGRESS_REACHABILITY_COPY, value);
}
