use std::fs;
use std::io;
use std::path::Path;
use std::time::Duration;

use chrono::{DateTime, Utc};
use semver::Version;
#[cfg(not(windows))]
use sysinfo::Signal;
use sysinfo::{Pid, ProcessesToUpdate, System};
use url::Url;

use crate::home::CompozyHome;
use crate::update::runtime_update::{RuntimeInstance, RuntimeLifecycle, RuntimeUpdaterError};

use super::boot_timestamp_drift::{
    BoundBootTimestampDrift, bound_app_owned_runtime_with_boot_timestamp_drift,
    is_bounded_forward_boot_timestamp_drift,
};
use super::discovery::{Discovery, ProcessTable, SystemProcessTable, discover};
use super::probe::{BoundProbe, HttpStatusTransport, Identity, IdentityProbe};
use super::provenance;
use super::quiesce::{HttpQuiesceTransport, QuiesceTransport};
use super::readiness::DaemonReadiness;
use super::supervisor::{Delay, StartResult, Supervisor, SystemSpawner, ThreadDelay};

const REQUEST_TIMEOUT: Duration = Duration::from_secs(2);
pub(crate) const STOP_DEADLINE: Duration = Duration::from_secs(15);
const STOP_POLL_INTERVAL: Duration = Duration::from_millis(100);

pub trait RecoveryProcess: ProcessTable {
    fn terminate(&self, pid: u32) -> io::Result<()>;
    fn force_terminate(&self, pid: u32) -> io::Result<()>;
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum StalledRuntimeRecovery {
    Recovered,
    Unverified,
    Failed,
}

impl RecoveryProcess for SystemProcessTable {
    fn terminate(&self, pid: u32) -> io::Result<()> {
        signal_process(pid, false)
    }

    fn force_terminate(&self, pid: u32) -> io::Result<()> {
        signal_process(pid, true)
    }
}

pub fn recover_stalled_runtime(
    home: &CompozyHome,
    record: &super::discovery::DaemonRecord,
    processes: &dyn RecoveryProcess,
    delay: &dyn Delay,
    stop_deadline: Duration,
    poll_interval: Duration,
) -> StalledRuntimeRecovery {
    let verified_record = matches!(
        discover(&home.daemon_info, processes),
        Discovery::Live(current) if current == *record
    );
    if !verified_record {
        return StalledRuntimeRecovery::Unverified;
    }
    recover_verified_runtime(home, record, processes, delay, stop_deadline, poll_interval)
}

pub fn recover_boot_timestamp_drift_runtime(
    home: &CompozyHome,
    record: &super::discovery::DaemonRecord,
    observed_start: DateTime<Utc>,
    processes: &dyn RecoveryProcess,
    delay: &dyn Delay,
    stop_deadline: Duration,
    poll_interval: Duration,
) -> StalledRuntimeRecovery {
    let verified_record = matches!(
        discover(&home.daemon_info, processes),
        Discovery::StartTimeMismatch {
            record: current,
            observed_start: current_observed,
        } if current == *record && current_observed == observed_start
    );
    if !verified_record || !is_bounded_forward_boot_timestamp_drift(record, observed_start) {
        return StalledRuntimeRecovery::Unverified;
    }
    recover_verified_runtime(home, record, processes, delay, stop_deadline, poll_interval)
}

fn recover_verified_runtime(
    home: &CompozyHome,
    record: &super::discovery::DaemonRecord,
    processes: &dyn RecoveryProcess,
    delay: &dyn Delay,
    stop_deadline: Duration,
    poll_interval: Duration,
) -> StalledRuntimeRecovery {
    let Some(executable) = processes.executable(record.pid) else {
        return StalledRuntimeRecovery::Unverified;
    };
    if !provenance::owns_executable(home, &executable) {
        return StalledRuntimeRecovery::Unverified;
    }
    if stop_process(processes, record.pid, delay, stop_deadline, poll_interval).is_err() {
        return StalledRuntimeRecovery::Failed;
    }
    if clear_daemon_record(&home.daemon_info).is_err() {
        return StalledRuntimeRecovery::Failed;
    }
    StalledRuntimeRecovery::Recovered
}

fn signal_process(pid: u32, force: bool) -> io::Result<()> {
    let mut system = System::new();
    let process_id = Pid::from_u32(pid);
    system.refresh_processes(ProcessesToUpdate::Some(&[process_id]), true);
    let process = system
        .process(process_id)
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "runtime process is absent"))?;
    let signaled = if force {
        process.kill()
    } else {
        #[cfg(windows)]
        let signaled = process.kill();
        #[cfg(not(windows))]
        let signaled = process.kill_with(Signal::Term) == Some(true);
        signaled
    };
    signaled
        .then_some(())
        .ok_or_else(|| io::Error::other("runtime process rejected termination"))
}

fn stop_process(
    processes: &dyn RecoveryProcess,
    pid: u32,
    delay: &dyn Delay,
    deadline: Duration,
    poll_interval: Duration,
) -> io::Result<()> {
    processes.terminate(pid)?;
    if wait_for_exit(processes, pid, delay, deadline, poll_interval) {
        return Ok(());
    }
    processes.force_terminate(pid)?;
    wait_for_exit(processes, pid, delay, deadline, poll_interval)
        .then_some(())
        .ok_or_else(|| io::Error::other("runtime process did not exit after forced termination"))
}

fn wait_for_exit(
    processes: &dyn ProcessTable,
    pid: u32,
    delay: &dyn Delay,
    deadline: Duration,
    poll_interval: Duration,
) -> bool {
    for _ in 0..poll_limit(deadline, poll_interval) {
        if processes.start_time(pid).is_none() {
            return true;
        }
        delay.sleep(poll_interval);
    }
    processes.start_time(pid).is_none()
}

