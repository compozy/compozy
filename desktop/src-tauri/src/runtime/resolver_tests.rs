use std::collections::HashMap;
use std::fs;

use chrono::{DateTime, Utc};

use super::*;
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

    fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
        descendant == ancestor
    }
}

struct FakeProbe(Identity);

impl IdentityProbe for FakeProbe {
    fn probe(&self, _record: &DaemonRecord, _child: Option<u32>) -> Identity {
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
fn should_wait_for_a_live_recorded_daemon_before_considering_installs() {
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
        executables: HashMap::new(),
    };
    let probe = FakeProbe(Identity::Unreachable);
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

    assert_eq!(resolver.resolve(&home), Resolution::Awaiting(record));
}

#[test]
fn should_attach_a_verified_forward_boot_timestamp_drift_daemon_without_spawning() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let observed_start = Utc::now();
    let record = DaemonRecord {
        pid: 42,
        port: 2123,
        started_at: observed_start + chrono::Duration::seconds(5),
    };
    fs::write(
        &home.daemon_info,
        serde_json::to_vec(&record).expect("drifted record serializes"),
    )
    .expect("drifted record writes");
    write_executable(&home.app_binary, b"owned drifted runtime");
    let marker = provenance::ProvenanceRecord::desktop(
        "0.4.1",
        "0.4.1",
        "stable",
        Utc::now(),
        provenance::sha256_file(&home.app_binary).expect("runtime binary hashes"),
    );
    provenance::write_atomic(&home.provenance, &marker).expect("provenance writes");
    let processes = FakeProcesses {
        starts: HashMap::from([(record.pid, observed_start)]),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
    };
    let probe = FakeProbe(Identity::Compozy(Box::new(identity(
        record.clone(),
        &home.root,
    ))));
    let installs = SystemInstallLocator;
    let resolver = AttachFirstResolver {
        processes: &processes,
        probe: &probe,
        installs: &installs,
    };

    assert!(
        matches!(resolver.resolve(&home), Resolution::Attached { identity, owned: true } if identity.record == record)
    );
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
            (Resolution::NeedsStart { source, .. }, Some(expected)) => assert_eq!(source, expected),
            (Resolution::NeedsProvision, None) => {}
            (actual, expected) => panic!("unexpected resolution {actual:?}, expected {expected:?}"),
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
