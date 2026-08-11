use std::fs::{self, OpenOptions};
use std::io::{self, Write};
use std::path::PathBuf;

use chrono::Utc;
use serde_json::json;
use tauri_plugin_log::{Target, TargetKind};

use crate::errors::redact_public_text;

pub const BOOTSTRAP_LOG_FILE_NAME: &str = "desktop-bootstrap.jsonl";

pub fn plugin(logs_dir: PathBuf, boot_id: String) -> tauri::plugin::TauriPlugin<tauri::Wry> {
    tauri_plugin_log::Builder::new()
        .targets([Target::new(TargetKind::Folder {
            path: logs_dir,
            file_name: Some("desktop".to_owned()),
        })])
        .format(move |out, message, record| {
            let entry = json!({
                "timestamp": Utc::now().to_rfc3339(),
                "level": record.level().to_string().to_lowercase(),
                "target": record.target(),
                "boot_id": boot_id,
                "message": redact_public_text(&message.to_string()),
            });
            out.finish(format_args!("{entry}"))
        })
        .level(log::LevelFilter::Info)
        .max_file_size(10 * 1024 * 1024)
        .build()
}

pub fn error(message: impl AsRef<str>) {
    log::error!("{}", redact_public_text(message.as_ref()));
}

pub fn bootstrap_error(
    logs_dir: &std::path::Path,
    boot_id: &str,
    context: &str,
    error: impl AsRef<str>,
) -> io::Result<()> {
    fs::create_dir_all(logs_dir)?;
    let path = logs_dir.join(BOOTSTRAP_LOG_FILE_NAME);
    let mut file = OpenOptions::new().create(true).append(true).open(&path)?;
    set_private_file_permissions(&path)?;
    let entry = json!({
        "timestamp": Utc::now().to_rfc3339(),
        "level": "error",
        "boot_id": redact_public_text(boot_id),
        "context": redact_public_text(context),
        "message": redact_public_text(error.as_ref()),
    });
    serde_json::to_writer(&mut file, &entry).map_err(io::Error::other)?;
    file.write_all(b"\n")?;
    file.sync_all()
}

#[cfg(unix)]
fn set_private_file_permissions(path: &std::path::Path) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
}

#[cfg(not(unix))]
fn set_private_file_permissions(_path: &std::path::Path) -> io::Result<()> {
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::*;

    #[test]
    fn should_write_a_redacted_correlated_bootstrap_jsonl_event() {
        let directory = tempfile::tempdir().expect("temp directory opens");

        bootstrap_error(
            directory.path(),
            "boot-1",
            "initialize logging",
            "refresh_token=do-not-share",
        )
        .expect("bootstrap event writes");

        let contents = fs::read_to_string(directory.path().join(BOOTSTRAP_LOG_FILE_NAME))
            .expect("bootstrap log reads");
        let value: serde_json::Value = serde_json::from_str(contents.trim()).expect("JSONL parses");
        assert_eq!(value["boot_id"], "boot-1");
        assert_eq!(value["level"], "error");
        assert!(!contents.contains("do-not-share"));
        assert!(contents.ends_with('\n'));
    }
}
