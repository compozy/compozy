# Continue runtime selection regression

Scope: the CLI/API continuation journey with a real isolated daemon and ACP mock subprocesses.
Persona: Rafa, moving an existing Claude conversation to a chosen Codex runtime.
The rendered Web uses the same isolated daemon. ACP responses are deterministic; this report
does not claim live-provider inference or account authentication coverage.

| Journey | Status | Evidence |
| --- | --- | --- |
| Continue with an immediate first message | Pass | First and second user events retain Codex/GPT-6-Astra/high/normal |
| Stage a continuation, then prompt twice | Pass | First and second user events retain the same complete selection |
| Refuse an unstartable selection without fallback | Pass | CLI error includes committed child; unbound with runtime failure; zero user events |
| Ordinary agent fallback remains available | Pass | The same failing agent binds Claude for an ordinary new session |
| Confirm unchanged defaults in the rendered Web dialog | Pass | Child answers both UI prompts on Codex; composer and API agree |
| Continue with Fast selected | Pass | Requested/effective/last-known speed is fast; speed resolution applied |

## Investigation

The Web dialog serialized provider, model, reasoning effort and explicit speed only after a local
runtime edit. Keeping the displayed defaults omitted the runtime entirely, so the request did not
freeze the displayed selection. Its owning component regression failed before the fix
(`runtime` was undefined), and now verifies the full displayed snapshot on submit. The derive
manager persists that choice as `runtime.selected`; prompt admission prefers it over the prior binding.
The submission uses the selector's capability normalization. Inherited ACP options stay
daemon-owned even after editing Fast or reasoning, so this snapshot does not promote an inherited
permission mode to an explicit override. A configured effort absent from the model's advertised
choices is refused visibly until the user chooses a supported effort, rather than omitting it and
letting launch restore that unsupported setting.

An additional confirmed defect is at the unbound runtime's first bind: it walks the target agent's
fallback chain even when a derived child explicitly selected another runtime. A rejected Codex
launch can therefore succeed on Claude while `selected` still names Codex. The deterministic
regression failed before the production change (`SendPrompt` returned success instead of the
chosen launch's error). The fix keeps explicitly selected derived sessions on their chosen route.
The guard applies to continuations only; fork and ordinary-session fallback behavior is preserved.
The original operator request body was not retained, so these reproductions establish the two
observable failure modes without claiming an exact reconstruction of that historical request.

## Verification

- Canonical `TestContinueSession` checks full runtime identity on immediate/staged first turns,
  a later turn, durable `prompt_runtime`, restart persistence, and visible launch refusal.
- Focused derive/fork/fallback checks pass with `CGO_ENABLED=1` and `-race`.
- Pinned golangci-lint v2.15.0 on `./internal/session`: zero issues.
- Root Turborepo selector/Continue/composer suites: 136 tests passed; Web typecheck passed.
- React Doctor on the changed Web files: 100/100, no findings; focused oxlint and formatting pass.
- Real daemon journeys above passed through public CLI/API and the rendered Web. Provider,
  model and effort agree across selected/requested/effective/last-known; Normal and Fast were
  independently confirmed through ACP readback. First and later `prompt_runtime` rows agree.
- Strict targeted evidence audit passed. Targeted teardown reports `clean: true`; no owned
  lab process remains. The installed application and operator daemon were never restarted.
- CI and bot reviews are separate delivery gates on the PR head.

## Rendered evidence

The dialog was confirmed with Codex/GPT-6-Astra/high/normal displayed and no runtime edit. The
new session answered the initial message and a second UI prompt; its composer retained Codex.
The rendered walk used the backend from `9adc8aacd` and Web changes subsequently committed as
`996d49d5e`. The object-storage key identifies that backend build. Later selector normalization
and inherited-option corrections are covered by the focused component suites; these screenshots
do not claim to exercise those additional interaction cases.

![Continue dialog preserving displayed defaults](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1414/9adc8aacd/01-continue-runtime.png)

![Claude source and Codex continuation after two prompts](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1414/9adc8aacd/02-child-composer.png)
