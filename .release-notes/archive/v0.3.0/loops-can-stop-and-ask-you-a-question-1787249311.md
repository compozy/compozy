---
title: Loops can stop and ask you a question
type: feature
---

Two new ways for a Loop to bring a person into a run: an `ask` node parks the run until someone answers a question, and a `review` block parks an action node until someone decides on the arguments it is about to run with. Both are answerable from the web run page, the CLI, HTTP, UDS, and native tools, so an agent supervising a Loop never depends on the web UI to unblock it. (#427)

- `compozy loop requests` lists what is waiting, `compozy loop request` reads one, and `compozy loop respond` answers it. Agents get `compozy__loop_requests`, `compozy__loop_request`, and `compozy__loop_respond`.
- The run page's **Needs you** region presents one question at a time — a "Question 1 of N" header steps through several instead of stacking forms. The bounded redacted context preview and the node and generation that asked sit behind a closed **Details** disclosure.
- The answer form renders only what the daemon authorized. An ask generates its fields from the `expect` schema, with enum values as choices and booleans as Yes or No; a review shows the proposed arguments with that node's own decision allowlist, so an unauthorized decision is absent rather than a disabled button. On `edit`, the fields arrive pre-filled with the proposed arguments.
- An answer that fails the shape comes back with the failure on the field that caused it and the request stays open. A request someone else already answered — or whose run has ended — shows the recorded outcome instead of a form.
- Each fan-out lane carries its own request, named by lane and answerable independently. The waits rail counts pending requests alongside timers and events.

```yaml
# a review block on an action node
review:
  when: inputs.environment == "production"
  prompt: "Review the production release"
  decisions: [approve, edit, reject, respond]
  responders: { agents: deny }
  on_reject: { route: repair_release }
```

```bash
compozy loop respond --run-id <run-id> --node publish --decision approve
```
