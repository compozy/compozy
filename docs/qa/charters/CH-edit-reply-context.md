# CH-edit-reply-context: Correct an instruction and reply with bounded context

```yaml
charter:
  id: CH-edit-reply-context
  mission: "As Maya, edit and reply across supported provider threads, interrupt the parent cache with a restart, and prove the agent follows current intent without history fetches or cross-workspace context."
  mode: charter-with-tour
  persona:
    name: Maya
    device: laptop
    network: flaky
    locale: en-US
  journey: J-edit-reply-context
  scenarios: [NB-bridge-edit-reply]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Slack: send a message, edit its text, then delete it; inspect the agent-visible prompt behavior for affected message identity and explicit updated/deleted operation."
      - "Telegram: exercise edited_message and edited_channel_post; each accepted edit must create one typed edit-family turn rather than a duplicate ordinary-message turn."
      - "Reply in Slack, Telegram, and Google Chat with an embedded/observed parent, then restart to cold-cache and reply again; warm context includes text/author, cold context stays empty with zero provider history fetch."
      - "Repeat matching parent IDs in two workspaces/conversations and confirm no cache or routed prompt crosses workspace, instance, or conversation ownership; record Discord ordinary edits as explicitly unsupported."
    must_avoid:
      - "Fetching provider history to make a cold-cache case pass; expecting Discord Gateway MESSAGE_UPDATE from the HTTP-interaction adapter; judging intent solely from raw webhook payloads."
  evidence_expectations:
    - "Provider webhook fixtures, routed prompt blocks, provider request negative log for history fetch, cross-workspace identities, and final channel responses reflecting the corrected instruction."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
