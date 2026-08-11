pub mod artifacts;
pub mod boot_timestamp_drift;
pub mod control_server_startup;
pub mod discovery;
pub mod health;
pub mod mutation;
pub mod probe;
pub mod provenance;
pub mod provision;
pub mod quiesce;
pub mod readiness;
pub mod recorded_start;
pub mod resolver;
pub mod startup_failure;
pub mod supervisor;
pub mod update_cadence;
pub mod update_lifecycle;
pub mod update_recovery;

pub(crate) const PROCESS_START_TOLERANCE_SECONDS: i64 = 2;

pub use discovery::{DaemonRecord, Discovery};
pub use probe::{BoundDaemonIdentity, Identity};
pub use resolver::{BinarySource, Resolution, RuntimeResolver};
