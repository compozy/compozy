use std::fs;
use std::path::Path;
use std::sync::{Arc, Mutex};

use chrono::{DateTime, Utc};
use semver::Version;
use serde::{Deserialize, Serialize};
use thiserror::Error;

pub use super::app_update_tauri::TauriAppUpdateBackend;
use super::app_update_tauri::remove_intent;

const MANUAL_FALLBACK_THRESHOLD: usize = 3;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AppUpdateEvent {
    Disabled,
    UpToDate {
        checked_at: DateTime<Utc>,
    },
    Ready {
        version: String,
        checked_at: DateTime<Utc>,
    },
    Failed {
        checked_at: DateTime<Utc>,
        manual_fallback: bool,
        message: String,
    },
    Applying {
        version: String,
    },
}

#[derive(Debug, Error)]
pub enum AppUpdaterError {
    #[error("the update feed has no release")]
    FeedNotFound,
    #[error("the update feed is invalid")]
    FeedInvalid,
    #[error("the update feed or artifact could not be reached")]
    Network,
    #[error("the update artifact signature is invalid")]
    InvalidSignature,
    #[error("the update could not be installed")]
    Install,
    #[error("the update requires explicit restart consent")]
    ConsentRequired,
    #[error("no app update is ready")]
    NotReady,
    #[error("the update intent could not be persisted")]
    Intent(#[source] std::io::Error),
}

pub trait PendingAppUpdate: Send {
    fn version(&self) -> &str;
    fn install(self: Box<Self>) -> Result<(), AppUpdaterError>;
}

pub trait AppUpdateBackend: Send + Sync {
    fn check_and_download(
        &self,
        public_key: &str,
    ) -> Result<Option<Box<dyn PendingAppUpdate>>, AppUpdaterError>;
}

pub struct AppUpdater {
    backend: Arc<dyn AppUpdateBackend>,
    current_key: String,
    previous_key: Option<String>,
    pending: Mutex<Option<Box<dyn PendingAppUpdate>>>,
    consecutive_failures: Mutex<usize>,
}

impl AppUpdater {
    pub fn new(
        backend: Arc<dyn AppUpdateBackend>,
        current_key: impl Into<String>,
        previous_key: Option<String>,
    ) -> Self {
        Self {
            backend,
            current_key: current_key.into(),
            previous_key: previous_key.filter(|key| !key.trim().is_empty()),
            pending: Mutex::new(None),
            consecutive_failures: Mutex::new(0),
        }
    }

    pub fn check(&self, enabled: bool) -> AppUpdateEvent {
        if !enabled {
            self.clear_pending();
            return AppUpdateEvent::Disabled;
        }
        let checked_at = Utc::now();
        let result = self.backend.check_and_download(&self.current_key);
        let result = match result {
            Err(AppUpdaterError::InvalidSignature) => self
                .previous_key
                .as_deref()
                .map_or(result, |key| self.backend.check_and_download(key)),
            other => other,
        };
        match result {
            Ok(Some(update)) => {
                let version = update.version().to_owned();
                *self
                    .pending
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner()) = Some(update);
                *self
                    .consecutive_failures
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner()) = 0;
                AppUpdateEvent::Ready {
                    version,
                    checked_at,
                }
            }
            Ok(None) => {
                self.clear_pending();
                *self
                    .consecutive_failures
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner()) = 0;
                AppUpdateEvent::UpToDate { checked_at }
            }
            Err(error) => {
                self.clear_pending();
                let mut failures = self
                    .consecutive_failures
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner());
                *failures += 1;
                AppUpdateEvent::Failed {
                    checked_at,
                    manual_fallback: *failures >= MANUAL_FALLBACK_THRESHOLD,
                    message: error.to_string(),
                }
            }
        }
    }

    pub fn apply(&self, consented: bool) -> Result<AppUpdateEvent, AppUpdaterError> {
        if !consented {
            return Err(AppUpdaterError::ConsentRequired);
        }
        let update = self
            .pending
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .take()
            .ok_or(AppUpdaterError::NotReady)?;
        let version = update.version().to_owned();
        update.install()?;
        Ok(AppUpdateEvent::Applying { version })
    }

    pub fn ready_version(&self) -> Option<String> {
        self.pending
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .as_ref()
            .map(|update| update.version().to_owned())
    }

    fn clear_pending(&self) {
        drop(
            self.pending
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .take(),
        );
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct AppUpdateIntent {
    pub current_version: String,
    pub target_version: String,
    pub consented: bool,
    pub started_at: DateTime<Utc>,
}

pub fn recover_intent(
    path: &Path,
    current_version: &str,
    _now: DateTime<Utc>,
) -> Result<Option<AppUpdateEvent>, AppUpdaterError> {
    let bytes = match fs::read(path) {
        Ok(bytes) => bytes,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(AppUpdaterError::Intent(error)),
    };
    let intent: AppUpdateIntent = serde_json::from_slice(&bytes).map_err(|error| {
        AppUpdaterError::Intent(std::io::Error::new(std::io::ErrorKind::InvalidData, error))
    })?;
    if !intent.consented {
        return Err(AppUpdaterError::Intent(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            "update intent lacks restart consent",
        )));
    }
    let target_reached = current_version == intent.target_version
        || Version::parse(current_version)
            .ok()
            .zip(Version::parse(&intent.target_version).ok())
            .is_some_and(|(current, target)| current >= target);
    if target_reached {
        remove_intent(path)?;
        return Ok(None);
    }
    Ok(Some(AppUpdateEvent::Failed {
        checked_at: intent.started_at,
        manual_fallback: true,
        message:
            "The app update did not restart successfully. Download the current release manually."
                .to_owned(),
    }))
}

