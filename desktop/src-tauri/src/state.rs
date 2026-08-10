use semver::{Version, VersionReq};
use serde::{Deserialize, Serialize};
use url::Url;

use crate::errors::ShellError;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "stage", rename_all = "snake_case")]
pub enum ProvisionStage {
    Download { pct: u8 },
    Verify,
    Install,
    Start,
    Ready,
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum UpdateTarget {
    App,
    Runtime,
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum DisconnectCause {
    RuntimeDown,
    HealthCheckFailed,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "state", rename_all = "snake_case")]
pub enum ShellState {
    Resolving,
    Provisioning {
        #[serde(flatten)]
        stage: ProvisionStage,
    },
    Starting {
        attempt: u8,
    },
    Attaching,
    Product {
        origin: Url,
        owned: bool,
    },
    Updating {
        target: UpdateTarget,
    },
    Disconnected {
        cause: DisconnectCause,
    },
    Skew {
        runtime: Version,
        needed: VersionReq,
        newer: bool,
    },
    #[serde(rename = "error")]
    ShellError {
        error: ShellError,
    },
}
