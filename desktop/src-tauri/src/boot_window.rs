use serde::Serialize;
use tauri::{
    AppHandle, Manager, Runtime, Webview, WebviewUrl, WebviewWindow, WebviewWindowBuilder,
    webview::PageLoadEvent,
};

use crate::errors::{DiagnosticError, ShellError};
use crate::state::{ProvisionStage, ShellState, UpdateTarget};

const BOOT_BRIDGE_SCRIPT: &str = r#"
if (!Object.hasOwn(window, "__TAURI__")) {
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
}
"#;

pub fn install_control_bridge<R: Runtime>(webview: &Webview<R>) -> tauri::Result<()> {
    webview.eval(BOOT_BRIDGE_SCRIPT)
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
struct BootPayload {
    state: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    action: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    target: Option<UpdateTarget>,
    #[serde(skip_serializing_if = "Option::is_none")]
    stage: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pct: Option<u8>,
    #[serde(skip_serializing_if = "Option::is_none")]
    attempt: Option<u8>,
    retry: bool,
    diagnose: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<DiagnosticError>,
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
            stage: None,
            pct: None,
            attempt: None,
            retry: false,
            diagnose: false,
            error: None,
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
            stage: None,
            pct: None,
            attempt: None,
            retry: false,
            diagnose: true,
            error: None,
        },
    )
}

pub fn show_error_notice<R: Runtime>(app: &AppHandle<R>, error: &ShellError) -> tauri::Result<()> {
    show_payload(
        app,
        BootPayload {
            state: "error".to_owned(),
            message: Some(error.safe_message.clone()),
            action: Some("Retry operation".to_owned()),
            target: None,
            stage: None,
            pct: None,
            attempt: None,
            retry: true,
            diagnose: true,
            error: Some(DiagnosticError::from(error)),
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
    let (stage, pct) = provision_progress(state);
    let attempt = match state {
        ShellState::Starting { attempt } => Some(*attempt),
        _ => None,
    };
    let shell_error = match state {
        ShellState::ShellError { error } => Some(DiagnosticError::from(error)),
        _ => None,
    };
    BootPayload {
        state: name.to_owned(),
        message: message.map(str::to_owned),
        action: action.map(str::to_owned),
        target: None,
        stage,
        pct,
        attempt,
        retry: shell_error.is_some(),
        diagnose: shell_error.is_some() || matches!(state, ShellState::Disconnected { .. }),
        error: shell_error,
    }
}

fn provision_progress(state: &ShellState) -> (Option<String>, Option<u8>) {
    let ShellState::Provisioning { stage } = state else {
        return (None, None);
    };
    match stage {
        ProvisionStage::Download { pct } => (Some("download".to_owned()), Some(*pct)),
        ProvisionStage::Verify => (Some("verify".to_owned()), None),
        ProvisionStage::Install => (Some("install".to_owned()), None),
        ProvisionStage::Start => (Some("start".to_owned()), None),
        ProvisionStage::Ready => (Some("ready".to_owned()), None),
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
        ShellState::ShellError { error } => (
            "error",
            Some(error.safe_message.as_str()),
            Some("Retry operation"),
        ),
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
                stage: None,
                pct: None,
                attempt: None,
                retry: false,
                diagnose: false,
                error: None,
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

    #[test]
    fn should_send_a_typed_safe_error_without_its_local_log_path_to_boot() {
        let payload = payload_from_state(&ShellState::ShellError {
            error: ShellError::new(
                crate::errors::ShellErrorCode::RuntimeUnhealthy,
                "The runtime is not responding.",
                std::path::PathBuf::from("/private/compozy/logs/desktop.log"),
            ),
        });

        let value = serde_json::to_value(payload).expect("boot payload serializes");
        assert_eq!(value["error"]["code"], "runtime_unhealthy");
        assert_eq!(
            value["error"]["safe_message"],
            "The runtime is not responding."
        );
        assert!(value["error"].get("log_path").is_none());
        assert_eq!(value["retry"], true);
        assert_eq!(value["diagnose"], true);
    }

    #[test]
    fn should_send_truthful_provision_progress_and_start_attempt_to_boot() {
        let provisioning = serde_json::to_value(payload_from_state(&ShellState::Provisioning {
            stage: ProvisionStage::Download { pct: 45 },
        }))
        .expect("provision payload serializes");
        let starting =
            serde_json::to_value(payload_from_state(&ShellState::Starting { attempt: 2 }))
                .expect("starting payload serializes");

        assert_eq!(provisioning["stage"], "download");
        assert_eq!(provisioning["pct"], 45);
        assert!(provisioning.get("attempt").is_none());
        assert_eq!(starting["attempt"], 2);
        assert!(starting.get("stage").is_none());
        assert!(starting.get("pct").is_none());
    }
}
