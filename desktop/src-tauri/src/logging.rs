use std::path::PathBuf;

use tauri_plugin_log::{Target, TargetKind};

use crate::errors::redact_public_text;

pub fn plugin(logs_dir: PathBuf) -> tauri::plugin::TauriPlugin<tauri::Wry> {
    tauri_plugin_log::Builder::new()
        .targets([Target::new(TargetKind::Folder {
            path: logs_dir,
            file_name: Some("desktop".to_owned()),
        })])
        .level(log::LevelFilter::Info)
        .max_file_size(10 * 1024 * 1024)
        .build()
}

pub fn error(message: impl AsRef<str>) {
    log::error!("{}", redact_public_text(message.as_ref()));
}
