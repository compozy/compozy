use std::fs;
use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sysinfo::{Pid, ProcessesToUpdate, System};

const PROCESS_START_TOLERANCE_SECONDS: i64 = 2;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DaemonRecord {
    pub pid: u32,
    pub port: u16,
    pub started_at: DateTime<Utc>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Discovery {
    Live(DaemonRecord),
    Absent,
    AbsentWithDiagnostic(String),
}

pub trait ProcessTable: Send + Sync {
    fn start_time(&self, pid: u32) -> Option<DateTime<Utc>>;
    fn executable(&self, pid: u32) -> Option<PathBuf>;
}

#[derive(Debug, Default)]
pub struct SystemProcessTable;

impl ProcessTable for SystemProcessTable {
    fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
        let mut system = System::new();
        let pid = Pid::from_u32(pid);
        system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
        let seconds = system.process(pid)?.start_time() as i64;
        DateTime::from_timestamp(seconds, 0)
    }

    fn executable(&self, pid: u32) -> Option<PathBuf> {
        let mut system = System::new();
        let pid = Pid::from_u32(pid);
        system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
        system.process(pid)?.exe().map(Path::to_path_buf)
    }
}

pub fn discover(path: &Path, processes: &dyn ProcessTable) -> Discovery {
    let bytes = match fs::read(path) {
        Ok(bytes) => bytes,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Discovery::Absent,
        Err(error) => {
            return Discovery::AbsentWithDiagnostic(format!("read daemon record: {error}"));
        }
    };
    let record: DaemonRecord = match serde_json::from_slice(&bytes) {
        Ok(record) => record,
        Err(error) => {
            return Discovery::AbsentWithDiagnostic(format!("decode daemon record: {error}"));
        }
    };
    if record.pid == 0 || record.port == 0 {
        return Discovery::AbsentWithDiagnostic("daemon record has invalid identity".to_owned());
    }
    let Some(observed_start) = processes.start_time(record.pid) else {
        return Discovery::Absent;
    };
    let difference = (observed_start.timestamp() - record.started_at.timestamp()).abs();
    if difference > PROCESS_START_TOLERANCE_SECONDS {
        return Discovery::Absent;
    }
    Discovery::Live(record)
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use super::*;

    #[derive(Default)]
    struct FakeProcesses {
        starts: HashMap<u32, DateTime<Utc>>,
    }

    impl ProcessTable for FakeProcesses {
        fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
            self.starts.get(&pid).copied()
        }

        fn executable(&self, _pid: u32) -> Option<PathBuf> {
            None
        }
    }

    fn write_record(directory: &tempfile::TempDir, body: &str) -> PathBuf {
        let path = directory.path().join("daemon.json");
        fs::write(&path, body).expect("fixture record writes");
        path
    }

    #[test]
    fn should_discover_live_record_with_matching_process_start() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let started_at = Utc::now();
        let path = write_record(
            &directory,
            &format!(
                r#"{{"pid":42,"port":2123,"started_at":"{}"}}"#,
                started_at.to_rfc3339()
            ),
        );
        let processes = FakeProcesses {
            starts: HashMap::from([(42, started_at)]),
        };
        assert!(matches!(discover(&path, &processes), Discovery::Live(_)));
    }

    #[test]
    fn should_reject_dead_or_reused_pid() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let started_at = Utc::now();
        let path = write_record(
            &directory,
            &format!(
                r#"{{"pid":42,"port":2123,"started_at":"{}"}}"#,
                started_at.to_rfc3339()
            ),
        );
        assert_eq!(
            discover(&path, &FakeProcesses::default()),
            Discovery::Absent
        );

        let processes = FakeProcesses {
            starts: HashMap::from([(42, started_at - chrono::Duration::hours(1))]),
        };
        assert_eq!(discover(&path, &processes), Discovery::Absent);
    }

    #[test]
    fn should_treat_malformed_record_as_absent_with_diagnostic() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = write_record(&directory, "{\"pid\":");
        assert!(matches!(
            discover(&path, &FakeProcesses::default()),
            Discovery::AbsentWithDiagnostic(_)
        ));
    }
}
