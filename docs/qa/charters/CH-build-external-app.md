# CH-build-external-app: Build an app from the published instructions

```yaml
charter:
  id: CH-build-external-app
  mission: As Ana, follow the Node, Go, and chat tutorials to display a real agent reply and recover the same session from a fresh client.
  mode: charter-with-tour
  persona:
    name: Ana
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-build-external-app
  scenarios: [RT-sdk-ts-golden-path, RT-sdk-go-golden-path, RT-sdk-chat-proxy, SITE-app-sdk-reference]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - Use a uniquely isolated lab and matching SDK version; record whether packages came from the release or local build.
      - Follow the authored quickstarts literally and retain the actual session ID, ordered chunks, accepted result, and final transcript.
      - Open JSON, prompt, and SSE API reference tabs and use their generated calls; inspect the browser allowed-operations table.
      - Verify a fresh client reads the same completed reply and no duplicate turn was admitted.
    must_avoid:
      - Do not substitute mock fetch fixtures or source inspection for a running-daemon result.
      - Do not claim a public package install when using a workspace dependency or local Go replace.
```
