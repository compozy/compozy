use std::fs;
use std::io::{self, Write};
use std::path::Path;

use atomicwrites::{AllowOverwrite, AtomicFile};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::diagnostics::DiagnosticReport;
use crate::errors::ShellError;
use crate::state::ShellState;

pub const APP_STATE_SCHEMA_VERSION: u8 = 1;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct AppRecord {
    pub schema_version: u8,
    pub pid: u32,
    pub started_at: DateTime<Utc>,
    pub diagnostic_report: DiagnosticReport,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub app_version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub channel: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub update: Option<AppUpdateStatus>,
    #[serde(flatten)]
    pub shell: ShellState,
}

impl AppRecord {
    pub fn new(
        pid: u32,
        started_at: DateTime<Utc>,
        diagnostic_report: DiagnosticReport,
        shell: ShellState,
    ) -> Self {
        Self {
            schema_version: APP_STATE_SCHEMA_VERSION,
            pid,
            started_at,
            diagnostic_report,
            app_version: None,
            channel: None,
            update: None,
            shell,
        }
    }

    pub fn with_metadata(
        mut self,
        app_version: impl Into<String>,
        channel: impl Into<String>,
        update: AppUpdateStatus,
    ) -> Self {
        self.app_version = Some(app_version.into());
        self.channel = Some(channel.into());
        self.update = Some(update);
        self
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum AppUpdatePhase {
    Idle,
    Checking,
    Downloading,
    Ready,
    Failed,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum RuntimeUpdatePhase {
    Idle,
    Available,
    Applying,
    RecoveryRequired,
    Failed,
    Managed,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct AppUpdateStatus {
    pub app_state: AppUpdatePhase,
    pub runtime_state: RuntimeUpdatePhase,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub app_available: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime_available: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_checked_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_error: Option<ShellError>,
}

impl Default for AppUpdateStatus {
    fn default() -> Self {
        Self {
            app_state: AppUpdatePhase::Idle,
            runtime_state: RuntimeUpdatePhase::Idle,
            app_available: None,
            runtime_available: None,
            last_checked_at: None,
            last_error: None,
        }
    }
}

pub fn write_atomic(path: &Path, record: &AppRecord) -> io::Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    AtomicFile::new(path, AllowOverwrite).write(|file| {
        serde_json::to_writer_pretty(&mut *file, record).map_err(io::Error::other)?;
        file.write_all(b"\n")?;
        file.sync_all()
    })?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use chrono::TimeZone;
    use std::path::PathBuf;
    use std::sync::Arc;
    use std::sync::atomic::{AtomicBool, Ordering};

    use crate::diagnostics::{DiagnosticReport, RuntimeDiagnosticMetadata};
    use crate::errors::{ShellError, ShellErrorCode};
    use crate::state::{ProvisionStage, UpdateTarget};

    use super::*;

    fn report(shell: &ShellState) -> DiagnosticReport {
        DiagnosticReport::from_snapshot(
            "boot-test",
            "0.4.1",
            None,
            None,
            &RuntimeDiagnosticMetadata::default(),
            shell,
            &AppUpdateStatus::default(),
        )
    }

    #[test]
    fn should_write_schema_valid_records_without_partial_reads() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("app.json");
        let done = Arc::new(AtomicBool::new(false));
        let reader_done = Arc::clone(&done);
        let reader_path = path.clone();
        let reader = std::thread::spawn(move || {
            while !reader_done.load(Ordering::Acquire) {
                match fs::read(&reader_path) {
                    Ok(bytes) => {
                        serde_json::from_slice::<AppRecord>(&bytes)
                            .expect("every observable record is complete");
                    }
                    Err(error) if error.kind() == io::ErrorKind::NotFound => {}
                    Err(error) => panic!("read app record: {error}"),
                }
            }
        });
        for index in 0..50 {
            let shell = if index % 2 == 0 {
                ShellState::Resolving
            } else {
                ShellState::Attaching
            };
            write_atomic(
                &path,
                &AppRecord::new(42, Utc::now(), report(&shell), shell),
            )
            .expect("record writes atomically");
        }
        done.store(true, Ordering::Release);
        reader.join().expect("reader joins");
    }

    #[test]
    fn should_represent_transitional_and_typed_error_states() {
        let states = [
            ShellState::Provisioning {
                stage: ProvisionStage::Verify,
            },
            ShellState::Attaching,
            ShellState::Updating {
                target: UpdateTarget::Runtime,
            },
            ShellState::ShellError {
                error: ShellError::new(
                    ShellErrorCode::RuntimeUnhealthy,
                    "The runtime is not responding.",
                    PathBuf::from("/tmp/compozy.log"),
                ),
            },
        ];
        for state in states {
            let value = serde_json::to_value(AppRecord::new(42, Utc::now(), report(&state), state))
                .expect("record serializes");
            assert!(
                value
                    .get("state")
                    .and_then(serde_json::Value::as_str)
                    .is_some()
            );
            if value["state"] == "error" {
                assert!(value["error"].get("code").is_some());
                assert!(value["error"].get("safe_message").is_some());
                assert!(value["error"].get("log_path").is_some());
            }
        }
    }

    #[test]
    fn should_persist_the_injected_tauri_package_version_in_the_diagnostic_report() {
        let report = DiagnosticReport::from_snapshot(
            "boot-version",
            "0.4.1",
            None,
            None,
            &RuntimeDiagnosticMetadata::default(),
            &ShellState::Resolving,
            &AppUpdateStatus::default(),
        );
        let record = AppRecord::new(42, Utc::now(), report, ShellState::Resolving).with_metadata(
            "0.4.1",
            "development",
            AppUpdateStatus::default(),
        );

        let value = serde_json::to_value(record).expect("record serializes");

        assert_eq!(value["app_version"], "0.4.1");
        assert_eq!(value["diagnostic_report"]["app_version"], "0.4.1");
    }

    #[test]
    fn should_preserve_the_observed_operating_system_process_start_time() {
        let started_at = Utc
            .with_ymd_and_hms(2026, 8, 11, 12, 0, 0)
            .single()
            .expect("fixture timestamp is valid");
        let record = AppRecord::new(
            42,
            started_at,
            report(&ShellState::Resolving),
            ShellState::Resolving,
        );

        let value = serde_json::to_value(record).expect("record serializes");
        assert_eq!(
            value["started_at"],
            serde_json::Value::String(
                started_at.to_rfc3339_opts(chrono::SecondsFormat::AutoSi, true)
            )
        );
    }

    #[cfg(unix)]
    #[test]
    fn should_report_write_failure_without_panicking() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().expect("temp directory opens");
        fs::set_permissions(directory.path(), fs::Permissions::from_mode(0o500))
            .expect("permissions update");
        let result = write_atomic(
            &directory.path().join("app.json"),
            &AppRecord::new(
                42,
                Utc::now(),
                report(&ShellState::Resolving),
                ShellState::Resolving,
            ),
        );
        fs::set_permissions(directory.path(), fs::Permissions::from_mode(0o700))
            .expect("permissions restore");
        assert!(result.is_err());
    }
}
