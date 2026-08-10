use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::durability::sync_directory;
use crate::home::CompozyHome;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ProvenanceRecord {
    pub installed_by: String,
    pub app_version: String,
    pub runtime_version: String,
    pub channel: String,
    pub installed_at: DateTime<Utc>,
    pub binary_sha256: String,
}

impl ProvenanceRecord {
    pub fn desktop(
        app_version: impl Into<String>,
        runtime_version: impl Into<String>,
        channel: impl Into<String>,
        installed_at: DateTime<Utc>,
        binary_sha256: impl Into<String>,
    ) -> Self {
        Self {
            installed_by: "desktop-app".to_owned(),
            app_version: app_version.into(),
            runtime_version: runtime_version.into(),
            channel: channel.into(),
            installed_at,
            binary_sha256: binary_sha256.into(),
        }
    }
}

pub fn validated_owned_binary(home: &CompozyHome) -> Option<PathBuf> {
    let marker: ProvenanceRecord =
        serde_json::from_slice(&fs::read(&home.provenance).ok()?).ok()?;
    if marker.installed_by != "desktop-app" {
        return None;
    }
    let binary = executable_file(home.app_binary.clone())?;
    if !same_path(&binary, &home.app_binary) {
        return None;
    }
    let digest = sha256_file(&binary).ok()?;
    digest
        .eq_ignore_ascii_case(&marker.binary_sha256)
        .then_some(binary)
}

pub fn owns_executable(home: &CompozyHome, executable: &Path) -> bool {
    validated_owned_binary(home).is_some_and(|owned| same_path(&owned, executable))
}

pub fn write_atomic(path: &Path, record: &ProvenanceRecord) -> std::io::Result<()> {
    let parent = path.parent().ok_or_else(|| {
        std::io::Error::new(std::io::ErrorKind::InvalidInput, "marker has no parent")
    })?;
    fs::create_dir_all(parent)?;
    let temporary = parent.join(".desktop-provenance.json.tmp");
    let mut file = File::create(&temporary)?;
    serde_json::to_writer(&mut file, record).map_err(std::io::Error::other)?;
    file.write_all(b"\n")?;
    file.sync_all()?;
    fs::rename(&temporary, path)?;
    sync_directory(parent)?;
    Ok(())
}

pub fn sha256_file(path: &Path) -> std::io::Result<String> {
    let mut file = File::open(path)?;
    let mut hasher = Sha256::new();
    let mut buffer = [0_u8; 64 * 1024];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    Ok(format!("{:x}", hasher.finalize()))
}

fn same_path(left: &Path, right: &Path) -> bool {
    fs::canonicalize(left)
        .ok()
        .zip(fs::canonicalize(right).ok())
        .is_some_and(|(left, right)| left == right)
}

pub(crate) fn executable_file(path: PathBuf) -> Option<PathBuf> {
    if !path.is_file() {
        return None;
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;

        let executable = fs::metadata(&path)
            .ok()
            .is_some_and(|metadata| metadata.permissions().mode() & 0o111 != 0);
        executable.then_some(path)
    }
    #[cfg(not(unix))]
    {
        Some(path)
    }
}
