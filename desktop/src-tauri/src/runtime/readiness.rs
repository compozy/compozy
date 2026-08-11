use std::path::Path;
use std::time::Duration;

use crate::home::CompozyHome;

use super::boot_timestamp_drift::{
    BoundBootTimestampDrift, bound_app_owned_runtime_with_boot_timestamp_drift,
};
use super::discovery::{Discovery, ProcessTable, SystemProcessTable, discover};
use super::probe::{BoundDaemonIdentity, BoundProbe, HttpStatusTransport, Identity, IdentityProbe};
use super::supervisor::ReadinessProbe;

pub struct DaemonReadiness<'a> {
    pub daemon_record: &'a Path,
    pub home: &'a Path,
    pub processes: &'a dyn ProcessTable,
    pub probe: &'a dyn IdentityProbe,
}

pub fn live_bound_runtime(home: &CompozyHome) -> bool {
    let processes = SystemProcessTable;
    let probe = BoundProbe::new(HttpStatusTransport::default(), Duration::from_secs(2));
    match discover(&home.daemon_info, &processes) {
        Discovery::Live(record) => matches!(probe.probe(&record, None), Identity::Compozy(_)),
        Discovery::StartTimeMismatch {
            record,
            observed_start,
        } => matches!(
            bound_app_owned_runtime_with_boot_timestamp_drift(
                home,
                &processes,
                &probe,
                &record,
                observed_start,
            ),
            BoundBootTimestampDrift::Attached(_)
        ),
        Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => false,
    }
}

impl ReadinessProbe for DaemonReadiness<'_> {
    fn ready(&self, expected_child_pid: Option<u32>) -> Option<BoundDaemonIdentity> {
        let identity = match discover(self.daemon_record, self.processes) {
            Discovery::Live(record) => match self.probe.probe(&record, None) {
                Identity::Compozy(identity) => *identity,
                Identity::Foreign | Identity::Unreachable => return None,
            },
            Discovery::StartTimeMismatch {
                record,
                observed_start,
            } => {
                let home = CompozyHome::from_root(self.home.to_path_buf());
                match bound_app_owned_runtime_with_boot_timestamp_drift(
                    &home,
                    self.processes,
                    self.probe,
                    &record,
                    observed_start,
                ) {
                    BoundBootTimestampDrift::Attached(identity) => *identity,
                    BoundBootTimestampDrift::Unreachable
                    | BoundBootTimestampDrift::NotOwned
                    | BoundBootTimestampDrift::IdentityMismatch => return None,
                }
            }
            Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => return None,
        };
        if expected_child_pid
            .is_some_and(|pid| !self.processes.is_descendant(identity.record.pid, pid))
        {
            return None;
        }
        Some(identity)
    }

    fn process_alive(&self, pid: u32) -> bool {
        self.processes.start_time(pid).is_some()
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;
    use std::path::PathBuf;

    use chrono::{DateTime, Utc};

    use super::*;
    use crate::runtime::discovery::DaemonRecord;
    use crate::runtime::probe::{DaemonStatusPayload, StatusPayload};

    struct FakeProcesses {
        starts: HashMap<u32, DateTime<Utc>>,
        descendant: bool,
    }

    impl ProcessTable for FakeProcesses {
        fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
            self.starts.get(&pid).copied()
        }

        fn executable(&self, _pid: u32) -> Option<PathBuf> {
            None
        }

        fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
            descendant == ancestor || self.descendant
        }
    }

    struct FakeProbe(BoundDaemonIdentity);

    impl IdentityProbe for FakeProbe {
        fn probe(&self, _record: &DaemonRecord, expected_child_pid: Option<u32>) -> Identity {
            if expected_child_pid.is_none() {
                Identity::Compozy(Box::new(self.0.clone()))
            } else {
                Identity::Foreign
            }
        }
    }

    fn identity(record: DaemonRecord, home: &Path) -> BoundDaemonIdentity {
        BoundDaemonIdentity {
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
            origin: url::Url::parse("http://localhost:2123").expect("fixture URL parses"),
        }
    }

    #[test]
    fn should_accept_a_bound_daemon_descended_from_the_spawned_launcher() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let daemon_record = directory.path().join("daemon.json");
        let started_at = Utc::now();
        let record = DaemonRecord {
            pid: 77,
            port: 2123,
            started_at,
        };
        std::fs::write(
            &daemon_record,
            serde_json::to_vec(&record).expect("record serializes"),
        )
        .expect("record writes");
        let processes = FakeProcesses {
            starts: HashMap::from([(77, started_at)]),
            descendant: true,
        };
        let probe = FakeProbe(identity(record, directory.path()));
        let readiness = DaemonReadiness {
            daemon_record: &daemon_record,
            home: directory.path(),
            processes: &processes,
            probe: &probe,
        };

        assert!(readiness.ready(Some(42)).is_some());
    }

    #[test]
    fn should_reject_a_bound_daemon_outside_the_spawned_launcher_lineage() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let daemon_record = directory.path().join("daemon.json");
        let started_at = Utc::now();
        let record = DaemonRecord {
            pid: 77,
            port: 2123,
            started_at,
        };
        std::fs::write(
            &daemon_record,
            serde_json::to_vec(&record).expect("record serializes"),
        )
        .expect("record writes");
        let processes = FakeProcesses {
            starts: HashMap::from([(77, started_at)]),
            descendant: false,
        };
        let probe = FakeProbe(identity(record, directory.path()));
        let readiness = DaemonReadiness {
            daemon_record: &daemon_record,
            home: directory.path(),
            processes: &processes,
            probe: &probe,
        };

        assert!(readiness.ready(Some(42)).is_none());
    }
}
