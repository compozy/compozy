use std::fs::{self, File, OpenOptions};
use std::io::{self, Write};
use std::path::{Path, PathBuf};

use atomicwrites::{AllowOverwrite, AtomicFile};
use chrono::{DateTime, Utc};
use fs2::FileExt;
use serde::{Deserialize, Serialize};

use crate::diagnostics::PreviousCrash;
use crate::home::CompozyHome;
use crate::runtime::discovery::{ProcessTable, SystemProcessTable};

const MARKER_FILE_NAME: &str = ".desktop-startup-marker.json";
const MARKER_LOCK_FILE_NAME: &str = ".desktop-startup-marker.lock";
const MARKER_SCHEMA_VERSION: u8 = 1;
const MAX_BOOT_ID_BYTES: usize = 128;

#[derive(Clone, Debug)]
pub struct StartupMarker {
    path: PathBuf,
    lock_path: PathBuf,
    boot_id: String,
    owner: MarkerOwner,
    previous_crash: Option<PreviousCrash>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
struct MarkerOwner {
    pid: u32,
    started_at: DateTime<Utc>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct MarkerDocument {
    schema_version: u8,
    boot_id: String,
    boot_phase: String,
    owner: MarkerOwner,
}

impl StartupMarker {
    pub fn begin(
        home: &CompozyHome,
        boot_id: String,
        process_started_at: DateTime<Utc>,
    ) -> io::Result<Self> {
        let processes = SystemProcessTable;
        Self::begin_with_owner(
            home,
            boot_id,
            MarkerOwner {
                pid: std::process::id(),
                started_at: process_started_at,
            },
            |pid| processes.start_time(pid),
        )
    }

    pub fn previous_crash(&self) -> Option<PreviousCrash> {
        self.previous_crash.clone()
    }

    pub fn clear(&self) -> io::Result<()> {
        with_marker_lock(&self.lock_path, || {
            if !self.owns_current_marker()? {
                return Ok(());
            }
            match fs::remove_file(&self.path) {
                Ok(()) => Ok(()),
                Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(()),
                Err(error) => Err(error),
            }
        })
    }

    pub fn update_phase(&self, boot_phase: &str) -> io::Result<()> {
        with_marker_lock(&self.lock_path, || {
            if !self.owns_current_marker()? {
                return Ok(());
            }
            write_private_json(
                &self.path,
                &MarkerDocument {
                    schema_version: MARKER_SCHEMA_VERSION,
                    boot_id: self.boot_id.clone(),
                    boot_phase: boot_phase.to_owned(),
                    owner: self.owner.clone(),
                },
            )
        })
    }

    fn begin_with_owner(
        home: &CompozyHome,
        boot_id: String,
        owner: MarkerOwner,
        process_start_time: impl Fn(u32) -> Option<DateTime<Utc>>,
    ) -> io::Result<Self> {
        ensure_private_directory(&home.support_bundles_dir)?;
        let path = home.support_bundles_dir.join(MARKER_FILE_NAME);
        let lock_path = home.support_bundles_dir.join(MARKER_LOCK_FILE_NAME);
        let marker_lock_path = lock_path.clone();
        with_marker_lock(&lock_path, || {
            let previous = read_marker_document(&path);
            if previous
                .as_ref()
                .is_some_and(|document| owner_is_active(&document.owner, &process_start_time))
            {
                return Err(io::Error::new(
                    io::ErrorKind::AlreadyExists,
                    "desktop startup marker belongs to an active process",
                ));
            }
            let previous_crash = read_previous_crash(&path, &process_start_time);
            write_private_json(
                &path,
                &MarkerDocument {
                    schema_version: MARKER_SCHEMA_VERSION,
                    boot_id: boot_id.clone(),
                    boot_phase: "bootstrap".to_owned(),
                    owner: owner.clone(),
                },
            )?;
            Ok(Self {
                path,
                lock_path: marker_lock_path,
                boot_id,
                owner,
                previous_crash,
            })
        })
    }

    fn owns_current_marker(&self) -> io::Result<bool> {
        match fs::read(&self.path) {
            Ok(bytes) => Ok(serde_json::from_slice::<MarkerDocument>(&bytes)
                .ok()
                .filter(marker_document_is_valid)
                .is_some_and(|document| {
                    document.boot_id == self.boot_id && document.owner == self.owner
                })),
            Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(false),
            Err(error) => Err(error),
        }
    }
}

fn with_marker_lock<T>(path: &Path, operation: impl FnOnce() -> io::Result<T>) -> io::Result<T> {
    let lock = open_marker_lock(path)?;
    FileExt::lock_exclusive(&lock)?;
    let operation_result = operation();
    let unlock_result = FileExt::unlock(&lock);
    match (operation_result, unlock_result) {
        (Ok(value), Ok(())) => Ok(value),
        (Ok(_), Err(error)) => Err(error),
        (Err(error), Ok(())) => Err(error),
        (Err(operation_error), Err(unlock_error)) => Err(io::Error::new(
            operation_error.kind(),
            format!(
                "startup marker operation and lock release failed: {operation_error}; {unlock_error}"
            ),
        )),
    }
}

fn open_marker_lock(path: &Path) -> io::Result<File> {
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() || !metadata.is_file() => {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "startup marker lock must be a regular file",
            ));
        }
        Ok(_) => {}
        Err(error) if error.kind() == io::ErrorKind::NotFound => {}
        Err(error) => return Err(error),
    }
    let file = open_private_lock_file(path)?;
    set_private_file_permissions(&file)?;
    Ok(file)
}

