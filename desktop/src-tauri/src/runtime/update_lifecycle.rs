use std::path::Path;
use std::thread;
use std::time::{Duration, Instant};

use semver::Version;
#[cfg(not(windows))]
use sysinfo::Signal;
use sysinfo::{Pid, ProcessesToUpdate, System};
use url::Url;

use crate::home::CompozyHome;
use crate::update::runtime_update::{RuntimeInstance, RuntimeLifecycle, RuntimeUpdaterError};

use super::discovery::{Discovery, ProcessTable, SystemProcessTable, discover};
use super::probe::{BoundProbe, HttpStatusTransport, Identity, IdentityProbe};
use super::quiesce::{HttpQuiesceTransport, QuiesceTransport};
use super::readiness::DaemonReadiness;
use super::supervisor::{StartResult, Supervisor, SystemSpawner, ThreadDelay};

const REQUEST_TIMEOUT: Duration = Duration::from_secs(2);
const STOP_DEADLINE: Duration = Duration::from_secs(15);

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
            Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => {
                return Err(RuntimeUpdaterError::Lifecycle);
            }
        };
        let probe = BoundProbe::new(HttpStatusTransport, REQUEST_TIMEOUT);
        match probe.probe(&record, &self.home.root, None) {
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
                Version::parse(version).map_err(|_| RuntimeUpdaterError::Version)
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
        let mut system = System::new();
        let system_pid = Pid::from_u32(pid);
        system.refresh_processes(ProcessesToUpdate::Some(&[system_pid]), true);
        let process = system
            .process(system_pid)
            .ok_or(RuntimeUpdaterError::Lifecycle)?;
        #[cfg(windows)]
        let signaled = process.kill();
        #[cfg(not(windows))]
        let signaled = process.kill_with(Signal::Term) == Some(true);
        if !signaled {
            return Err(RuntimeUpdaterError::Lifecycle);
        }
        let started = Instant::now();
        while started.elapsed() < STOP_DEADLINE {
            system.refresh_processes(ProcessesToUpdate::Some(&[system_pid]), true);
            if system.process(system_pid).is_none() {
                return Ok(());
            }
            thread::sleep(Duration::from_millis(100));
        }
        Err(RuntimeUpdaterError::Lifecycle)
    }

    fn start_and_verify(
        &self,
        executable: &Path,
        expected: &Version,
    ) -> Result<(), RuntimeUpdaterError> {
        let probe = BoundProbe::new(HttpStatusTransport, REQUEST_TIMEOUT);
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
            Err(_) => return Err(RuntimeUpdaterError::Lifecycle),
        };
        let actual = identity
            .status
            .daemon
            .version
            .as_deref()
            .ok_or(RuntimeUpdaterError::Version)
            .and_then(|version| {
                Version::parse(version).map_err(|_| RuntimeUpdaterError::Version)
            })?;
        if &actual != expected {
            return Err(RuntimeUpdaterError::Version);
        }
        Ok(())
    }
}
