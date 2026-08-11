use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::errors::{DiagnosticError, ShellError, sanitize_public_text};
use crate::record::AppUpdateStatus;
use crate::state::ShellState;

pub const DIAGNOSTIC_REPORT_SCHEMA_VERSION: u8 = 1;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct PreviousCrash {
    pub boot_id: String,
    pub boot_phase: String,
    pub started_at: DateTime<Utc>,
}

#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct RuntimeDiagnosticMetadata {
    pub version: Option<String>,
    pub owned: Option<bool>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DiagnosticReport {
    pub schema_version: u8,
    pub boot_id: String,
    pub boot_phase: String,
    pub app_version: String,
    pub runtime_version: Option<String>,
    pub runtime_owned: Option<bool>,
    pub current_error: Option<DiagnosticError>,
    pub previous_crash: Option<PreviousCrash>,
}

impl DiagnosticReport {
    pub fn from_snapshot(
        boot_id: impl Into<String>,
        app_version: impl Into<String>,
        previous_crash: Option<PreviousCrash>,
        startup_diagnostic: Option<&ShellError>,
        runtime: &RuntimeDiagnosticMetadata,
        shell: &ShellState,
        update: &AppUpdateStatus,
    ) -> Self {
        Self {
            schema_version: DIAGNOSTIC_REPORT_SCHEMA_VERSION,
            boot_id: boot_id.into(),
            boot_phase: boot_phase_from_state(shell).to_owned(),
            app_version: app_version.into(),
            runtime_version: sanitize_runtime_version(runtime.version.as_deref()),
            runtime_owned: runtime.owned,
            current_error: current_diagnostic_error(shell, update, startup_diagnostic),
            previous_crash,
        }
    }
}

pub fn sanitize_runtime_version(value: Option<&str>) -> Option<String> {
    let value = value.map(sanitize_public_text)?;
    let value = truncate_utf8(&value, 128);
    (!value.is_empty()).then_some(value)
}

fn truncate_utf8(value: &str, limit: usize) -> String {
    if value.len() <= limit {
        return value.to_owned();
    }
    let mut boundary = limit;
    while !value.is_char_boundary(boundary) {
        boundary -= 1;
    }
    value[..boundary].to_owned()
}

pub fn boot_phase_from_state(state: &ShellState) -> &'static str {
    match state {
        ShellState::Resolving => "resolving",
        ShellState::Provisioning { .. } => "provisioning",
        ShellState::Starting { .. } => "starting",
        ShellState::Attaching => "attaching",
        ShellState::Product { .. } => "product",
        ShellState::Updating { .. } => "updating",
        ShellState::Disconnected { .. } => "disconnected",
        ShellState::Skew { .. } => "skew",
        ShellState::ShellError { .. } => "error",
    }
}

pub fn current_diagnostic_error(
    shell: &ShellState,
    update: &AppUpdateStatus,
    startup_diagnostic: Option<&ShellError>,
) -> Option<DiagnosticError> {
    match shell {
        ShellState::ShellError { error } => Some(DiagnosticError::from(error)),
        _ => update
            .last_error
            .as_ref()
            .map(DiagnosticError::from)
            .or_else(|| startup_diagnostic.map(DiagnosticError::from)),
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;
    use crate::errors::{ShellError, ShellErrorCode};
    use crate::state::ShellState;

    #[test]
    fn should_prefer_the_current_shell_error_in_a_safe_diagnostic_report() {
        let report = DiagnosticReport::from_snapshot(
            "boot-1",
            "1.2.3",
            None,
            Some(&ShellError::from_code(
                ShellErrorCode::ConfigInvalid,
                PathBuf::from("/private/config.log"),
            )),
            &RuntimeDiagnosticMetadata::default(),
            &ShellState::ShellError {
                error: ShellError::from_code(
                    ShellErrorCode::RuntimeUnhealthy,
                    PathBuf::from("/private/runtime.log"),
                ),
            },
            &AppUpdateStatus::default(),
        );

        assert_eq!(report.boot_phase, "error");
        assert_eq!(
            report
                .current_error
                .as_ref()
                .expect("current error exists")
                .code,
            ShellErrorCode::RuntimeUnhealthy
        );
        let value = serde_json::to_string(&report).expect("report serializes");
        assert!(!value.contains("/private"));
        assert!(!value.contains("log_path"));
    }

    #[test]
    fn should_sanitize_and_bound_external_runtime_version_metadata() {
        let version = format!("v{} refresh_token=do-not-copy", "1".repeat(200));
        let report = DiagnosticReport::from_snapshot(
            "boot-1",
            "1.2.3",
            None,
            None,
            &RuntimeDiagnosticMetadata {
                version: Some(version),
                owned: Some(true),
            },
            &ShellState::Resolving,
            &AppUpdateStatus::default(),
        );

        let version = report.runtime_version.expect("runtime version exists");
        assert!(version.len() <= 128);
        assert!(!version.contains("refresh_token"));
        assert!(!version.contains("do-not-copy"));
    }
}