#[cfg(test)]
mod tests {
    use std::collections::VecDeque;

    #[cfg(target_os = "macos")]
    use super::super::app_update_tauri::with_macos_backup;
    use super::super::app_update_tauri::write_intent;
    use super::*;

    enum FakeResponse {
        UpToDate,
        Ready(&'static str),
        Failed(AppUpdaterError),
    }

    struct FakeBackend {
        responses: Mutex<VecDeque<FakeResponse>>,
        keys: Mutex<Vec<String>>,
        installed: Arc<Mutex<Vec<String>>>,
    }

    impl FakeBackend {
        fn new(responses: Vec<FakeResponse>) -> Self {
            Self {
                responses: Mutex::new(responses.into()),
                keys: Mutex::new(Vec::new()),
                installed: Arc::new(Mutex::new(Vec::new())),
            }
        }

        fn keys(&self) -> Vec<String> {
            self.keys
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .clone()
        }
    }

    impl AppUpdateBackend for FakeBackend {
        fn check_and_download(
            &self,
            public_key: &str,
        ) -> Result<Option<Box<dyn PendingAppUpdate>>, AppUpdaterError> {
            self.keys
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .push(public_key.to_owned());
            match self
                .responses
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .pop_front()
                .expect("fixture response exists")
            {
                FakeResponse::UpToDate => Ok(None),
                FakeResponse::Ready(version) => Ok(Some(Box::new(FakePending {
                    version: version.to_owned(),
                    installed: Arc::clone(&self.installed),
                }))),
                FakeResponse::Failed(error) => Err(error),
            }
        }
    }

    struct FakePending {
        version: String,
        installed: Arc<Mutex<Vec<String>>>,
    }

    impl PendingAppUpdate for FakePending {
        fn version(&self) -> &str {
            &self.version
        }

        fn install(self: Box<Self>) -> Result<(), AppUpdaterError> {
            self.installed
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .push(self.version);
            Ok(())
        }
    }

    #[test]
    fn should_offer_downloaded_update_and_apply_only_after_restart_consent() {
        let backend = Arc::new(FakeBackend::new(vec![FakeResponse::Ready("0.4.0")]));
        let updater = AppUpdater::new(backend.clone(), "current", None);
        let event = updater.check(true);
        assert!(matches!(event, AppUpdateEvent::Ready { version, .. } if version == "0.4.0"));
        assert!(matches!(
            updater.apply(false),
            Err(AppUpdaterError::ConsentRequired)
        ));
        assert!(matches!(
            updater.apply(true),
            Ok(AppUpdateEvent::Applying { version }) if version == "0.4.0"
        ));
        assert_eq!(
            backend
                .installed
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .as_slice(),
            ["0.4.0"]
        );
    }

    #[test]
    fn should_retry_invalid_signature_with_previous_pinned_key_only() {
        let backend = Arc::new(FakeBackend::new(vec![
            FakeResponse::Failed(AppUpdaterError::InvalidSignature),
            FakeResponse::Ready("0.4.0"),
        ]));
        let updater = AppUpdater::new(backend.clone(), "current", Some("previous".to_owned()));
        assert!(matches!(updater.check(true), AppUpdateEvent::Ready { .. }));
        assert_eq!(backend.keys(), ["current", "previous"]);
    }

