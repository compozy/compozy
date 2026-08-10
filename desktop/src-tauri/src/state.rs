use semver::{Version, VersionReq};
use serde::{Deserialize, Serialize};
use url::Url;

use crate::errors::{ShellError, ShellErrorCode};

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

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Resolution {
    Attached { origin: Url, owned: bool },
    NeedsStart,
    NeedsProvision,
    Failed { error: ShellError },
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ShellEvent {
    Resolved(Resolution),
    ReadinessOk,
    StartFailed {
        error: ShellError,
    },
    HandshakeOk {
        origin: Url,
        owned: bool,
    },
    Skew {
        runtime: Version,
        needed: VersionReq,
        newer: bool,
    },
    HealthLost {
        cause: DisconnectCause,
    },
    ProbeOk {
        origin: Url,
        owned: bool,
    },
}

impl ShellState {
    pub fn advance(self, event: ShellEvent, log_error: ShellError) -> Self {
        match (self, event) {
            (Self::Resolving, ShellEvent::Resolved(Resolution::Attached { .. })) => Self::Attaching,
            (Self::Resolving, ShellEvent::Resolved(Resolution::NeedsStart)) => {
                Self::Starting { attempt: 1 }
            }
            (Self::Resolving, ShellEvent::Resolved(Resolution::NeedsProvision)) => {
                Self::Provisioning {
                    stage: ProvisionStage::Download { pct: 0 },
                }
            }
            (Self::Resolving, ShellEvent::Resolved(Resolution::Failed { error })) => {
                Self::ShellError { error }
            }
            (Self::Starting { .. }, ShellEvent::ReadinessOk) => Self::Attaching,
            (Self::Starting { attempt }, ShellEvent::StartFailed { error }) if attempt >= 3 => {
                Self::ShellError { error }
            }
            (Self::Starting { attempt }, ShellEvent::StartFailed { .. }) => Self::Starting {
                attempt: attempt + 1,
            },
            (Self::Attaching, ShellEvent::HandshakeOk { origin, owned }) => {
                Self::Product { origin, owned }
            }
            (
                Self::Attaching,
                ShellEvent::Skew {
                    runtime,
                    needed,
                    newer,
                },
            ) => Self::Skew {
                runtime,
                needed,
                newer,
            },
            (Self::Product { .. }, ShellEvent::HealthLost { cause }) => {
                Self::Disconnected { cause }
            }
            (Self::Disconnected { .. }, ShellEvent::ProbeOk { origin, owned }) => {
                Self::Product { origin, owned }
            }
            _ => Self::ShellError {
                error: ShellError::new(
                    ShellErrorCode::HandshakeFailed,
                    "The app reached an invalid runtime state.",
                    log_error.log_path,
                ),
            },
        }
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;

    fn log_error() -> ShellError {
        ShellError::from_code(
            ShellErrorCode::HandshakeFailed,
            PathBuf::from("/tmp/compozy.log"),
        )
    }

    fn origin() -> Url {
        Url::parse("http://localhost:2123").expect("fixture URL is valid")
    }

    #[test]
    fn should_advance_attached_resolution_through_attaching_to_product() {
        let state = ShellState::Resolving.advance(
            ShellEvent::Resolved(Resolution::Attached {
                origin: origin(),
                owned: false,
            }),
            log_error(),
        );
        assert_eq!(state, ShellState::Attaching);

        let state = state.advance(
            ShellEvent::HandshakeOk {
                origin: origin(),
                owned: false,
            },
            log_error(),
        );
        assert_eq!(
            state,
            ShellState::Product {
                origin: origin(),
                owned: false
            }
        );
    }

    #[test]
    fn should_map_needs_provision_to_initial_download_stage() {
        let state = ShellState::Resolving.advance(
            ShellEvent::Resolved(Resolution::NeedsProvision),
            log_error(),
        );
        assert_eq!(
            state,
            ShellState::Provisioning {
                stage: ProvisionStage::Download { pct: 0 }
            }
        );
    }

    #[test]
    fn should_move_ready_owned_runtime_to_attaching() {
        let state =
            ShellState::Starting { attempt: 1 }.advance(ShellEvent::ReadinessOk, log_error());
        assert_eq!(state, ShellState::Attaching);
    }

    #[test]
    fn should_surface_older_and_newer_skew() {
        for newer in [false, true] {
            let runtime = Version::new(0, 2, 0);
            let needed = VersionReq::parse(">=0.3.0").expect("fixture requirement is valid");
            let state = ShellState::Attaching.advance(
                ShellEvent::Skew {
                    runtime: runtime.clone(),
                    needed: needed.clone(),
                    newer,
                },
                log_error(),
            );
            assert_eq!(
                state,
                ShellState::Skew {
                    runtime,
                    needed,
                    newer
                }
            );
        }
    }

    #[test]
    fn should_disconnect_and_reconnect_without_app_restart() {
        let state = ShellState::Product {
            origin: origin(),
            owned: false,
        }
        .advance(
            ShellEvent::HealthLost {
                cause: DisconnectCause::RuntimeDown,
            },
            log_error(),
        );
        assert_eq!(
            state,
            ShellState::Disconnected {
                cause: DisconnectCause::RuntimeDown
            }
        );

        let state = state.advance(
            ShellEvent::ProbeOk {
                origin: origin(),
                owned: false,
            },
            log_error(),
        );
        assert!(matches!(state, ShellState::Product { .. }));
    }

    #[test]
    fn should_stop_after_third_start_failure() {
        let error = ShellError::from_code(
            ShellErrorCode::RuntimeStartFailed,
            PathBuf::from("/tmp/compozy.log"),
        );
        let state = ShellState::Starting { attempt: 3 }.advance(
            ShellEvent::StartFailed {
                error: error.clone(),
            },
            log_error(),
        );
        assert_eq!(state, ShellState::ShellError { error });
    }
}
