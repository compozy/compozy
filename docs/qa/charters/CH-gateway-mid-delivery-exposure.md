# CH-gateway-mid-delivery-exposure: Change exposure during a public delivery

```yaml
charter:
  id: CH-gateway-mid-delivery-exposure
  mission: "As Bruno, run the Network Tour while a repository webhook and bridge callback are in flight, changing public exposure and provider address at each boundary, and prove accepted work is attributable while withdrawn or offline delivery fails visibly with no hidden queue."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: flaky
    locale: en-US
  journey: J-deliver-through-public-gateway
  scenarios: [RT-gateway-public-ingress-bindings, RT-gateway-offline-delivery-redelivery, TA-056, TA-060, NB-bridge-provider-setup]
  tour: Network Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Send valid, invalid-signature, stale, replayed, oversized, disabled-trigger, and rate-limited deliveries while toggling public ingress before admission, after claim, and before destination dispatch."
      - "Stop the daemon for one sender attempt, prove sender-visible failure and zero hidden work, restore health, then use the sender's own redelivery action and observe exactly one attributed result."
      - "Change the verified public address and confirm every trigger and bridge becomes reconfirmation-required before any new URL is presented as live."
      - "Delete and recreate a subject, attempt a cross-workspace bind, and keep an external-proxy bridge unbound; no orphan, scope leak, or gateway-owned rewrite may remain."
    must_avoid:
      - "Filing missing store-and-forward as a defect; accepting only a Compozy-side result without the sender receipt."
  evidence_expectations:
    - "Sender receipts, binding generations, public status transitions, run/bridge attribution, dedup result, no-work-during-downtime proof, and redelivery outcome."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

