use std::fs::{self, File, OpenOptions};
use std::io::Read;
use std::path::{Path, PathBuf};
use std::time::Duration;

use chrono::Utc;
use flate2::read::GzDecoder;
use semver::Version;
use thiserror::Error;
use url::Url;
use zip::ZipArchive;

use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::state::ProvisionStage;

use super::artifacts::{
    ArtifactError, VerifiedArtifact, download_verified, fetch_signed_manifest, http_client,
    verify_manifest,
};
use super::provenance::{self, ProvenanceRecord};

const DOWNLOAD_NAME: &str = ".runtime-download";
const STAGED_NAME: &str = ".runtime-staged";
const MAX_RUNTIME_BINARY_BYTES: u64 = 256 * 1024 * 1024;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ResumeChoice {
    Continue,
    StartOver,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProvisionOutcome {
    Installed { version: Version },
    AttachExternally,
    UpToDate,
}

#[derive(Clone, Debug)]
pub struct StagedRuntime {
    pub verified: VerifiedArtifact,
    pub binary_path: PathBuf,
}

#[derive(Debug, Error)]
pub enum ProvisionError {
    #[error(transparent)]
    Artifact(#[from] ArtifactError),
    #[error("runtime archive is malformed")]
    Archive,
    #[error("runtime staging needs {required} bytes of free space")]
    DiskSpace { required: u64 },
    #[error("runtime staging path is not writable: {path}")]
    Permission { path: PathBuf },
    #[error("runtime provisioning could not access {path}")]
    Io {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },
}

pub struct ProvisionRequest<'a> {
    pub home: &'a CompozyHome,
    pub manifest_url: &'a Url,
    pub public_key: &'a str,
    pub target: &'a str,
    pub installed: Option<&'a Version>,
    pub app_version: &'a str,
    pub channel: &'a str,
    pub resume: ResumeChoice,
}

pub fn provision(
    request: &ProvisionRequest<'_>,
    runtime_present: &dyn Fn() -> bool,
    progress: &mut dyn FnMut(ProvisionStage),
) -> Result<ProvisionOutcome, ProvisionError> {
    if runtime_present() {
        return Ok(ProvisionOutcome::AttachExternally);
    }
    let client = http_client(Duration::from_secs(30))?;
    let paths = staging_paths(request.home);
    if request.resume == ResumeChoice::StartOver {
        remove_staging(&paths)?;
    }
    progress(ProvisionStage::Download { pct: 0 });
    let (manifest, signature) = fetch_signed_manifest(&client, request.manifest_url)?;
    progress(ProvisionStage::Verify);
    let Some(verified) = verify_manifest(
        &manifest,
        &signature,
        request.public_key,
        request.target,
        request.installed,
    )?
    else {
        return Ok(ProvisionOutcome::UpToDate);
    };
    if !verified_download_exists(&paths.download, &verified.artifact) {
        download_verified(&client, &verified.artifact, &paths.download)
            .map_err(map_artifact_download)?;
    }
    extract_binary(&paths.download, &paths.staged_binary)?;
    if runtime_present() {
        remove_staging(&paths)?;
        return Ok(ProvisionOutcome::AttachExternally);
    }
    progress(ProvisionStage::Install);
    install_staged(
        request.home,
        &paths.staged_binary,
        &verified.version,
        request.app_version,
        request.channel,
    )?;
    remove_file_if_present(&paths.download)?;
    progress(ProvisionStage::Start);
    Ok(ProvisionOutcome::Installed {
        version: verified.version,
    })
}

pub fn stage_update(
    home: &CompozyHome,
    manifest_url: &Url,
    public_key: &str,
    target: &str,
    installed: &Version,
) -> Result<Option<StagedRuntime>, ProvisionError> {
    let client = http_client(Duration::from_secs(30))?;
    let (manifest, signature) = fetch_signed_manifest(&client, manifest_url)?;
    let Some(verified) =
        verify_manifest(&manifest, &signature, public_key, target, Some(installed))?
    else {
        return Ok(None);
    };
    let paths = staging_paths(home);
    remove_staging(&paths)?;
    download_verified(&client, &verified.artifact, &paths.download)
        .map_err(map_artifact_download)?;
    extract_binary(&paths.download, &paths.staged_binary)?;
    remove_file_if_present(&paths.download)?;
    Ok(Some(StagedRuntime {
        verified,
        binary_path: paths.staged_binary,
    }))
}

pub fn install_staged(
    home: &CompozyHome,
    staged_binary: &Path,
    runtime_version: &Version,
    app_version: &str,
    channel: &str,
) -> Result<(), ProvisionError> {
    let parent = home
        .app_binary
        .parent()
        .ok_or_else(|| ProvisionError::Permission {
            path: home.app_binary.clone(),
        })?;
    fs::create_dir_all(parent).map_err(|error| map_io(parent, error, 0))?;
    File::open(staged_binary)
        .and_then(|file| file.sync_all())
        .map_err(|error| map_io(staged_binary, error, 0))?;
    fs::rename(staged_binary, &home.app_binary)
        .map_err(|error| map_io(&home.app_binary, error, 0))?;
    File::open(parent)
        .and_then(|directory| directory.sync_all())
        .map_err(|error| map_io(parent, error, 0))?;
    let digest = provenance::sha256_file(&home.app_binary)
        .map_err(|error| map_io(&home.app_binary, error, 0))?;
    let marker = ProvenanceRecord::desktop(
        app_version,
        runtime_version.to_string(),
        channel,
        Utc::now(),
        digest,
    );
    provenance::write_atomic(&home.provenance, &marker)
        .map_err(|error| map_io(&home.provenance, error, 0))
}

pub fn incomplete_stage(home: &CompozyHome) -> bool {
    let paths = staging_paths(home);
    paths.download.exists() || paths.staged_binary.exists()
}

pub fn shell_error(home: &CompozyHome, error: &ProvisionError) -> ShellError {
    match error {
        ProvisionError::Artifact(ArtifactError::Network) => {
            ShellError::from_code(ShellErrorCode::ProvisionNetwork, home.app_log.clone())
        }
        ProvisionError::DiskSpace { required } => ShellError::new(
            ShellErrorCode::ProvisionDiskSpace,
            format!("The runtime needs {required} bytes of free space."),
            home.app_log.clone(),
        ),
        ProvisionError::Permission { path } => ShellError::new(
            ShellErrorCode::ProvisionPermission,
            format!("The runtime cannot be written to {}.", path.display()),
            home.app_log.clone(),
        ),
        _ => ShellError::new(
            ShellErrorCode::RuntimeStartFailed,
            "The runtime package could not be verified or installed.",
            home.app_log.clone(),
        ),
    }
}

struct StagingPaths {
    download: PathBuf,
    staged_binary: PathBuf,
}

fn staging_paths(home: &CompozyHome) -> StagingPaths {
    let bin = home.app_binary.parent().unwrap_or(home.root.as_path());
    StagingPaths {
        download: bin.join(DOWNLOAD_NAME),
        staged_binary: bin.join(STAGED_NAME),
    }
}

fn extract_binary(archive_path: &Path, destination: &Path) -> Result<(), ProvisionError> {
    if cfg!(windows) {
        extract_zip(archive_path, destination)
    } else {
        extract_tar_gz(archive_path, destination)
    }
}

fn extract_tar_gz(archive_path: &Path, destination: &Path) -> Result<(), ProvisionError> {
    let file = File::open(archive_path).map_err(|error| map_io(archive_path, error, 0))?;
    let mut archive = tar::Archive::new(GzDecoder::new(file));
    let entries = archive.entries().map_err(|_| ProvisionError::Archive)?;
    for entry in entries {
        let mut entry = entry.map_err(|_| ProvisionError::Archive)?;
        let path = entry.path().map_err(|_| ProvisionError::Archive)?;
        if path.file_name().is_some_and(|name| name == "compozy")
            && entry.header().entry_type().is_file()
        {
            let size = entry.size();
            write_executable(&mut entry, destination, size)?;
            return Ok(());
        }
    }
    Err(ProvisionError::Archive)
}

fn extract_zip(archive_path: &Path, destination: &Path) -> Result<(), ProvisionError> {
    let file = File::open(archive_path).map_err(|error| map_io(archive_path, error, 0))?;
    let mut archive = ZipArchive::new(file).map_err(|_| ProvisionError::Archive)?;
    for index in 0..archive.len() {
        let mut entry = archive
            .by_index(index)
            .map_err(|_| ProvisionError::Archive)?;
        let path = Path::new(entry.name());
        if path.file_name().is_some_and(|name| name == "compozy.exe") && !entry.is_dir() {
            let size = entry.size();
            write_executable(&mut entry, destination, size)?;
            return Ok(());
        }
    }
    Err(ProvisionError::Archive)
}

fn write_executable(
    reader: &mut dyn Read,
    destination: &Path,
    size: u64,
) -> Result<(), ProvisionError> {
    if size == 0 || size > MAX_RUNTIME_BINARY_BYTES {
        return Err(ProvisionError::Archive);
    }
    if let Some(parent) = destination.parent() {
        fs::create_dir_all(parent).map_err(|error| map_io(parent, error, size))?;
    }
    let mut file = OpenOptions::new()
        .create(true)
        .truncate(true)
        .write(true)
        .open(destination)
        .map_err(|error| map_io(destination, error, size))?;
    let copied = std::io::copy(&mut reader.take(size + 1), &mut file)
        .map_err(|error| map_io(destination, error, size))?;
    if copied != size {
        return Err(ProvisionError::Archive);
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;

        file.set_permissions(fs::Permissions::from_mode(0o755))
            .map_err(|error| map_io(destination, error, size))?;
    }
    file.sync_all()
        .map_err(|error| map_io(destination, error, size))
}

fn remove_staging(paths: &StagingPaths) -> Result<(), ProvisionError> {
    remove_file_if_present(&paths.download)?;
    remove_file_if_present(&paths.staged_binary)
}

fn remove_file_if_present(path: &Path) -> Result<(), ProvisionError> {
    match fs::remove_file(path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(map_io(path, error, 0)),
    }
}

fn verified_download_exists(path: &Path, artifact: &super::artifacts::PlatformArtifact) -> bool {
    let Ok(metadata) = fs::metadata(path) else {
        return false;
    };
    if metadata.len() != artifact.size {
        return false;
    }
    provenance::sha256_file(path).is_ok_and(|digest| digest.eq_ignore_ascii_case(&artifact.sha256))
}

fn map_artifact_download(error: ArtifactError) -> ProvisionError {
    ProvisionError::Artifact(error)
}

fn map_io(path: &Path, error: std::io::Error, required: u64) -> ProvisionError {
    let disk_full = error.raw_os_error() == Some(libc::ENOSPC)
        || (cfg!(windows) && error.raw_os_error() == Some(112));
    if disk_full {
        return ProvisionError::DiskSpace { required };
    }
    if error.kind() == std::io::ErrorKind::PermissionDenied {
        return ProvisionError::Permission {
            path: path.to_path_buf(),
        };
    }
    ProvisionError::Io {
        path: path.to_path_buf(),
        source: error,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_classify_disk_space_and_permission_failures_with_actionable_paths() {
        let path = PathBuf::from("/tmp/compozy/bin");
        assert!(matches!(
            map_io(&path, std::io::Error::from_raw_os_error(libc::ENOSPC), 4096,),
            ProvisionError::DiskSpace { required: 4096 }
        ));
        assert!(matches!(
            map_io(
                &path,
                std::io::Error::new(std::io::ErrorKind::PermissionDenied, "fixture"),
                0,
            ),
            ProvisionError::Permission { path: denied } if denied == path
        ));
    }

    #[test]
    fn should_expose_incomplete_download_or_stage_for_guided_resume() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let bin = home.app_binary.parent().expect("binary has parent");
        fs::create_dir_all(bin).expect("bin creates");
        assert!(!incomplete_stage(&home));
        fs::write(bin.join(DOWNLOAD_NAME), b"partial").expect("partial writes");
        assert!(incomplete_stage(&home));
    }

    #[test]
    fn should_abort_before_network_when_external_runtime_is_already_present() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let manifest_url =
            Url::parse("http://127.0.0.1:1/runtime.json").expect("manifest URL parses");
        let request = ProvisionRequest {
            home: &home,
            manifest_url: &manifest_url,
            public_key: "unused",
            target: "darwin-aarch64",
            installed: None,
            app_version: "0.3.0",
            channel: "beta",
            resume: ResumeChoice::Continue,
        };
        assert_eq!(
            provision(&request, &|| true, &mut |_stage| {})
                .expect("external runtime aborts provisioning"),
            ProvisionOutcome::AttachExternally
        );
    }
}