fn poll_limit(deadline: Duration, interval: Duration) -> usize {
    if interval.is_zero() {
        return 1;
    }
    let deadline_millis = deadline.as_millis();
    let interval_millis = interval.as_millis().max(1);
    deadline_millis.div_ceil(interval_millis).max(1) as usize
}

fn clear_daemon_record(path: &Path) -> io::Result<()> {
    match fs::remove_file(path) {
        Ok(()) => {}
        Err(error) if error.kind() == io::ErrorKind::NotFound => {}
        Err(error) => return Err(error),
    }
    if let Some(parent) = path.parent() {
        crate::durability::sync_directory(parent)?;
    }
    Ok(())
}

pub struct SystemRuntimeLifecycle {
    home: CompozyHome,
    processes: SystemProcessTable,
}

impl SystemRuntimeLifecycle {
    pub fn new(home: CompozyHome) -> Self {
        Self {
            home,
            processes: SystemProcessTable,
        }
    }

    fn probe_identity(&self) -> Result<super::probe::BoundDaemonIdentity, RuntimeUpdaterError> {
        let record = match discover(&self.home.daemon_info, &self.processes) {
            Discovery::Live(record) => record,
            Discovery::StartTimeMismatch {
                record,
                observed_start,
            } => {
                let probe = BoundProbe::new(HttpStatusTransport::default(), REQUEST_TIMEOUT);
                return match bound_app_owned_runtime_with_boot_timestamp_drift(
                    &self.home,
                    &self.processes,
                    &probe,
                    &record,
                    observed_start,
                ) {
                    BoundBootTimestampDrift::Attached(identity) => Ok(*identity),
                    BoundBootTimestampDrift::Unreachable
                    | BoundBootTimestampDrift::NotOwned
                    | BoundBootTimestampDrift::IdentityMismatch => {
                        Err(RuntimeUpdaterError::Lifecycle)
                    }
                };
            }
            Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => {
                return Err(RuntimeUpdaterError::Lifecycle);
            }
        };
        let probe = BoundProbe::new(HttpStatusTransport::default(), REQUEST_TIMEOUT);
        match probe.probe(&record, None) {
            Identity::Compozy(identity) => Ok(*identity),
            Identity::Foreign | Identity::Unreachable => Err(RuntimeUpdaterError::Lifecycle),
        }
    }
}

impl RuntimeLifecycle for SystemRuntimeLifecycle {
    fn processes(&self) -> &dyn ProcessTable {
        &self.processes
    }

    fn probe(&self) -> Result<RuntimeInstance, RuntimeUpdaterError> {
        let identity = self.probe_identity()?;
        let version = identity
            .status
            .daemon
            .version
            .as_deref()
            .ok_or(RuntimeUpdaterError::Version)
            .and_then(|version| {
                Version::parse(version.trim_start_matches('v'))
                    .map_err(|_| RuntimeUpdaterError::Version)
            })?;
        let executable = self
            .processes
            .executable(identity.record.pid)
            .ok_or(RuntimeUpdaterError::Lifecycle)?;
        Ok(RuntimeInstance {
            pid: identity.record.pid,
            executable,
            origin: identity.origin,
            version,
        })
    }

    fn quiesce(&self, origin: &Url) -> Result<Box<dyn QuiesceTransport>, RuntimeUpdaterError> {
        let transport = HttpQuiesceTransport::new(origin, REQUEST_TIMEOUT)?;
        Ok(Box::new(transport))
    }

    fn stop(&self, pid: u32) -> Result<(), RuntimeUpdaterError> {
        let delay = ThreadDelay;
        self.processes
            .terminate(pid)
            .and_then(|()| {
                wait_for_exit(
                    &self.processes,
                    pid,
                    &delay,
                    STOP_DEADLINE,
                    STOP_POLL_INTERVAL,
                )
                .then_some(())
                .ok_or_else(|| io::Error::other("runtime process did not exit after termination"))
            })
            .map_err(|_| RuntimeUpdaterError::Lifecycle)
    }

    fn start_and_verify(
        &self,
        executable: &Path,
        expected: &Version,
    ) -> Result<(), RuntimeUpdaterError> {
        let probe = BoundProbe::new(HttpStatusTransport::default(), REQUEST_TIMEOUT);
        let readiness = DaemonReadiness {
            daemon_record: &self.home.daemon_info,
            home: &self.home.root,
            processes: &self.processes,
            probe: &probe,
        };
        let spawner = SystemSpawner;
        let delay = ThreadDelay;
        let supervisor = Supervisor::system(&spawner, &readiness, &delay);
        let identity = match supervisor.start_owned(executable, &self.home) {
            Ok(StartResult::Owned(runtime)) => runtime.identity,
            Ok(StartResult::Attached(identity)) => identity,
            Err(error) => {
                crate::logging::error(format!(
                    "restart runtime after update: {}",
                    error.safe_message
                ));
                return Err(RuntimeUpdaterError::Lifecycle);
            }
        };
        let actual = identity
            .status
            .daemon
            .version
            .as_deref()
            .ok_or(RuntimeUpdaterError::Version)
            .and_then(|version| {
                Version::parse(version.trim_start_matches('v'))
                    .map_err(|_| RuntimeUpdaterError::Version)
            })?;
        if &actual != expected {
            return Err(RuntimeUpdaterError::Version);
        }
        Ok(())
    }
}

#[cfg(test)]
#[path = "update_lifecycle_tests.rs"]
mod tests;
