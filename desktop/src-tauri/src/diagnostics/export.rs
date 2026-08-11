use std::fs;
use std::io;
use std::path::{Component, Path, PathBuf};

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use thiserror::Error;

use super::export_archive::write_bundle;
use crate::diagnostics::DiagnosticReport;
use crate::home::CompozyHome;

pub const DIAGNOSTIC_MANIFEST_SCHEMA_VERSION: u8 = 1;
pub const MAX_DIAGNOSTIC_BUNDLE_BYTES: u64 = 64 * 1024;
pub const MANIFEST_FILE_NAME: &str = "manifest.json";
pub const DESKTOP_LOG_ARCHIVE_ENTRY: &str = "desktop.log";
pub const BOOTSTRAP_LOG_ARCHIVE_ENTRY: &str = "desktop-bootstrap.jsonl";

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DiagnosticExportManifest {
    pub schema_version: u8,
    pub kind: String,
    pub generated_at: DateTime<Utc>,
    pub report: DiagnosticReport,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DiagnosticExport {
    pub bundle_path: PathBuf,
    pub bytes: u64,
}

#[derive(Debug, Error)]
pub enum DiagnosticExportError {
    #[error("diagnostic export requires explicit consent")]
    ConsentRequired,
    #[error("diagnostic export path is invalid")]
    InvalidPath,
    #[error("diagnostic export path already exists")]
    AlreadyExists,
    #[error("diagnostic export could not be created")]
    Create(#[source] io::Error),
    #[error("diagnostic export exceeds its size limit")]
    TooLarge,
}

pub fn export(
    home: &CompozyHome,
    report: &DiagnosticReport,
    consent: bool,
    output_path: Option<&Path>,
) -> Result<DiagnosticExport, DiagnosticExportError> {
    if !consent {
        return Err(DiagnosticExportError::ConsentRequired);
    }
    let path = resolve_output_path(home, output_path)?;
    prepare_output_directory(home, &path)?;
    reject_existing_destination(&path)?;
    let manifest = DiagnosticExportManifest {
        schema_version: DIAGNOSTIC_MANIFEST_SCHEMA_VERSION,
        kind: "compozyos_desktop_diagnostics".to_owned(),
        generated_at: Utc::now(),
        report: report.clone(),
    };
    let manifest_bytes = serde_json::to_vec_pretty(&manifest)
        .map_err(io::Error::other)
        .map_err(DiagnosticExportError::Create)?;
    write_bundle(&path, home, report, manifest_bytes)?;
    let bytes = fs::metadata(&path)
        .map_err(DiagnosticExportError::Create)?
        .len();
    if bytes > MAX_DIAGNOSTIC_BUNDLE_BYTES {
        remove_created_bundle(&path)?;
        return Err(DiagnosticExportError::TooLarge);
    }
    Ok(DiagnosticExport {
        bundle_path: path,
        bytes,
    })
}

pub fn default_bundle_path(home: &CompozyHome, at: DateTime<Utc>) -> PathBuf {
    home.support_bundles_dir.join(format!(
        "compozyos-desktop-diagnostics-{}.tar.gz",
        at.format("%Y%m%dT%H%M%S%3fZ")
    ))
}

fn resolve_output_path(
    home: &CompozyHome,
    output_path: Option<&Path>,
) -> Result<PathBuf, DiagnosticExportError> {
    let path = match output_path {
        Some(path) if path.as_os_str().is_empty() => {
            return Err(DiagnosticExportError::InvalidPath);
        }
        Some(path) => path.to_path_buf(),
        None => default_bundle_path(home, Utc::now()),
    };
    if path.file_name().is_none() || path.extension().and_then(|value| value.to_str()) != Some("gz")
    {
        return Err(DiagnosticExportError::InvalidPath);
    }
    if output_parent(&path)? != home.support_bundles_dir {
        return Err(DiagnosticExportError::InvalidPath);
    }
    Ok(path)
}

fn prepare_output_directory(home: &CompozyHome, path: &Path) -> Result<(), DiagnosticExportError> {
    let parent = output_parent(path)?;
    ensure_no_untrusted_ancestor_symlink(parent)?;
    if parent != home.support_bundles_dir {
        return Err(DiagnosticExportError::InvalidPath);
    }
    ensure_private_default_directory(parent)
}

fn ensure_no_untrusted_ancestor_symlink(path: &Path) -> Result<(), DiagnosticExportError> {
    let mut current = PathBuf::new();
    let mut missing_ancestor = false;
    for component in path.components() {
        match component {
            Component::Prefix(prefix) => current.push(prefix.as_os_str()),
            Component::RootDir => current.push(Path::new(std::path::MAIN_SEPARATOR_STR)),
            Component::CurDir => {
                if current.as_os_str().is_empty() {
                    current.push(Path::new("."));
                }
            }
            Component::ParentDir => return Err(DiagnosticExportError::InvalidPath),
            Component::Normal(component) => {
                if current.as_os_str().is_empty() {
                    current.push(Path::new("."));
                }
                current.push(component);
                if missing_ancestor {
                    continue;
                }
                match fs::symlink_metadata(&current) {
                    Ok(metadata) if metadata.file_type().is_symlink() => {
                        if !is_allowed_system_root_redirect(&current) {
                            return Err(DiagnosticExportError::InvalidPath);
                        }
                        current =
                            fs::canonicalize(&current).map_err(DiagnosticExportError::Create)?;
                    }
                    Ok(metadata) if !metadata.is_dir() => {
                        return Err(DiagnosticExportError::InvalidPath);
                    }
                    Ok(_) => {}
                    Err(error) if error.kind() == io::ErrorKind::NotFound => {
                        missing_ancestor = true;
                    }
                    Err(error) => return Err(DiagnosticExportError::Create(error)),
                }
            }
        }
    }
    Ok(())
}

#[cfg(target_os = "macos")]
fn is_allowed_system_root_redirect(path: &Path) -> bool {
    matches!(path.to_str(), Some("/var" | "/tmp" | "/etc"))
}

#[cfg(not(target_os = "macos"))]
fn is_allowed_system_root_redirect(_path: &Path) -> bool {
    false
}

fn output_parent(path: &Path) -> Result<&Path, DiagnosticExportError> {
    match path.parent() {
        Some(parent) if parent.as_os_str().is_empty() => Ok(Path::new(".")),
        Some(parent) => Ok(parent),
        None => Err(DiagnosticExportError::InvalidPath),
    }
}

fn ensure_private_default_directory(path: &Path) -> Result<(), DiagnosticExportError> {
    ensure_private_directory_exists(path)?;
    set_private_directory_permissions(path).map_err(DiagnosticExportError::Create)
}

fn ensure_custom_parent_directory(path: &Path) -> Result<(), DiagnosticExportError> {
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() || !metadata.is_dir() => {
            Err(DiagnosticExportError::InvalidPath)
        }
        Ok(_) => Ok(()),
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            ensure_private_directory_exists(path)
        }
        Err(error) => Err(DiagnosticExportError::Create(error)),
    }
}

