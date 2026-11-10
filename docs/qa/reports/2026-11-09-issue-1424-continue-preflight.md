# Continue preflight and advanced default semantics

Scope: the Web Continue dialog against a real isolated daemon and ACP mock subprocesses.
Persona: Bruno, moving a conversation to another agent with advanced runtime options.
ACP responses are deterministic. Model capability descriptors and the Cursor matrix were injected
at the browser's model-catalog boundary because the lab has no live provider catalog; agents,
permission policy, the continuation request, and the child launch ran on the real daemon. This
report does not claim live-provider or account coverage.

| Journey | Status | Evidence |
| --- | --- | --- |
| Provider default on an inherited option | Pass | Control moved 1m to 200k; POST sent `context_window=200k`; child effective options report 200k |
| Permission policy excludes inherited `allow_all=on` | Pass | Projection lists it; control shows off; POST has no `acp_options`; child effective options omit it at approve-reads |
| Cursor omitted effort against the advertised default | Pass | Local refusal, no POST; a message-only edit keeps the refusal visible |
| Cursor configuration requiring thinking | Pass | Local refusal, no POST; enabling Thinking sends the exact tuple |
| Exact Cursor tuple with no live catalog | Pass | Visible start failure; child selected runtime stays Cursor with no effective runtime, so no fallback ran |
| Explicit option removed by a catalog refresh | Component suite only | The lab has no control that refetches the injected catalog; covered by `session-continue-dialog.test.tsx` |

## Investigation

`setRuntimeACPSelection` received `null` without an option ID, so Provider default changed nothing
in any selector surface. Passing the option ID fixes the removal. Removal alone would not reach
the launch: an omitted option inherits from the agent, and that meaning stays unchanged. Continue
therefore submits the provider's advertised default as an explicit value, and refuses when no
default is advertised. Unrestricted options the permission level drops are reported by the daemon
in `effective_runtime.policy_excluded_acp_options`, so the Web does not copy the provider-specific
unrestricted-mode rules. `effective_runtime.acp_options` keeps reporting them for existing readers.

## Rendered evidence

![Provider default displays the advertised value](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1424/650f6d5ed/01-provider-default.png)

![Inherited allow_all shows the policy-effective value](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1424/650f6d5ed/02-policy-allow-all.png)

![Cursor omitted effort refused before creation](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1424/650f6d5ed/03-cursor-default-refusal.png)

![Cursor thinking requirement refused before creation](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1424/650f6d5ed/04-cursor-thinking-refusal.png)

Targeted teardown reported `clean: true`. The installed application and operator daemon were not
restarted.
