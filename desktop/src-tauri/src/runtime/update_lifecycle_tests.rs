use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use std::sync::Mutex;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use chrono::{DateTime, Duration as ChronoDuration, Utc};

use super::*;
use crate::home::CompozyHome;
use crate::runtime::discovery::DaemonRecord;
use crate::runtime::provenance::{self, ProvenanceRecord};

struct FakeRecoveryProcesses {
    starts: Mutex<HashMap<u32, DateTime<Utc>>>,
    executables: HashMap<u32, PathBuf>,
    graceful_stops: AtomicUsize,
    forced_stops: AtomicUsize,
    stop_on_graceful: bool,
    stop_on_forced: bool,
    replacement_after_graceful: Option<DateTime<Utc>>,
}

impl ProcessTable for FakeRecoveryProcesses {
    fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
        self.starts
            .lock()
            .expect("process starts lock")
            .get(&pid)
            .copied()
    }

    fn executable(&self, pid: u32) -> Option<PathBuf> {
        self.executables.get(&pid).cloned()
    }

    fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
        descendant == ancestor
    }
}

impl RecoveryProcess for FakeRecoveryProcesses {
    fn terminate(&self, pid: u32, expected_start: DateTime<Utc>) -> std::io::Result<()> {
        if self.start_time(pid) != Some(expected_start) {
            return Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "process identity changed",
            ));
        }
        self.graceful_stops.fetch_add(1, Ordering::SeqCst);
        if self.stop_on_graceful {
            self.starts
                .lock()
                .expect("process starts lock")
                .remove(&pid);
        } else if let Some(replacement) = self.replacement_after_graceful {
            self.starts
                .lock()
                .expect("process starts lock")
                .insert(pid, replacement);
        }
        Ok(())
    }

    fn force_terminate(&self, pid: u32, expected_start: DateTime<Utc>) -> std::io::Result<()> {
        if self.start_time(pid) != Some(expected_start) {
            return Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "process identity changed",
            ));
        }
        self.forced_stops.fetch_add(1, Ordering::SeqCst);
        if self.stop_on_forced {
            self.starts
                .lock()
                .expect("process starts lock")
                .remove(&pid);
        }
        Ok(())
    }
}

#[derive(Default)]
struct FakeDelay;

impl Delay for FakeDelay {
    fn sleep(&self, _duration: Duration) {}
}

fn owned_runtime() -> (tempfile::TempDir, CompozyHome, DaemonRecord) {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let parent = home.app_binary.parent().expect("runtime binary has parent");
    fs::create_dir_all(parent).expect("runtime directory creates");
    fs::write(&home.app_binary, b"desktop runtime").expect("runtime binary writes");
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;

        fs::set_permissions(&home.app_binary, fs::Permissions::from_mode(0o755))
            .expect("runtime binary becomes executable");
    }
    let marker = ProvenanceRecord::desktop(
        "0.4.1",
        "0.4.1",
        "stable",
        Utc::now(),
        provenance::sha256_file(&home.app_binary).expect("runtime binary hashes"),
    );
    provenance::write_atomic(&home.provenance, &marker).expect("provenance writes");
    let record = DaemonRecord {
        pid: 42,
        port: 2123,
        started_at: Utc::now(),
    };
    fs::write(
        &home.daemon_info,
        serde_json::to_vec(&record).expect("daemon record serializes"),
    )
    .expect("daemon record writes");
    (directory, home, record)
}

#[test]
fn should_recover_only_a_verified_desktop_owned_runtime_once() {
    let (_directory, home, record) = owned_runtime();
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, record.started_at)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: true,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Recovered
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 1);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 0);
    assert!(!home.daemon_info.exists());

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Unverified
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 1);
}

#[test]
fn should_not_touch_an_external_runtime_during_recovery() {
    let (_directory, home, record) = owned_runtime();
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, record.started_at)])),
        executables: HashMap::from([(record.pid, PathBuf::from("/usr/local/bin/compozy"))]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: true,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Unverified
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 0);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 0);
    assert!(home.daemon_info.exists());
}

