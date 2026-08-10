pub mod artifacts;
pub mod discovery;
pub mod health;
pub mod mutation;
pub mod probe;
pub mod provenance;
pub mod provision;
pub mod quiesce;
pub mod readiness;
pub mod resolver;
pub mod supervisor;
pub mod update_lifecycle;

pub(crate) const PROCESS_START_TOLERANCE_SECONDS: i64 = 2;

pub use discovery::{DaemonRecord, Discovery};
pub use probe::{BoundDaemonIdentity, Identity};
pub use resolver::{BinarySource, Resolution, RuntimeResolver};
