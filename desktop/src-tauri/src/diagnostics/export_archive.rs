use std::fs::{self, File, OpenOptions};
use std::io::{self, Read, Seek, SeekFrom, Write};
use std::path::Path;

use atomicwrites::{AtomicFile, DisallowOverwrite};
use chrono::{DateTime, Utc};
use flate2::Compression;
use flate2::write::GzEncoder;
use serde_json::{Value, json};
use tar::{Builder, Header};

use super::export::{
    BOOTSTRAP_LOG_ARCHIVE_ENTRY, DESKTOP_LOG_ARCHIVE_ENTRY, DiagnosticExportError,
    MANIFEST_FILE_NAME,
};
use crate::diagnostics::DiagnosticReport;
use crate::errors::sanitize_public_text;
use crate::home::CompozyHome;
use crate::logging::BOOTSTRAP_LOG_FILE_NAME;

const MAX_LOG_TAIL_BYTES: usize = 16 * 1024;
const MAX_BUNDLE_CONTENT_BYTES: usize = 48 * 1024;
const MAX_LOG_MESSAGE_BYTES: usize = 512;

pub(super) fn write_bundle(
    path: &Path,
    home: &CompozyHome,
    report: &DiagnosticReport,
    manifest: Vec<u8>,
) -> Result<(), DiagnosticExportError> {
    let entries = bundle_entries(home, report, manifest)?;
    AtomicFile::new(path, DisallowOverwrite)
        .write(|file| -> io::Result<()> {
            set_private_file_permissions(file)?;
            let encoder = GzEncoder::new(file, Compression::default());
            let mut archive = Builder::new(encoder);
            for entry in &entries {
                append_entry(&mut archive, entry)?;
            }
            archive.into_inner()?.finish()?;
            Ok(())
        })
        .map_err(io::Error::from)
        .map_err(map_write_error)
}

fn bundle_entries(
    home: &CompozyHome,
    report: &DiagnosticReport,
    manifest: Vec<u8>,
) -> Result<Vec<ArchiveEntry>, DiagnosticExportError> {
    let mut total_bytes = manifest.len();
    if total_bytes > MAX_BUNDLE_CONTENT_BYTES {
        return Err(DiagnosticExportError::TooLarge);
    }
    let mut entries = vec![ArchiveEntry::new(MANIFEST_FILE_NAME, manifest)];
    let bootstrap_log = home.logs_dir.join(BOOTSTRAP_LOG_FILE_NAME);
    for (entry_name, source) in [
        (DESKTOP_LOG_ARCHIVE_ENTRY, home.app_log.as_path()),
        (BOOTSTRAP_LOG_ARCHIVE_ENTRY, bootstrap_log.as_path()),
    ] {
        match redacted_log_tail(source, report, home) {
            Ok(Some(bytes)) if total_bytes + bytes.len() <= MAX_BUNDLE_CONTENT_BYTES => {
                total_bytes += bytes.len();
                entries.push(ArchiveEntry::new(entry_name, bytes));
            }
            Ok(Some(_)) | Ok(None) => {}
            Err(error) => crate::logging::error(format!("read diagnostic log tail: {error}")),
        }
    }
    Ok(entries)
}

fn redacted_log_tail(
    path: &Path,
    report: &DiagnosticReport,
    home: &CompozyHome,
) -> io::Result<Option<Vec<u8>>> {
    let Some(bytes) = read_log_tail(path)? else {
        return Ok(None);
    };
    let start = if bytes.len() == MAX_LOG_TAIL_BYTES {
        bytes
            .iter()
            .position(|byte| *byte == b'\n')
            .map_or(bytes.len(), |index| index + 1)
    } else {
        0
    };
    let text = match std::str::from_utf8(&bytes[start..]) {
        Ok(text) => text,
        Err(_) => return Ok(None),
    };
    let mut output = Vec::new();
    for line in text.lines() {
        let Some(line) = redact_log_line(line, report, home) else {
            continue;
        };
        if output.len() + line.len() + 1 > MAX_LOG_TAIL_BYTES {
            break;
        }
        output.extend_from_slice(line.as_bytes());
        output.push(b'\n');
    }
    Ok((!output.is_empty()).then_some(output))
}

