# J-operate-remote-computers — Use agents on your computers

The controller adds a computer to the operator's account, runs work on that computer, and can
revoke a device or stop serving a route.

```yaml
journey:
  id: J-operate-remote-computers
  name: "Use agents on your computers"
  value_statement: "Run work on the intended computer and control which devices can reach it."
  personas: [Iris, Bruno]
  entry_points:
    - url: "Settings → Computers"
      origin: direct
    - url: "compozy computer add ssh <host>"
      origin: direct
    - url: "compozy remote status --json"
      origin: direct
  actions:
    - step: 1
      verb: "Sign in and add a computer over SSH or from the account"
      expected_observable: "The controller shows the computer and its route."
    - step: 2
      verb: "Select the computer and start a session"
      expected_observable: "The session, workspace, and subsequent terminal use that computer."
    - step: 3
      verb: "Open a hosted browser or confirm webhook ingress on the target"
      expected_observable: "The browser uses its own device grant and each webhook reaches its confirmed trigger."
    - step: 4
      verb: "Revoke a device and inspect its former connection"
      expected_observable: "The device's live streams close. Work stays on the computer."
  goal:
    observable: "The controller can find and use the intended computer from the routes it authorizes."
    side_effects: [computer-linked, session-started]
  true_end_state: "The session is on the selected computer with reachability and device access visible to the operator."
  exit:
    natural: "The operator leaves the computer linked or removes its link from the owning installation."
  crosses: [account, Web, CLI, HTTP, UDS, SSH, Automations]
```

The upgrade entry is covered by `RT-computers-upgrade`. The first release boot explains the removal
and keeps app tokens, local work, and trigger bindings. The operator adds computers again after sign-in.
