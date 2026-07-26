# CH-automation-crud-loop-target: Manage Loop-target jobs and triggers end to end

```yaml
charter:
  id: CH-automation-crud-loop-target
  mission: "As Bruno, create, edit, disable, re-enable, fire, and delete schedule/event automations that target a Loop, using every product modal and verifying persisted state after refresh."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [TA-automation-crud-loop-target, LP-034, LP-035]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Create one schedule and one event trigger through their modals, including an invalid payload mapping before the valid save."
      - "Edit name, target inputs, and enabled state; cancel one dirty modal and verify no ghost update."
      - "Prove disabled does not fire, re-enabled fires one real Loop run, and deletion survives refresh."
      - "Attempt a cross-workspace Loop target and an unsupported start kind."
```