    #[test]
    fn should_distinguish_up_to_date_disabled_network_and_feed_not_found_states() {
        let disabled_backend = Arc::new(FakeBackend::new(vec![FakeResponse::Ready("0.4.0")]));
        let disabled = AppUpdater::new(disabled_backend.clone(), "current", None);
        assert!(matches!(disabled.check(true), AppUpdateEvent::Ready { .. }));
        assert_eq!(disabled.check(false), AppUpdateEvent::Disabled);
        assert_eq!(disabled_backend.keys(), ["current"]);
        assert!(matches!(
            disabled.apply(true),
            Err(AppUpdaterError::NotReady)
        ));

        let up_to_date = AppUpdater::new(
            Arc::new(FakeBackend::new(vec![FakeResponse::UpToDate])),
            "current",
            None,
        );
        assert!(matches!(
            up_to_date.check(true),
            AppUpdateEvent::UpToDate { .. }
        ));

        for error in [
            AppUpdaterError::Network,
            AppUpdaterError::FeedNotFound,
            AppUpdaterError::FeedInvalid,
        ] {
            let updater = AppUpdater::new(
                Arc::new(FakeBackend::new(vec![FakeResponse::Failed(error)])),
                "current",
                None,
            );
            assert!(matches!(
                updater.check(true),
                AppUpdateEvent::Failed {
                    manual_fallback: false,
                    ..
                }
            ));
        }
    }

    #[test]
    fn should_escalate_repeated_failures_without_making_partial_update_applyable() {
        let updater = AppUpdater::new(
            Arc::new(FakeBackend::new(vec![
                FakeResponse::Failed(AppUpdaterError::Network),
                FakeResponse::Failed(AppUpdaterError::Network),
                FakeResponse::Failed(AppUpdaterError::Network),
            ])),
            "current",
            None,
        );
        for expected_fallback in [false, false, true] {
            assert!(matches!(
                updater.check(true),
                AppUpdateEvent::Failed { manual_fallback, .. }
                    if manual_fallback == expected_fallback
            ));
        }
        assert!(matches!(
            updater.apply(true),
            Err(AppUpdaterError::NotReady)
        ));

        let stale = AppUpdater::new(
            Arc::new(FakeBackend::new(vec![
                FakeResponse::Ready("0.4.0"),
                FakeResponse::Failed(AppUpdaterError::InvalidSignature),
            ])),
            "current",
            None,
        );
        assert!(matches!(stale.check(true), AppUpdateEvent::Ready { .. }));
        assert!(matches!(stale.check(true), AppUpdateEvent::Failed { .. }));
        assert!(matches!(stale.apply(true), Err(AppUpdaterError::NotReady)));
    }

    #[test]
    fn should_surface_watchdog_fallback_until_target_version_launches() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("intent.json");
        let started_at = Utc::now() - chrono::Duration::minutes(1);
        write_intent(
            &path,
            &AppUpdateIntent {
                current_version: "0.3.0".to_owned(),
                target_version: "0.4.0".to_owned(),
                consented: true,
                started_at,
            },
        )
        .expect("intent writes");
        assert!(matches!(
            recover_intent(&path, "0.3.0", Utc::now()),
            Ok(Some(AppUpdateEvent::Failed {
                manual_fallback: true,
                ..
            }))
        ));
        assert!(
            recover_intent(&path, "0.4.0", Utc::now())
                .expect("successful launch recovers")
                .is_none()
        );
        assert!(!path.exists());

        write_intent(
            &path,
            &AppUpdateIntent {
                current_version: "0.3.0".to_owned(),
                target_version: "0.4.0".to_owned(),
                consented: true,
                started_at,
            },
        )
        .expect("roll-forward intent writes");
        assert!(
            recover_intent(&path, "0.5.0", Utc::now())
                .expect("newer launch recovers")
                .is_none()
        );
        assert!(!path.exists());
    }

    #[test]
    fn should_fail_a_stale_relaunch_without_waiting_for_an_unobserved_timer() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("intent.json");
        let started_at = Utc::now();
        write_intent(
            &path,
            &AppUpdateIntent {
                current_version: "0.3.0".to_owned(),
                target_version: "0.4.0".to_owned(),
                consented: true,
                started_at,
            },
        )
        .expect("intent writes");

        assert!(matches!(
            recover_intent(&path, "0.3.0", started_at),
            Ok(Some(AppUpdateEvent::Failed {
                manual_fallback: true,
                ..
            }))
        ));
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn should_restore_own_app_bundle_when_macos_install_fails() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let bundle = directory.path().join("CompozyOS.app");
        let executable = bundle.join("Contents/MacOS/CompozyOS");
        fs::create_dir_all(executable.parent().expect("executable has parent"))
            .expect("bundle creates");
        fs::write(&executable, b"previous app").expect("previous app writes");

        let result = with_macos_backup(&bundle, || {
            fs::write(&executable, b"broken app").map_err(|_| AppUpdaterError::Install)?;
            Err(AppUpdaterError::Install)
        });
        assert!(matches!(result, Err(AppUpdaterError::Install)));
        assert_eq!(
            fs::read(executable).expect("restored app reads"),
            b"previous app"
        );
    }
}
