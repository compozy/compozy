use std::path::Path;

use serde::Deserialize;
use serde_json::{Value, json};
use tauri::{AppHandle, Wry};
use tauri_plugin_clipboard_manager::ClipboardExt;

use crate::app_state::AppStatePublisher;
use crate::control::ControlError;
use crate::diagnostics::export::{self, DiagnosticExportError};
use crate::home::CompozyHome;

pub fn report(publisher: &AppStatePublisher) -> Result<Value, ControlError> {
    serde_json::to_value(publisher.diagnostic_report()).map_err(|error| {
        crate::logging::error(format!("serialize diagnostic report: {error}"));
        ControlError::new(
            "diagnostic_unavailable",
            "The diagnostic report could not be created.",
        )
    })
}

pub fn copy(app: &AppHandle<Wry>, publisher: &AppStatePublisher) -> Result<Value, ControlError> {
    let text = serde_json::to_string_pretty(&publisher.diagnostic_report()).map_err(|error| {
        crate::logging::error(format!("serialize copied diagnostics: {error}"));
        ControlError::new(
            "diagnostic_unavailable",
            "The diagnostic report could not be created.",
        )
    })?;
    app.clipboard().write_text(text).map_err(|error| {
        crate::logging::error(format!("copy diagnostics to clipboard: {error}"));
        ControlError::new(
            "diagnostic_copy_failed",
            "The diagnostic report could not be copied.",
        )
    })?;
    Ok(json!({"copied": true}))
}

pub fn export(
    home: &CompozyHome,
    publisher: &AppStatePublisher,
    params: &Value,
) -> Result<Value, ControlError> {
    let params: DiagnosticExportParams = serde_json::from_value(params.clone()).map_err(|_| {
        ControlError::new(
            "diagnostic_export_invalid_request",
            "The diagnostic export request is invalid.",
        )
    })?;
    let output_path = params.output_path.as_deref().map(Path::new);
    let exported = export::export(
        home,
        &publisher.diagnostic_report(),
        params.consent,
        output_path,
    )
    .map_err(control_error_for_export)?;
    serde_json::to_value(exported).map_err(|error| {
        crate::logging::error(format!("serialize diagnostic export: {error}"));
        ControlError::new(
            "diagnostic_export_failed",
            "The diagnostic export could not be completed.",
        )
    })
}

#[derive(Deserialize)]
struct DiagnosticExportParams {
    consent: bool,
    #[serde(default)]
    output_path: Option<String>,
}

fn control_error_for_export(error: DiagnosticExportError) -> ControlError {
    match error {
        DiagnosticExportError::ConsentRequired => ControlError::new(
            "diagnostic_consent_required",
            "Confirm diagnostic export before creating a local bundle.",
        ),
        DiagnosticExportError::InvalidPath => ControlError::new(
            "diagnostic_export_invalid_path",
            "The diagnostic export path is invalid.",
        ),
        DiagnosticExportError::AlreadyExists => ControlError::new(
            "diagnostic_export_exists",
            "A diagnostic export already exists at that path.",
        ),
        DiagnosticExportError::Create(_) | DiagnosticExportError::TooLarge => {
            crate::logging::error(format!("export diagnostics: {error}"));
            ControlError::new(
                "diagnostic_export_failed",
                "The diagnostic export could not be completed.",
            )
        }
    }
}