fn read_log_tail(path: &Path) -> io::Result<Option<Vec<u8>>> {
    let metadata = match fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error),
    };
    if metadata.file_type().is_symlink() || !metadata.is_file() {
        return Ok(None);
    }
    let mut file = open_regular_file(path)?;
    let length = file.metadata()?.len();
    if length > MAX_LOG_TAIL_BYTES as u64 {
        file.seek(SeekFrom::Start(length - MAX_LOG_TAIL_BYTES as u64))?;
    }
    let mut bytes = Vec::with_capacity(MAX_LOG_TAIL_BYTES);
    file.take(MAX_LOG_TAIL_BYTES as u64)
        .read_to_end(&mut bytes)?;
    Ok(Some(bytes))
}

#[cfg(unix)]
fn open_regular_file(path: &Path) -> io::Result<File> {
    use std::os::unix::fs::OpenOptionsExt;

    OpenOptions::new()
        .read(true)
        .custom_flags(libc::O_NOFOLLOW)
        .open(path)
}

#[cfg(not(unix))]
fn open_regular_file(path: &Path) -> io::Result<File> {
    OpenOptions::new().read(true).open(path)
}

fn redact_log_line(line: &str, report: &DiagnosticReport, home: &CompozyHome) -> Option<String> {
    let value = serde_json::from_str::<Value>(line).ok()?;
    if value.get("boot_id")?.as_str()? != report.boot_id.as_str() {
        return None;
    }
    let timestamp = DateTime::parse_from_rfc3339(value.get("timestamp")?.as_str()?)
        .ok()?
        .with_timezone(&Utc)
        .to_rfc3339();
    let level = value.get("level")?.as_str()?.to_ascii_lowercase();
    if !matches!(level.as_str(), "error" | "warn" | "info" | "debug") {
        return None;
    }
    let message = value.get("message")?.as_str()?;
    if contains_non_shareable_content(message) {
        return None;
    }
    let message = redact_log_message(message, home);
    if message.is_empty() {
        return None;
    }
    serde_json::to_string(&json!({
        "timestamp": timestamp,
        "level": level,
        "boot_id": report.boot_id,
        "message": message,
    }))
    .ok()
}

fn contains_non_shareable_content(message: &str) -> bool {
    let message = message.to_ascii_lowercase();
    [
        "config",
        "session",
        "transcript",
        "credential",
        "password",
        "secret",
        "token",
        "authorization",
        "compozy.db",
        "sqlite",
    ]
    .iter()
    .any(|term| message.contains(term))
}

fn redact_log_message(message: &str, home: &CompozyHome) -> String {
    let mut redacted = message.to_owned();
    for path in [
        home.root.as_path(),
        home.logs_dir.as_path(),
        home.app_log.as_path(),
    ] {
        let path = path.to_string_lossy();
        if !path.is_empty() && path != "/" {
            redacted = redacted.replace(path.as_ref(), "[redacted]");
        }
    }
    if let Some(user_home) = dirs::home_dir() {
        let user_home = user_home.to_string_lossy();
        if !user_home.is_empty() && user_home != "/" {
            redacted = redacted.replace(user_home.as_ref(), "[redacted]");
        }
    }
    truncate_utf8(&sanitize_public_text(&redacted), MAX_LOG_MESSAGE_BYTES)
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

fn append_entry<W: Write>(archive: &mut Builder<W>, entry: &ArchiveEntry) -> io::Result<()> {
    let mut header = Header::new_gnu();
    header.set_size(entry.bytes.len() as u64);
    header.set_mode(0o600);
    header.set_cksum();
    archive.append_data(&mut header, entry.name, entry.bytes.as_slice())
}

fn map_write_error(error: io::Error) -> DiagnosticExportError {
    if error.kind() == io::ErrorKind::AlreadyExists {
        DiagnosticExportError::AlreadyExists
    } else {
        DiagnosticExportError::Create(error)
    }
}

struct ArchiveEntry {
    name: &'static str,
    bytes: Vec<u8>,
}

impl ArchiveEntry {
    fn new(name: &'static str, bytes: Vec<u8>) -> Self {
        Self { name, bytes }
    }
}

#[cfg(unix)]
fn set_private_file_permissions(file: &File) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    file.set_permissions(fs::Permissions::from_mode(0o600))
}

#[cfg(not(unix))]
fn set_private_file_permissions(_file: &File) -> io::Result<()> {
    Ok(())
}
