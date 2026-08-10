use serde::Serialize;
use tauri::{
    AppHandle, Manager, Runtime, WebviewUrl, WebviewWindow, WebviewWindowBuilder,
    webview::PageLoadEvent,
};

use crate::errors::ShellError;
use crate::state::{ShellState, UpdateTarget};

const BOOT_BRIDGE_SCRIPT: &str = r#"
Object.defineProperty(window, "__TAURI__", {
  configurable: false,
  enumerable: false,
  value: Object.freeze({
    core: Object.freeze({
      invoke: (command, args) => window.__TAURI_INTERNALS__.invoke(command, args)
    })
  }),
  writable: false
});
"#;

#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
struct BootPayload {
    state: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    action: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    target: Option<UpdateTarget>,
}

pub fn render<R: Runtime>(window: &WebviewWindow<R>, state: &ShellState) -> tauri::Result<()> {
    render_payload(window, &payload_from_state(state))
}

pub fn show<R: Runtime>(app: &AppHandle<R>, state: ShellState) -> tauri::Result<()> {
    show_payload(app, payload_from_state(&state))
}

pub fn show_update_offer<R: Runtime>(
    app: &AppHandle<R>,
    target: UpdateTarget,
) -> tauri::Result<()> {
    show_payload(
        app,
        BootPayload {
            state: "product".to_owned(),
            message: None,
            action: None,
            target: Some(target),
        },
    )
}

pub fn show_managed_update<R: Runtime>(app: &AppHandle<R>, action: &str) -> tauri::Result<()> {
    show_payload(
        app,
        BootPayload {
            state: "skew".to_owned(),
            message: None,
            action: Some(action.to_owned()),
            target: None,
        },
    )
}

pub fn show_error_notice<R: Runtime>(app: &AppHandle<R>, error: &ShellError) -> tauri::Result<()> {
    show_payload(
        app,
        BootPayload {
            state: "error".to_owned(),
            message: Some(error.safe_message.clone()),
            action: None,
            target: None,
        },
    )
}

pub fn close<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<()> {
    if let Some(boot) = app.get_webview_window("boot") {
        boot.close()?;
    }
    Ok(())
}

fn show_payload<R: Runtime>(app: &AppHandle<R>, payload: BootPayload) -> tauri::Result<()> {
    if let Some(boot) = app.get_webview_window("boot") {
        render_payload(&boot, &payload)?;
        boot.show()?;
        boot.set_focus()?;
        return Ok(());
    }
    let page_payload = payload.clone();
    let boot = WebviewWindowBuilder::new(app, "boot", WebviewUrl::App("boot.html".into()))
        .title("CompozyOS")
        .inner_size(520.0, 320.0)
        .min_inner_size(420.0, 280.0)
        .resizable(false)
        .center()
        .background_color(tauri::window::Color(19, 18, 17, 255))
        .initialization_script(BOOT_BRIDGE_SCRIPT)
        .on_page_load(move |window, event| {
            if event.event() == PageLoadEvent::Finished
                && let Err(error) = render_payload(&window, &page_payload)
            {
                crate::logging::error(format!("render recreated boot state: {error}"));
            }
        })
        .build()?;
    boot.show()?;
    boot.set_focus()
}

fn render_payload<R: Runtime>(
    window: &WebviewWindow<R>,
    payload: &BootPayload,
) -> tauri::Result<()> {
    let payload = serde_json::to_string(payload)?;
    window.eval(format!(
        "window.__COMPOZY_BOOT__ && window.__COMPOZY_BOOT__.render({payload});"
    ))
}

fn payload_from_state(state: &ShellState) -> BootPayload {
    let (name, message, action) = boot_copy(state);
    BootPayload {
        state: name.to_owned(),
        message: message.map(str::to_owned),
        action: action.map(str::to_owned),
        target: None,
    }
}

fn boot_copy(state: &ShellState) -> (&'static str, Option<&str>, Option<&str>) {
    match state {
        ShellState::Resolving => ("resolving", None, None),
        ShellState::Provisioning { .. } => ("provisioning", None, None),
        ShellState::Starting { .. } => ("starting", None, None),
        ShellState::Attaching => ("attaching", None, None),
        ShellState::Product { .. } => ("product", None, None),
        ShellState::Updating { .. } => ("updating", None, None),
        ShellState::Disconnected { .. } => (
            "disconnected",
            None,
            Some("CompozyOS will reconnect when the runtime returns."),
        ),
        ShellState::Skew { newer, .. } => (
            "skew",
            None,
            Some(if *newer {
                "Update the CompozyOS app."
            } else {
                "Update the CompozyOS runtime through its install channel."
            }),
        ),
        ShellState::ShellError { error } => ("error", Some(error.safe_message.as_str()), None),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_expose_only_the_requested_update_target_to_the_boot_bridge() {
        for target in [UpdateTarget::App, UpdateTarget::Runtime] {
            let payload = BootPayload {
                state: "product".to_owned(),
                message: None,
                action: None,
                target: Some(target),
            };
            let value = serde_json::to_value(payload).expect("offer serializes");
            assert_eq!(value["state"], "product");
            assert_eq!(
                value["target"],
                match target {
                    UpdateTarget::App => "app",
                    UpdateTarget::Runtime => "runtime",
                }
            );
        }
    }

    #[test]
    fn should_create_visible_boot_window_for_initial_resolution() {
        let app = tauri::test::mock_app();

        show(app.handle(), ShellState::Resolving).expect("initial boot window is shown");

        assert!(app.get_webview_window("boot").is_some());
    }
}
