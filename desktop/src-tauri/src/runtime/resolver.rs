use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};

use crate::errors::ShellError;
use crate::home::CompozyHome;

use super::boot_timestamp_drift::{
    BoundBootTimestampDrift, bound_app_owned_runtime_with_boot_timestamp_drift,
};
use super::discovery::{DaemonRecord, Discovery, ProcessTable, discover};
use super::probe::{BoundDaemonIdentity, Identity, IdentityProbe, identity_error};
use super::provenance;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BinarySource {
    AppOwned,
    OperatorPath,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Resolution {
    Attached {
        identity: BoundDaemonIdentity,
        owned: bool,
    },
    Awaiting(DaemonRecord),
    StartTimeMismatch {
        record: DaemonRecord,
        observed_start: DateTime<Utc>,
    },
    NeedsStart {
        binary: PathBuf,
        source: BinarySource,
    },
    NeedsProvision,
    Failed {
        error: ShellError,
    },
}

pub trait RuntimeResolver {
    fn resolve(&self, home: &CompozyHome) -> Resolution;
}

pub trait InstallLocator: Send + Sync {
    fn app_owned(&self, home: &CompozyHome) -> Option<PathBuf>;
    fn operator_install(&self, home: &CompozyHome) -> Option<PathBuf>;
    fn owns_executable(&self, home: &CompozyHome, executable: &Path) -> bool;
}

pub struct AttachFirstResolver<'a> {
    pub processes: &'a dyn ProcessTable,
    pub probe: &'a dyn IdentityProbe,
    pub installs: &'a dyn InstallLocator,
}

impl RuntimeResolver for AttachFirstResolver<'_> {
    fn resolve(&self, home: &CompozyHome) -> Resolution {
        match discover(&home.daemon_info, self.processes) {
            Discovery::Live(record) => match self.probe.probe(&record, None) {
                Identity::Compozy(identity) => {
                    let owned = self
                        .processes
                        .executable(record.pid)
                        .is_some_and(|executable| self.installs.owns_executable(home, &executable));
                    return Resolution::Attached {
                        identity: *identity,
                        owned,
                    };
                }
                Identity::Unreachable => return Resolution::Awaiting(record),
                identity @ Identity::Foreign => {
                    return Resolution::Failed {
                        error: identity_error(&identity, home.app_log.clone())
                            .expect("non-Compozy identity maps to an error"),
                    };
                }
            },
            Discovery::StartTimeMismatch {
                record,
                observed_start,
            } => match bound_app_owned_runtime_with_boot_timestamp_drift(
                home,
                self.processes,
                self.probe,
                &record,
                observed_start,
            ) {
                BoundBootTimestampDrift::Attached(identity) => {
                    return Resolution::Attached {
                        identity: *identity,
                        owned: true,
                    };
                }
                BoundBootTimestampDrift::Unreachable => {
                    return Resolution::StartTimeMismatch {
                        record,
                        observed_start,
                    };
                }
                BoundBootTimestampDrift::NotOwned => {
                    return Resolution::Failed {
                        error: ShellError::from_code(
                            crate::errors::ShellErrorCode::NotOwned,
                            home.app_log.clone(),
                        ),
                    };
                }
                BoundBootTimestampDrift::IdentityMismatch => {
                    return Resolution::Failed {
                        error: ShellError::from_code(
                            crate::errors::ShellErrorCode::RuntimeUnhealthy,
                            home.app_log.clone(),
                        ),
                    };
                }
            },
            Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => {}
        }
        if let Some(binary) = self.installs.app_owned(home) {
            return Resolution::NeedsStart {
                binary,
                source: BinarySource::AppOwned,
            };
        }
        if let Some(binary) = self.installs.operator_install(home) {
            return Resolution::NeedsStart {
                binary,
                source: BinarySource::OperatorPath,
            };
        }
        Resolution::NeedsProvision
    }
}

#[derive(Debug, Default)]
pub struct SystemInstallLocator;

impl InstallLocator for SystemInstallLocator {
    fn app_owned(&self, home: &CompozyHome) -> Option<PathBuf> {
        provenance::validated_owned_binary(home)
    }

    fn operator_install(&self, home: &CompozyHome) -> Option<PathBuf> {
        operator_install_with_path(home, which::which("compozy").ok())
    }

    fn owns_executable(&self, home: &CompozyHome, executable: &Path) -> bool {
        provenance::owns_executable(home, executable)
    }
}

fn operator_install_with_path(home: &CompozyHome, path_binary: Option<PathBuf>) -> Option<PathBuf> {
    first_operator_binary(path_binary, platform_probe_paths(home))
}

fn first_operator_binary(
    path_binary: Option<PathBuf>,
    fallback_paths: impl IntoIterator<Item = PathBuf>,
) -> Option<PathBuf> {
    if let Some(binary) = path_binary.and_then(provenance::executable_file) {
        return Some(binary);
    }
    fallback_paths
        .into_iter()
        .find_map(provenance::executable_file)
}

fn platform_probe_paths(home: &CompozyHome) -> Vec<PathBuf> {
    let executable = if cfg!(windows) {
        "compozy.exe"
    } else {
        "compozy"
    };
    let mut paths = Vec::new();
    if let Some(operator_home) = dirs::home_dir() {
        paths.push(operator_home.join(".local/bin").join(executable));
    }
    paths.push(home.root.join("bin").join(executable));
    #[cfg(target_os = "macos")]
    {
        paths.push(PathBuf::from("/opt/homebrew/bin/compozy"));
        paths.push(PathBuf::from("/usr/local/bin/compozy"));
    }
    #[cfg(all(unix, not(target_os = "macos")))]
    {
        paths.push(PathBuf::from("/usr/local/bin/compozy"));
        paths.push(PathBuf::from("/usr/bin/compozy"));
    }
    paths
}

#[cfg(test)]
#[path = "resolver_tests.rs"]
mod tests;
