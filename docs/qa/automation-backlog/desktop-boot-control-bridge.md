# Initial desktop boot controls — configured webview bridge

- Source: `J-desktop-first-run` / `APP-install-first-run-provision` / `BUG-20260810-boot-controls-unavailable`
- Why automate: Rust webview mocks do not execute the JavaScript loaded by the boot window declared in `tauri.conf.json`, so they cannot prove that the initial configured window receives the minimal command bridge.
- Suggested layer: macOS desktop E2E using the built development app and an isolated stopped runtime.
- Spec sketch: launch the app into `runtime_start_failed`, select `Retry operation` and `Show diagnostics`, assert neither reports `app control unavailable`, independently confirm the retry through `compozy app status`, and confirm both diagnostic paths render. Repeat after the boot window is closed and recreated.
- Status: proposed
