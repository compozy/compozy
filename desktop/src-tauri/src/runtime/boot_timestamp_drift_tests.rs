use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;

use chrono::{DateTime, Duration as ChronoDuration, Utc};

use super::*;
use crate::runtime::probe::{DaemonStatusPayload, StatusPayload};
use crate::runtime::provenance::{self, ProvenanceRecord};

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

fn identity(record: DaemonRecord, home: &CompozyHome) -> BoundDaemonIdentity {
    BoundDaemonIdentity {
        status: StatusPayload {
            schema_version: "2026-07-16".to_owned(),
            daemon: DaemonStatusPayload {
                pid: record.pid,
                started_at: record.started_at,
                http_host: "localhost".to_owned(),
                http_port: record.port,
                user_home_dir: home.root.clone(),
                version: Some("0.4.1".to_owned()),
            },
        },
        record,
        origin: url::Url::parse("http://localhost:2123").expect("fixture URL parses"),
    }
}

fn owned_runtime() -> (tempfile::TempDir, CompozyHome, DaemonRecord, DateTime<Utc>) {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    fs::create_dir_all(home.app_binary.parent().expect("binary parent exists"))
        .expect("binary directory creates");
    fs::write(&home.app_binary, b"owned runtime").expect("binary fixture writes");
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;

        fs::set_permissions(&home.app_binary, fs::Permissions::from_mode(0o755))
            .expect("binary fixture becomes executable");
    }
    provenance::write_atomic(
        &home.provenance,
        &ProvenanceRecord::desktop(
            "0.4.1",
            "0.4.1",
            "stable",
            Utc::now(),
            provenance::sha256_file(&home.app_binary).expect("binary hashes"),
        ),
    )
    .expect("provenance writes");
    let observed_start = Utc::now();
    let record = DaemonRecord {
        pid: 42,
        port: 2123,
        started_at: observed_start + ChronoDuration::seconds(5),
    };
    (directory, home, record, observed_start)
}

fn processes(home: &CompozyHome, record: &DaemonRecord, observed: DateTime<Utc>) -> FakeProcesses {
    FakeProcesses {
        starts: HashMap::from([(record.pid, observed)]),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
    }
}

#[test]
fn should_bind_a_forward_bounded_desktop_owned_compozy_runtime() {
    let (_directory, home, record, observed) = owned_runtime();
    let processes = processes(&home, &record, observed);
    let probe = FakeProbe(Identity::Compozy(Box::new(identity(record.clone(), &home))));

    assert!(matches!(
        bound_app_owned_runtime_with_boot_timestamp_drift(
            &home, &processes, &probe, &record, observed
        ),
        BoundBootTimestampDrift::Attached(identity) if identity.record == record
    ));
}

#[test]
fn should_reject_reverse_and_unbounded_boot_timestamp_drift() {
    let (_directory, home, record, observed) = owned_runtime();
    let probe = FakeProbe(Identity::Compozy(Box::new(identity(record.clone(), &home))));
    let reverse_observed = record.started_at + ChronoDuration::seconds(1);
    let reverse_processes = processes(&home, &record, reverse_observed);
    let unbounded_record = DaemonRecord {
        started_at: observed
            + ChronoDuration::from_std(READY_DEADLINE + std::time::Duration::from_secs(1))
                .expect("deadline converts to chrono duration"),
        ..record.clone()
    };
    let unbounded_processes = processes(&home, &unbounded_record, observed);

    assert!(matches!(
        bound_app_owned_runtime_with_boot_timestamp_drift(
            &home,
            &reverse_processes,
            &probe,
            &record,
            reverse_observed,
        ),
        BoundBootTimestampDrift::IdentityMismatch
    ));
    assert!(matches!(
        bound_app_owned_runtime_with_boot_timestamp_drift(
            &home,
            &unbounded_processes,
            &probe,
            &unbounded_record,
            observed,
        ),
        BoundBootTimestampDrift::IdentityMismatch
    ));
}

#[test]
fn should_require_current_provenance_for_a_boot_timestamp_drift_runtime() {
    let (_directory, home, record, observed) = owned_runtime();
    fs::write(&home.app_binary, b"tampered runtime").expect("binary fixture tampers");
    let processes = processes(&home, &record, observed);
    let probe = FakeProbe(Identity::Compozy(Box::new(identity(record.clone(), &home))));

    assert!(matches!(
        bound_app_owned_runtime_with_boot_timestamp_drift(
            &home, &processes, &probe, &record, observed
        ),
        BoundBootTimestampDrift::NotOwned
    ));
}

#[test]
fn should_require_a_compozy_identity_for_a_boot_timestamp_drift_runtime() {
    let (_directory, home, record, observed) = owned_runtime();
    let processes = processes(&home, &record, observed);
    let foreign = FakeProbe(Identity::Foreign);
    let mismatched = FakeProbe(Identity::Compozy(Box::new(identity(
        DaemonRecord {
            started_at: record.started_at + ChronoDuration::seconds(1),
            ..record.clone()
        },
        &home,
    ))));

    assert!(matches!(
        bound_app_owned_runtime_with_boot_timestamp_drift(
            &home, &processes, &foreign, &record, observed
        ),
        BoundBootTimestampDrift::IdentityMismatch
    ));
    assert!(matches!(
        bound_app_owned_runtime_with_boot_timestamp_drift(
            &home,
            &processes,
            &mismatched,
            &record,
            observed
        ),
        BoundBootTimestampDrift::IdentityMismatch
    ));
}
