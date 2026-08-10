use std::path::Path;

use super::discovery::{Discovery, ProcessTable, discover};
use super::probe::{BoundDaemonIdentity, Identity, IdentityProbe};
use super::supervisor::ReadinessProbe;

pub struct DaemonReadiness<'a> {
    pub daemon_record: &'a Path,
    pub home: &'a Path,
    pub processes: &'a dyn ProcessTable,
    pub probe: &'a dyn IdentityProbe,
}

impl ReadinessProbe for DaemonReadiness<'_> {
    fn ready(&self, expected_child_pid: Option<u32>) -> Option<BoundDaemonIdentity> {
        let Discovery::Live(record) = discover(self.daemon_record, self.processes) else {
            return None;
        };
        match self.probe.probe(&record, self.home, expected_child_pid) {
            Identity::Compozy(identity) => Some(*identity),
            Identity::Foreign | Identity::Unreachable => None,
        }
    }

    fn process_alive(&self, pid: u32) -> bool {
        self.processes.start_time(pid).is_some()
    }
}
