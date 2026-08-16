# CH-session-attachments: Trust files through the full session lifecycle

```yaml
charter:
  id: CH-session-attachments
  mission: "As Théo, attach supported files to a session and prove their previews, dispatch, reload, queue retention, capability gates, and cleanup stay truthful."
  mode: charter-with-tour
  persona:
    name: Théo
    goal: "Return to a long-lived background agent session and immediately see my persisted conversation, current and truthful, with the live run resuming."
    device: desktop
    network: wifi-fast
    modality: mouse-keyboard
    locale: en-US
    patience_seconds: 15
  journey: J-session-attachments
  scenarios: [ET-session-attachment-picker, ET-session-attachment-paste-reload, ET-session-attachment-multiple-drop, ET-session-attachment-oversize, ET-session-attachment-unsupported-type, ET-session-attachment-model-gate, RT-session-queued-attachment-dispatch, RT-session-delete-attachment-files]
  tour: Network Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Use picker, paste, and multi-file drop, then compare preview order with the persisted transcript after a cold reload."
      - "Try an oversized file, disguised unsupported bytes, and image/PDF inputs against unknown and explicitly unsupported bound ACP agent capabilities without losing the draft."
      - "Throttle to Slow 3G, cut the network during upload, restore it, and check for a retry, bounded spinner, one submission, and an intact draft."
      - "Queue an attachment-bearing prompt while the session is busy and verify the same ordered refs dispatch once."
      - "Delete the stopped session and confirm only its workspace-scoped attachment tree is removed."
    must_avoid:
      - "Do not use component mocks as behavioral evidence; exercise the isolated daemon and web app through public surfaces."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report, never here. -->
