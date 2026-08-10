use std::fs;
use std::io::{self, Write};
use std::path::Path;

use atomicwrites::{AllowOverwrite, AtomicFile};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::errors::ShellError;
use crate::state::ShellState;

pub const APP_STATE_SCHEMA_VERSION: u8 = 1;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct AppRecord {
    pub schema_version: u8,
    pub pid: u32,
    pub started_at: DateTime<Utc>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub diagnostic: Option<ShellError>,
    #[serde(flatten)]
    pub shell: ShellState,
}

impl AppRecord {
    pub fn new(pid: u32, started_at: DateTime<Utc>, shell: ShellState) -> Self {
        Self {
            schema_version: APP_STATE_SCHEMA_VERSION,
            pid,
            started_at,
            diagnostic: None,
            shell,
        }
    }

    pub fn with_diagnostic(mut self, diagnostic: Option<ShellError>) -> Self {
        self.diagnostic = diagnostic;
        self
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
    use std::path::PathBuf;
    use std::sync::Arc;
    use std::sync::atomic::{AtomicBool, Ordering};

    use crate::errors::{ShellError, ShellErrorCode};
    use crate::state::{ProvisionStage, UpdateTarget};

    use super::*;

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
            write_atomic(&path, &AppRecord::new(42, Utc::now(), shell))
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
            let value = serde_json::to_value(AppRecord::new(42, Utc::now(), state))
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

    #[cfg(unix)]
    #[test]
    fn should_report_write_failure_without_panicking() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().expect("temp directory opens");
        fs::set_permissions(directory.path(), fs::Permissions::from_mode(0o500))
            .expect("permissions update");
        let result = write_atomic(
            &directory.path().join("app.json"),
            &AppRecord::new(42, Utc::now(), ShellState::Resolving),
        );
        fs::set_permissions(directory.path(), fs::Permissions::from_mode(0o700))
            .expect("permissions restore");
        assert!(result.is_err());
    }
}
