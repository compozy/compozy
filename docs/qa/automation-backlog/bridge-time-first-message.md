# Bridge time-to-first-message replay budget

- Legacy ID: AB-016
- Source: J-connect-bridge-provider / NB-bridge-provider-setup / CH-first-slack-response, CH-guided-setup-credentials, CH-structured-telegram-setup
- Why automate: setup ergonomics regress through extra commands and hidden prerequisites even when every individual command still works. The initial Slack baseline is about seven operator actions and the Telegram guided/structured baseline is about four; other providers need measured baselines before a threshold can be set.
- Suggested layer: scripted CLI/HTTP/UDS replay harness against isolated fake-provider endpoints, publishing operator-action counts and elapsed time without timing-flaky pass criteria.
- Spec sketch: define an action as one deliberate operator input or external console change, replay Slack and Telegram setup from no bridge to the first confirmed real provider message, and record commands, prompts, remediation loops, action count, and elapsed time. Fail on extra mandatory actions relative to an approved baseline; report measurements only for the other six providers until baselines are reviewed.
- Status: proposed
