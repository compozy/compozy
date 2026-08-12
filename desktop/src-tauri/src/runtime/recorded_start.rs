use std::time::Duration;

use crate::home::CompozyHome;
use chrono::{DateTime, Utc};

use super::boot_timestamp_drift::{
    BoundBootTimestampDrift, bound_app_owned_runtime_with_boot_timestamp_drift,
};
use super::discovery::DaemonRecord;
use super::probe::{BoundDaemonIdentity, IdentityProbe};
use super::provenance;
use super::readiness::DaemonReadiness;
use super::supervisor::{Delay, READY_DEADLINE, Supervisor, SystemSpawner, ThreadDelay};
use super::update_lifecycle::{
    RecoveryProcess, STOP_DEADLINE, StalledRuntimeRecovery, recover_boot_timestamp_drift_runtime,
    recover_stalled_runtime,
};

const RECOVERY_POLL_INTERVAL: Duration = Duration::from_millis(100);

pub enum RecordedDaemonStart {
    Attached {
        identity: Box<BoundDaemonIdentity>,
        owned: bool,
    },
    Recovered,
    Unverified,
    Failed,
}

pub fn await_or_recover_recorded_daemon(
    home: &CompozyHome,
    record: &DaemonRecord,
    processes: &dyn RecoveryProcess,
    probe: &dyn IdentityProbe,
) -> RecordedDaemonStart {
    let readiness = DaemonReadiness {
        daemon_record: &home.daemon_info,
        home: &home.root,
        processes,
        probe,
    };
    let spawner = SystemSpawner;
    let delay = ThreadDelay;
    let supervisor = Supervisor::system(&spawner, &readiness, &delay);
    if let Some(identity) = supervisor.wait_for_recorded_daemon(record) {
        let owned = processes
            .executable(record.pid)
            .is_some_and(|executable| provenance::owns_executable(home, &executable));
        return RecordedDaemonStart::Attached {
            identity: Box::new(identity),
            owned,
        };
    }
    match recover_stalled_runtime(
        home,
        record,
        processes,
        &delay,
        STOP_DEADLINE,
        RECOVERY_POLL_INTERVAL,
    ) {
        StalledRuntimeRecovery::Recovered => RecordedDaemonStart::Recovered,
        StalledRuntimeRecovery::Unverified => RecordedDaemonStart::Unverified,
        StalledRuntimeRecovery::Failed => RecordedDaemonStart::Failed,
    }
}

pub fn await_or_recover_boot_timestamp_drift_daemon(
    home: &CompozyHome,
    record: &DaemonRecord,
    observed_start: DateTime<Utc>,
    processes: &dyn RecoveryProcess,
    probe: &dyn IdentityProbe,
) -> RecordedDaemonStart {
    let delay = ThreadDelay;
    for _ in 0..poll_limit(READY_DEADLINE, RECOVERY_POLL_INTERVAL) {
        match bound_app_owned_runtime_with_boot_timestamp_drift(
            home,
            processes,
            probe,
            record,
            observed_start,
        ) {
            BoundBootTimestampDrift::Attached(identity) => {
                return RecordedDaemonStart::Attached {
                    identity,
                    owned: true,
                };
            }
            BoundBootTimestampDrift::Unreachable => delay.sleep(RECOVERY_POLL_INTERVAL),
            BoundBootTimestampDrift::NotOwned | BoundBootTimestampDrift::IdentityMismatch => {
                return RecordedDaemonStart::Unverified;
            }
        }
    }
    match recover_boot_timestamp_drift_runtime(
        home,
        record,
        observed_start,
        processes,
        &delay,
        STOP_DEADLINE,
        RECOVERY_POLL_INTERVAL,
    ) {
        StalledRuntimeRecovery::Recovered => RecordedDaemonStart::Recovered,
        StalledRuntimeRecovery::Unverified => RecordedDaemonStart::Unverified,
        StalledRuntimeRecovery::Failed => RecordedDaemonStart::Failed,
    }
}

fn poll_limit(deadline: Duration, interval: Duration) -> usize {
    let deadline_millis = deadline.as_millis();
    let interval_millis = interval.as_millis().max(1);
    deadline_millis.div_ceil(interval_millis).max(1) as usize
}