fn ensure_private_directory_exists(path: &Path) -> Result<(), DiagnosticExportError> {
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() || !metadata.is_dir() => {
            Err(DiagnosticExportError::InvalidPath)
        }
        Ok(_) => Ok(()),
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            let parent = output_parent(path)?;
            if parent != path {
                ensure_custom_parent_directory(parent)?;
            }
            match fs::create_dir(path) {
                Ok(()) => {
                    set_private_directory_permissions(path).map_err(DiagnosticExportError::Create)
                }
                Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {
                    match fs::symlink_metadata(path) {
                        Ok(metadata) if !metadata.file_type().is_symlink() && metadata.is_dir() => {
                            Ok(())
                        }
                        Ok(_) => Err(DiagnosticExportError::InvalidPath),
                        Err(error) => Err(DiagnosticExportError::Create(error)),
                    }
                }
                Err(error) => Err(DiagnosticExportError::Create(error)),
            }
        }
        Err(error) => Err(DiagnosticExportError::Create(error)),
    }
}

fn reject_existing_destination(path: &Path) -> Result<(), DiagnosticExportError> {
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() => {
            Err(DiagnosticExportError::InvalidPath)
        }
        Ok(_) => Err(DiagnosticExportError::AlreadyExists),
        Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(DiagnosticExportError::Create(error)),
    }
}

fn remove_created_bundle(path: &Path) -> Result<(), DiagnosticExportError> {
    fs::remove_file(path).map_err(DiagnosticExportError::Create)
}

#[cfg(unix)]
fn set_private_directory_permissions(path: &Path) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    fs::set_permissions(path, fs::Permissions::from_mode(0o700))
}

#[cfg(not(unix))]
fn set_private_directory_permissions(_path: &Path) -> io::Result<()> {
    Ok(())
}

#[cfg(test)]
#[path = "export_tests.rs"]
mod tests;