#[cfg(unix)]
fn open_private_lock_file(path: &Path) -> io::Result<File> {
    use std::os::unix::fs::OpenOptionsExt;

    OpenOptions::new()
        .create(true)
        .read(true)
        .write(true)
        .custom_flags(libc::O_NOFOLLOW)
        .open(path)
}

#[cfg(not(unix))]
fn open_private_lock_file(path: &Path) -> io::Result<File> {
    OpenOptions::new()
        .create(true)
        .read(true)
        .write(true)
        .open(path)
}

fn read_marker_document(path: &Path) -> Option<MarkerDocument> {
    let bytes = fs::read(path).ok()?;
    serde_json::from_slice::<MarkerDocument>(&bytes)
        .ok()
        .filter(marker_document_is_valid)
}

fn read_previous_crash(
    path: &Path,
    process_start_time: &impl Fn(u32) -> Option<DateTime<Utc>>,
) -> Option<PreviousCrash> {
    let document = read_marker_document(path)?;
    (!owner_is_active(&document.owner, process_start_time))
        .then_some(document)
        .and_then(previous_crash_from_document)
}

fn marker_document_is_valid(document: &MarkerDocument) -> bool {
    document.schema_version == MARKER_SCHEMA_VERSION
        && !document.boot_id.trim().is_empty()
        && document.boot_id.len() <= MAX_BOOT_ID_BYTES
        && document.owner.pid > 0
        && is_diagnostic_phase(&document.boot_phase)
}

fn previous_crash_from_document(document: MarkerDocument) -> Option<PreviousCrash> {
    marker_document_is_valid(&document).then_some(PreviousCrash {
        boot_id: document.boot_id,
        boot_phase: document.boot_phase,
        started_at: document.owner.started_at,
    })
}

fn owner_is_active(
    owner: &MarkerOwner,
    process_start_time: &impl Fn(u32) -> Option<DateTime<Utc>>,
) -> bool {
    process_start_time(owner.pid).is_some_and(|started_at| started_at == owner.started_at)
}

fn is_diagnostic_phase(value: &str) -> bool {
    matches!(
        value,
        "bootstrap"
            | "resolving"
            | "provisioning"
            | "starting"
            | "attaching"
            | "product"
            | "updating"
            | "disconnected"
            | "skew"
            | "error"
    )
}

fn write_private_json(path: &Path, value: &impl Serialize) -> io::Result<()> {
    AtomicFile::new(path, AllowOverwrite)
        .write(|file| -> io::Result<()> {
            set_private_file_permissions(file)?;
            serde_json::to_writer(&mut *file, value).map_err(io::Error::other)?;
            file.write_all(b"\n")?;
            Ok(())
        })
        .map_err(io::Error::from)
}

fn ensure_private_directory(path: &Path) -> io::Result<()> {
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() || !metadata.is_dir() => {
            Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "startup marker directory must be a real directory",
            ))
        }
        Ok(_) => set_private_directory_permissions(path),
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            let parent = path.parent().ok_or_else(|| {
                io::Error::new(
                    io::ErrorKind::InvalidInput,
                    "startup marker directory has no parent",
                )
            })?;
            ensure_private_parent_directory(parent)?;
            match fs::create_dir(path) {
                Ok(()) => set_private_directory_permissions(path),
                Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {
                    ensure_private_directory(path)
                }
                Err(error) => Err(error),
            }
        }
        Err(error) => Err(error),
    }
}

fn ensure_private_parent_directory(path: &Path) -> io::Result<()> {
    match fs::symlink_metadata(path) {
        Ok(metadata) if metadata.file_type().is_symlink() || !metadata.is_dir() => {
            Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "startup marker parent must be a real directory",
            ))
        }
        Ok(_) => Ok(()),
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            let parent = path.parent().ok_or_else(|| {
                io::Error::new(
                    io::ErrorKind::InvalidInput,
                    "startup marker parent has no parent",
                )
            })?;
            ensure_private_parent_directory(parent)?;
            match fs::create_dir(path) {
                Ok(()) => set_private_directory_permissions(path),
                Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {
                    ensure_private_parent_directory(path)
                }
                Err(error) => Err(error),
            }
        }
        Err(error) => Err(error),
    }
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

#[cfg(unix)]
fn set_private_file_permissions(file: &File) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    file.set_permissions(fs::Permissions::from_mode(0o600))
}

#[cfg(not(unix))]
fn set_private_file_permissions(_file: &File) -> io::Result<()> {
    Ok(())
}

#[cfg(test)]
#[path = "startup_marker_tests.rs"]
mod tests;
