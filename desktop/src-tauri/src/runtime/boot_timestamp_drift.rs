use chrono::{DateTime, Utc};

use crate::home::CompozyHome;

use super::discovery::{DaemonRecord, ProcessTable};
use super::probe::{BoundDaemonIdentity, Identity, IdentityProbe};
use super::provenance;
use super::supervisor::READY_DEADLINE;

pub enum BoundBootTimestampDrift {
    Attached(Box<BoundDaemonIdentity>),
    Unreachable,
    NotOwned,
    IdentityMismatch,
}

pub(crate) fn is_bounded_forward_boot_timestamp_drift(
    record: &DaemonRecord,
    observed: DateTime<Utc>,
) -> bool {
    record
        .started_at
        .signed_duration_since(observed)
        .to_std()
        .is_ok_and(|drift| drift <= READY_DEADLINE)
}

pub fn bound_app_owned_runtime_with_boot_timestamp_drift(
    home: &CompozyHome,
    processes: &dyn ProcessTable,
    probe: &dyn IdentityProbe,
    record: &DaemonRecord,
    observed_start: DateTime<Utc>,
) -> BoundBootTimestampDrift {
    if !is_bounded_forward_boot_timestamp_drift(record, observed_start) {
        return BoundBootTimestampDrift::IdentityMismatch;
    }
    if processes.start_time(record.pid) != Some(observed_start) {
        return BoundBootTimestampDrift::IdentityMismatch;
    }
    if !processes
        .executable(record.pid)
        .is_some_and(|executable| provenance::owns_executable(home, &executable))
    {
        return BoundBootTimestampDrift::NotOwned;
    }
    match probe.probe(record, None) {
        Identity::Compozy(identity) if identity.record == *record => {
            BoundBootTimestampDrift::Attached(identity)
        }
        Identity::Compozy(_) => BoundBootTimestampDrift::IdentityMismatch,
        Identity::Unreachable => BoundBootTimestampDrift::Unreachable,
        Identity::Foreign => BoundBootTimestampDrift::IdentityMismatch,
    }
}

#[cfg(test)]
#[path = "boot_timestamp_drift_tests.rs"]
mod tests;
