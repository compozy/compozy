# Session timeline React lifecycle console gate

- Source: J-14 / RT-047 and J-11 / RT-023; `BUG-20260811-session-timeline-lifecycle-warning`
- Why automate: jsdom does not execute the browser layout and `ResizeObserver` timing that made TanStack Virtual call `flushSync` from its layout update, so a component test stayed green even with the production correction removed.
- Suggested layer: E2E browser (`make test-e2e-web`) with a running-session fixture and console-error assertion.
- Spec sketch: open a session with enough variable-height messages to activate timeline virtualization, wait for row and viewport measurement, page older messages, append a live turn, and reconnect after backgrounding. Fail on React lifecycle console errors, especially nested `flushSync`; assert the transcript remains readable, mounted rows stay bounded, and one fenced EventSource resumes from the last cursor.
- Status: proposed
