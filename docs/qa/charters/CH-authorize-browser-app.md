# CH-authorize-browser-app: Audit a trusted app from grant to revocation

```yaml
charter:
  id: CH-authorize-browser-app
  mission: As Dora, grant one browser origin app access, reject unauthorized use, and revoke an active connection without changing existing local callers.
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-authorize-browser-app
  scenarios: [ET-app-token-management, ET-app-token-browser-access]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - Create, list, replace origins, clear origins, and revoke through structured local CLI and HTTP/UDS, then re-read metadata.
      - Use a real foreign browser origin for preflight, token-bearing fetch, and an event stream; test allowed and denied operations.
      - Revoke during the stream and verify closure within five seconds, next-request denial, and unchanged local Web/tokenless access.
      - Try Gateway authentication and inspect last use before and after the denial; keep paired devices separate from token inventory.
    must_avoid:
      - Do not persist secrets in screenshots, logs, reports, or source files; redact credentials before saving evidence.
      - Do not represent a curl Origin header as a completed browser CORS walk.
```
