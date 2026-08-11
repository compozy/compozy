use std::fs::File;
use std::io::Read;
use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sysinfo::{Pid, ProcessesToUpdate, System};

use super::PROCESS_START_TOLERANCE_SECONDS;

const MAX_ANCESTRY_DEPTH: usize = 256;
const MAX_DAEMON_RECORD_BYTES: u64 = 64 * 1024;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DaemonRecord {
    pub pid: u32,
    pub port: u16,
    pub started_at: DateTime<Utc>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Discovery {
    Live(DaemonRecord),
    StartTimeMismatch {
        record: DaemonRecord,
        observed_start: DateTime<Utc>,
    },
    Absent,
    AbsentWithDiagnostic(String),
}

pub trait ProcessTable: Send + Sync {
    fn start_time(&self, pid: u32) -> Option<DateTime<Utc>>;
    fn executable(&self, pid: u32) -> Option<PathBuf>;

    fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool;
}

#[derive(Debug, Default)]
pub struct SystemProcessTable;

impl ProcessTable for SystemProcessTable {
    fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
        let (system, pid) = refreshed_process(pid);
        let seconds = system.process(pid)?.start_time() as i64;
        DateTime::from_timestamp(seconds, 0)
    }

    fn executable(&self, pid: u32) -> Option<PathBuf> {
        let (system, pid) = refreshed_process(pid);
        system.process(pid)?.exe().map(Path::to_path_buf)
    }

    fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
        if descendant == ancestor {
            return true;
        }
        let mut system = System::new();
        system.refresh_processes(ProcessesToUpdate::All, true);
        let ancestor = Pid::from_u32(ancestor);
        let mut current = Pid::from_u32(descendant);
        for _ in 0..MAX_ANCESTRY_DEPTH {
            let Some(parent) = system.process(current).and_then(sysinfo::Process::parent) else {
                return false;
            };
            if parent == ancestor {
                return true;
            }
            if parent == current {
                return false;
            }
            current = parent;
        }
        false
    }
}

fn refreshed_process(pid: u32) -> (System, Pid) {
    let mut system = System::new();
    let pid = Pid::from_u32(pid);
    system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
    (system, pid)
}

pub fn discover(path: &Path, processes: &dyn ProcessTable) -> Discovery {
    let bytes = match read_daemon_record(path) {
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
        return Discovery::StartTimeMismatch {
            record,
            observed_start,
        };
    }
    Discovery::Live(record)
}

fn read_daemon_record(path: &Path) -> std::io::Result<Vec<u8>> {
    let mut bytes = Vec::new();
    File::open(path)?
        .take(MAX_DAEMON_RECORD_BYTES + 1)
        .read_to_end(&mut bytes)?;
    if bytes.len() as u64 > MAX_DAEMON_RECORD_BYTES {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            "daemon record exceeds size limit",
        ));
    }
    Ok(bytes)
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

        fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
            descendant == ancestor
        }
    }

    fn write_record(directory: &tempfile::TempDir, body: &str) -> PathBuf {
        let path = directory.path().join("daemon.json");
        std::fs::write(&path, body).expect("fixture record writes");
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
    fn should_distinguish_a_dead_pid_from_a_live_start_time_mismatch() {
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
        assert!(matches!(
            discover(&path, &processes),
            Discovery::StartTimeMismatch {
                record,
                observed_start,
            } if record.started_at == started_at
                && observed_start == started_at - chrono::Duration::hours(1)
        ));
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