#[test]
fn should_not_touch_a_runtime_when_its_process_start_no_longer_matches_the_record() {
    let (_directory, home, record) = owned_runtime();
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(
            record.pid,
            record.started_at
                + ChronoDuration::seconds(crate::runtime::PROCESS_START_TOLERANCE_SECONDS + 1),
        )])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: true,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Unverified
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 0);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 0);
    assert!(home.daemon_info.exists());
}

#[test]
fn should_not_touch_a_runtime_when_its_provenance_hash_no_longer_matches() {
    let (_directory, home, record) = owned_runtime();
    fs::write(&home.app_binary, b"tampered desktop runtime")
        .expect("runtime binary changes after provenance writes");
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, record.started_at)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: true,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Unverified
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 0);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 0);
    assert!(home.daemon_info.exists());
}

#[test]
fn should_recover_a_forward_boot_timestamp_drift_only_when_its_drift_is_bounded() {
    let (_directory, home, mut record) = owned_runtime();
    let observed_start = record.started_at;
    record.started_at += ChronoDuration::seconds(5);
    fs::write(
        &home.daemon_info,
        serde_json::to_vec(&record).expect("drifted record serializes"),
    )
    .expect("drifted record writes");
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, observed_start)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: true,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_boot_timestamp_drift_runtime(
            &home,
            &record,
            observed_start,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Recovered
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 1);
    assert!(!home.daemon_info.exists());
}

#[test]
fn should_not_touch_a_reverse_boot_timestamp_drift() {
    let (_directory, home, record) = owned_runtime();
    let observed_start = record.started_at + ChronoDuration::seconds(5);
    fs::write(
        &home.daemon_info,
        serde_json::to_vec(&record).expect("drifted record serializes"),
    )
    .expect("drifted record writes");
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, observed_start)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: true,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_boot_timestamp_drift_runtime(
            &home,
            &record,
            observed_start,
            &processes,
            &delay,
            Duration::from_secs(1),
            Duration::from_millis(10),
        ),
        StalledRuntimeRecovery::Unverified
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 0);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 0);
    assert!(home.daemon_info.exists());
}

#[test]
fn should_force_stop_a_verified_runtime_that_ignores_graceful_termination() {
    let (_directory, home, record) = owned_runtime();
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, record.started_at)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: false,
        stop_on_forced: true,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_millis(1),
            Duration::from_millis(1),
        ),
        StalledRuntimeRecovery::Recovered
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 1);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 1);
}

#[test]
fn should_not_force_stop_a_reused_pid_after_graceful_termination() {
    let (_directory, home, record) = owned_runtime();
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, record.started_at)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: false,
        stop_on_forced: true,
        replacement_after_graceful: Some(record.started_at + ChronoDuration::hours(1)),
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_millis(1),
            Duration::from_millis(1),
        ),
        StalledRuntimeRecovery::Recovered
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 1);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 0);
}

#[test]
fn should_report_failed_recovery_when_a_verified_runtime_survives_forced_termination() {
    let (_directory, home, record) = owned_runtime();
    let processes = FakeRecoveryProcesses {
        starts: Mutex::new(HashMap::from([(record.pid, record.started_at)])),
        executables: HashMap::from([(record.pid, home.app_binary.clone())]),
        graceful_stops: AtomicUsize::new(0),
        forced_stops: AtomicUsize::new(0),
        stop_on_graceful: false,
        stop_on_forced: false,
        replacement_after_graceful: None,
    };
    let delay = FakeDelay;

    assert_eq!(
        recover_stalled_runtime(
            &home,
            &record,
            &processes,
            &delay,
            Duration::from_millis(1),
            Duration::from_millis(1),
        ),
        StalledRuntimeRecovery::Failed
    );
    assert_eq!(processes.graceful_stops.load(Ordering::SeqCst), 1);
    assert_eq!(processes.forced_stops.load(Ordering::SeqCst), 1);
    assert!(home.daemon_info.exists());
}
