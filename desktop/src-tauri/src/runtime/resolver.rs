use std::fs;
use std::path::{Path, PathBuf};

use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;

use super::discovery::{Discovery, ProcessTable, discover};
use super::probe::{BoundDaemonIdentity, Identity, IdentityProbe};
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
            Discovery::Live(record) => match self.probe.probe(&record, &home.root, None) {
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
                Identity::Foreign => {
                    return Resolution::Failed {
                        error: ShellError::from_code(
                            ShellErrorCode::PortConflictForeign,
                            home.app_log.clone(),
                        ),
                    };
                }
                Identity::Unreachable => {
                    return Resolution::Failed {
                        error: ShellError::from_code(
                            ShellErrorCode::RuntimeUnhealthy,
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
    if let Some(binary) = path_binary.and_then(executable_file) {
        return Some(binary);
    }
    fallback_paths.into_iter().find_map(executable_file)
}

fn executable_file(path: PathBuf) -> Option<PathBuf> {
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
mod tests {
    use std::collections::HashMap;

    use chrono::{DateTime, Utc};

    use super::*;
    use crate::runtime::discovery::DaemonRecord;
    use crate::runtime::probe::{DaemonStatusPayload, StatusPayload};

    fn write_executable(path: &Path, body: &[u8]) {
        fs::create_dir_all(path.parent().expect("binary parent exists"))
            .expect("binary directory creates");
        fs::write(path, body).expect("binary fixture writes");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;

            fs::set_permissions(path, fs::Permissions::from_mode(0o755))
                .expect("binary fixture becomes executable");
        }
    }

    struct FakeProcesses {
        starts: HashMap<u32, DateTime<Utc>>,
        executables: HashMap<u32, PathBuf>,
    }

    impl ProcessTable for FakeProcesses {
        fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
            self.starts.get(&pid).copied()
        }

        fn executable(&self, pid: u32) -> Option<PathBuf> {
            self.executables.get(&pid).cloned()
        }
    }

    struct FakeProbe(Identity);

    impl IdentityProbe for FakeProbe {
        fn probe(&self, _record: &DaemonRecord, _home: &Path, _child: Option<u32>) -> Identity {
            self.0.clone()
        }
    }

    struct FakeInstalls {
        app_owned: Option<PathBuf>,
        operator: Option<PathBuf>,
        owned_live: bool,
    }

    impl InstallLocator for FakeInstalls {
        fn app_owned(&self, _home: &CompozyHome) -> Option<PathBuf> {
            self.app_owned.clone()
        }

        fn operator_install(&self, _home: &CompozyHome) -> Option<PathBuf> {
            self.operator.clone()
        }

        fn owns_executable(&self, _home: &CompozyHome, _executable: &Path) -> bool {
            self.owned_live
        }
    }

    fn identity(record: DaemonRecord, home: &Path) -> BoundDaemonIdentity {
        BoundDaemonIdentity {
            origin: url::Url::parse("http://localhost:2123").expect("fixture URL parses"),
            status: StatusPayload {
                schema_version: "2026-07-16".to_owned(),
                daemon: DaemonStatusPayload {
                    pid: record.pid,
                    started_at: record.started_at,
                    http_host: "localhost".to_owned(),
                    http_port: record.port,
                    user_home_dir: home.to_path_buf(),
                    version: Some("0.3.0".to_owned()),
                },
            },
            record,
        }
    }

    #[test]
    fn should_attach_live_daemon_before_considering_installs() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let started_at = Utc::now();
        let record = DaemonRecord {
            pid: 42,
            port: 2123,
            started_at,
        };
        fs::write(
            &home.daemon_info,
            serde_json::to_vec(&record).expect("record serializes"),
        )
        .expect("record writes");
        let processes = FakeProcesses {
            starts: HashMap::from([(42, started_at)]),
            executables: HashMap::from([(42, PathBuf::from("/tmp/compozy"))]),
        };
        let probe = FakeProbe(Identity::Compozy(Box::new(identity(record, &home.root))));
        let installs = FakeInstalls {
            app_owned: Some(PathBuf::from("/app/compozy")),
            operator: Some(PathBuf::from("/usr/bin/compozy")),
            owned_live: false,
        };
        let resolver = AttachFirstResolver {
            processes: &processes,
            probe: &probe,
            installs: &installs,
        };
        assert!(matches!(
            resolver.resolve(&home),
            Resolution::Attached { owned: false, .. }
        ));
    }

    #[test]
    fn should_prefer_app_owned_then_operator_then_provision() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let processes = FakeProcesses {
            starts: HashMap::new(),
            executables: HashMap::new(),
        };
        let probe = FakeProbe(Identity::Unreachable);
        let cases = [
            (
                Some(PathBuf::from("/app/compozy")),
                Some(PathBuf::from("/usr/bin/compozy")),
                Some(BinarySource::AppOwned),
            ),
            (
                None,
                Some(PathBuf::from("/usr/bin/compozy")),
                Some(BinarySource::OperatorPath),
            ),
            (None, None, None),
        ];
        for (app_owned, operator, expected) in cases {
            let installs = FakeInstalls {
                app_owned,
                operator,
                owned_live: false,
            };
            let resolver = AttachFirstResolver {
                processes: &processes,
                probe: &probe,
                installs: &installs,
            };
            match (resolver.resolve(&home), expected) {
                (Resolution::NeedsStart { source, .. }, Some(expected)) => {
                    assert_eq!(source, expected);
                }
                (Resolution::NeedsProvision, None) => {}
                (actual, expected) => {
                    panic!("unexpected resolution {actual:?}, expected {expected:?}")
                }
            }
        }
    }

    #[test]
    fn should_require_valid_provenance_hash_for_app_ownership() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        write_executable(&home.app_binary, b"owned runtime");
        let digest = provenance::sha256_file(&home.app_binary).expect("binary hashes");
        let marker =
            provenance::ProvenanceRecord::desktop("0.3.0", "0.3.0", "beta", Utc::now(), digest);
        fs::write(
            &home.provenance,
            serde_json::to_vec(&marker).expect("provenance serializes"),
        )
        .expect("provenance writes");
        let locator = SystemInstallLocator;
        assert_eq!(locator.app_owned(&home), Some(home.app_binary.clone()));
        assert!(locator.owns_executable(&home, &home.app_binary));

        fs::write(
            &home.provenance,
            r#"{"installed_by":"desktop-app","binary_sha256":"deadbeef"}"#,
        )
        .expect("invalid provenance writes");
        assert_eq!(locator.app_owned(&home), None);
        assert!(!locator.owns_executable(&home, &home.app_binary));
    }

    #[test]
    fn should_fall_back_deterministically_when_gui_path_lookup_is_empty() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        write_executable(&home.app_binary, b"operator runtime without provenance");
        assert_eq!(
            first_operator_binary(None, [home.app_binary.clone()]),
            Some(home.app_binary.clone())
        );

        fs::remove_file(&home.app_binary).expect("operator fixture removes");
        assert_eq!(first_operator_binary(None, [home.app_binary.clone()]), None);
    }
}
