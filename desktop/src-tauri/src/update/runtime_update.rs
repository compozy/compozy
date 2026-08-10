use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::time::Duration;

use chrono::Utc;
use reqwest::blocking::Client;
use semver::Version;
use serde::Deserialize;
use thiserror::Error;
use url::Url;

use crate::home::CompozyHome;
use crate::runtime::artifacts::current_target;
use crate::runtime::discovery::ProcessTable;
use crate::runtime::mutation::{
    JournalPhase, MutationError, RecoveryAction, UpdateJournal, UpdateLock, classify_recovery,
    clear_journal, finalize_swap, read_journal, rollback_swap, swap_binary, verify_ownership,
    write_journal,
};
use crate::runtime::provenance::{self, ProvenanceRecord};
use crate::runtime::provision::{ProvisionError, StagedRuntime, stage_update};
use crate::runtime::quiesce::{QuiesceError, QuiesceGuard, QuiesceTransport};

const QUIESCE_ATTEMPTS: usize = 60;
const QUIESCE_POLL_INTERVAL: Duration = Duration::from_millis(500);

#[derive(Clone, Debug, Deserialize, Eq, PartialEq)]
pub struct RuntimeUpdateStatus {
    pub supported: bool,
    pub managed: bool,
    pub install_method: String,
    pub current_version: String,
    #[serde(default)]
    pub latest_version: String,
    pub available: bool,
    pub status: String,
    #[serde(default)]
    pub recommendation: String,
    #[serde(default)]
    pub last_error: String,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RuntimeUpdateEvent {
    Idle,
    Available {
        version: String,
    },
    Managed {
        version: String,
        recommendation: String,
    },
    Declined {
        version: String,
    },
    Applying {
        version: String,
    },
    Completed {
        version: String,
    },
    Failed {
        message: String,
    },
    RecoveryRequired {
        version: String,
    },
}

#[derive(Clone, Debug)]
pub struct RuntimeInstance {
    pub pid: u32,
    pub executable: PathBuf,
    pub origin: Url,
    pub version: Version,
}

#[derive(Debug, Error)]
pub enum RuntimeUpdaterError {
    #[error("runtime update status could not be read")]
    Status,
    #[error("runtime update is not available for an app-owned runtime")]
    NotAvailable,
    #[error("runtime update staging failed")]
    Stage(#[from] ProvisionError),
    #[error("runtime mutation failed")]
    Mutation(#[from] MutationError),
    #[error("runtime quiesce failed")]
    Quiesce(#[from] QuiesceError),
    #[error("runtime lifecycle operation failed")]
    Lifecycle,
    #[error("runtime update metadata is invalid")]
    Version,
}

pub trait RuntimeLifecycle: Send + Sync {
    fn processes(&self) -> &dyn ProcessTable;
    fn probe(&self) -> Result<RuntimeInstance, RuntimeUpdaterError>;
    fn quiesce(&self, origin: &Url) -> Result<Box<dyn QuiesceTransport>, RuntimeUpdaterError>;
    fn stop(&self, pid: u32) -> Result<(), RuntimeUpdaterError>;
    fn start_and_verify(
        &self,
        executable: &Path,
        expected: &Version,
    ) -> Result<(), RuntimeUpdaterError>;
}

pub trait RuntimeStager: Send + Sync {
    fn stage(&self, installed: &Version) -> Result<Option<StagedRuntime>, ProvisionError>;
}

pub struct ManifestRuntimeStager {
    home: CompozyHome,
    manifest_url: Url,
    public_key: String,
    target: String,
}

impl ManifestRuntimeStager {
    pub fn new(
        home: CompozyHome,
        manifest_url: Url,
        public_key: impl Into<String>,
    ) -> Result<Self, RuntimeUpdaterError> {
        Ok(Self {
            home,
            manifest_url,
            public_key: public_key.into(),
            target: current_target()
                .map_err(|_| RuntimeUpdaterError::Version)?
                .to_owned(),
        })
    }
}

impl RuntimeStager for ManifestRuntimeStager {
    fn stage(&self, installed: &Version) -> Result<Option<StagedRuntime>, ProvisionError> {
        stage_update(
            &self.home,
            &self.manifest_url,
            &self.public_key,
            &self.target,
            installed,
        )
    }
}

pub struct RuntimeUpdater {
    home: CompozyHome,
    status_url: Url,
    client: Client,
    stager: Box<dyn RuntimeStager>,
    lifecycle: Box<dyn RuntimeLifecycle>,
    app_version: String,
    channel: String,
    declined: Mutex<Option<String>>,
}

impl RuntimeUpdater {
    pub fn new(
        home: CompozyHome,
        origin: &Url,
        stager: Box<dyn RuntimeStager>,
        lifecycle: Box<dyn RuntimeLifecycle>,
        app_version: impl Into<String>,
        channel: impl Into<String>,
    ) -> Result<Self, RuntimeUpdaterError> {
        let status_url = origin
            .join("api/settings/update")
            .map_err(|_| RuntimeUpdaterError::Status)?;
        let client = Client::builder()
            .timeout(Duration::from_secs(5))
            .build()
            .map_err(|_| RuntimeUpdaterError::Status)?;
        Ok(Self {
            home,
            status_url,
            client,
            stager,
            lifecycle,
            app_version: app_version.into(),
            channel: channel.into(),
            declined: Mutex::new(None),
        })
    }

    pub fn check(&self) -> Result<RuntimeUpdateEvent, RuntimeUpdaterError> {
        if let Some(journal) = read_journal(&self.home.update_journal)?
            && classify_recovery(&journal) == RecoveryAction::RecoveryRequired
        {
            return Ok(RuntimeUpdateEvent::RecoveryRequired {
                version: journal.to_version.to_string(),
            });
        }
        let status = self.read_status()?;
        if !status.available {
            return Ok(RuntimeUpdateEvent::Idle);
        }
        if status.install_method != "desktop-app" {
            return Ok(RuntimeUpdateEvent::Managed {
                version: status.latest_version,
                recommendation: status.recommendation,
            });
        }
        let instance = self.lifecycle.probe()?;
        if verify_ownership(&self.home, &instance.executable).is_err() {
            return Ok(RuntimeUpdateEvent::Managed {
                version: status.latest_version,
                recommendation:
                    "This runtime is managed outside the app. Update it with its installer."
                        .to_owned(),
            });
        }
        Ok(RuntimeUpdateEvent::Available {
            version: status.latest_version,
        })
    }

    pub fn apply(&self, consented: bool) -> Result<RuntimeUpdateEvent, RuntimeUpdaterError> {
        let status = self.read_status()?;
        if !status.available || status.install_method != "desktop-app" {
            return Err(RuntimeUpdaterError::NotAvailable);
        }
        if !consented {
            *self
                .declined
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner()) =
                Some(status.latest_version.clone());
            return Ok(RuntimeUpdateEvent::Declined {
                version: status.latest_version,
            });
        }
        let current =
            Version::parse(&status.current_version).map_err(|_| RuntimeUpdaterError::Version)?;
        let lock = UpdateLock::acquire(&self.home.update_lock, self.lifecycle.processes())?;
        let staged = self
            .stager
            .stage(&current)?
            .ok_or(RuntimeUpdaterError::NotAvailable)?;
        self.apply_staged(current, staged, lock)
    }

    pub fn was_declined(&self, version: &str) -> bool {
        self.declined
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .as_deref()
            == Some(version)
    }

    fn apply_staged(
        &self,
        current: Version,
        staged: StagedRuntime,
        lock: UpdateLock,
    ) -> Result<RuntimeUpdateEvent, RuntimeUpdaterError> {
        let instance = self.lifecycle.probe()?;
        verify_ownership(&self.home, &instance.executable)?;
        if instance.version != current {
            return Err(RuntimeUpdaterError::Version);
        }
        let transport = self.lifecycle.quiesce(&instance.origin)?;
        let mut quiesce =
            QuiesceGuard::prepare(transport.as_ref(), QUIESCE_ATTEMPTS, QUIESCE_POLL_INTERVAL)?;
        let mut journal = UpdateJournal::staged(
            current,
            staged.verified.version.clone(),
            staged.binary_path.clone(),
            staged.verified.schema_heads.clone(),
        );
        write_journal(&self.home.update_journal, &journal)?;
        quiesce.revalidate()?;
        self.lifecycle.stop(instance.pid)?;
        quiesce.commit();
        journal.advance(JournalPhase::Stopped)?;
        write_journal(&self.home.update_journal, &journal)?;
        let swap = match swap_binary(&self.home, &staged.binary_path) {
            Ok(swap) => swap,
            Err(failure) => {
                if let Some(swap) = failure.committed
                    && rollback_swap(&self.home, &swap).is_err()
                {
                    return Ok(RuntimeUpdateEvent::RecoveryRequired {
                        version: journal.to_version.to_string(),
                    });
                }
                self.lifecycle
                    .start_and_verify(&self.home.app_binary, &journal.from_version)?;
                clear_journal(&self.home.update_journal)?;
                return Err(failure.error.into());
            }
        };
        journal.advance(JournalPhase::Swapped)?;
        write_journal(&self.home.update_journal, &journal)?;
        let digest = provenance::sha256_file(&self.home.app_binary)
            .map_err(|_| RuntimeUpdaterError::Lifecycle)?;
        let marker = ProvenanceRecord::desktop(
            &self.app_version,
            staged.verified.version.to_string(),
            &self.channel,
            Utc::now(),
            digest,
        );
        if let Err(error) = provenance::write_atomic(&self.home.provenance, &marker) {
            crate::logging::error(format!("write runtime provenance after update: {error}"));
            rollback_swap(&self.home, &swap)?;
            self.lifecycle
                .start_and_verify(&self.home.app_binary, &journal.from_version)?;
            clear_journal(&self.home.update_journal)?;
            return Err(RuntimeUpdaterError::Lifecycle);
        }
        journal.advance(JournalPhase::Migrating)?;
        write_journal(&self.home.update_journal, &journal)?;
        if let Err(error) = self
            .lifecycle
            .start_and_verify(&self.home.app_binary, &staged.verified.version)
        {
            crate::logging::error(format!("start migrated runtime: {error}"));
            return Ok(RuntimeUpdateEvent::RecoveryRequired {
                version: staged.verified.version.to_string(),
            });
        }
        journal.advance(JournalPhase::Started)?;
        write_journal(&self.home.update_journal, &journal)?;
        finalize_swap(&swap)?;
        journal.advance(JournalPhase::Done)?;
        write_journal(&self.home.update_journal, &journal)?;
        clear_journal(&self.home.update_journal)?;
        lock.release()?;
        Ok(RuntimeUpdateEvent::Completed {
            version: staged.verified.version.to_string(),
        })
    }

    fn read_status(&self) -> Result<RuntimeUpdateStatus, RuntimeUpdaterError> {
        let response = self
            .client
            .get(self.status_url.clone())
            .send()
            .map_err(|_| RuntimeUpdaterError::Status)?;
        if !response.status().is_success() {
            return Err(RuntimeUpdaterError::Status);
        }
        response.json().map_err(|_| RuntimeUpdaterError::Status)
    }
}

#[cfg(test)]
mod tests {
    use std::collections::{BTreeMap, HashMap, VecDeque};
    use std::fs;
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::thread::JoinHandle;

    use crate::runtime::artifacts::{PlatformArtifact, VerifiedArtifact};
    use crate::runtime::provenance;
    use crate::runtime::quiesce::DrainStatus;

    use super::*;

    struct StatusServer {
        origin: Url,
        thread: Option<JoinHandle<()>>,
    }

    impl StatusServer {
        fn start(body: &str, requests: usize) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").expect("status server binds");
            let address = listener.local_addr().expect("status address reads");
            let body = body.to_owned();
            let thread = std::thread::spawn(move || {
                for _request in 0..requests {
                    let (mut stream, _) = listener.accept().expect("status request accepts");
                    let mut request = [0_u8; 4096];
                    let read = stream.read(&mut request).expect("status request reads");
                    assert!(request[..read].starts_with(b"GET /api/settings/update"));
                    let response = format!(
                        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                        body.len(),
                        body
                    );
                    stream
                        .write_all(response.as_bytes())
                        .expect("status response writes");
                }
            });
            Self {
                origin: Url::parse(&format!("http://{address}/")).expect("origin parses"),
                thread: Some(thread),
            }
        }
    }

    impl Drop for StatusServer {
        fn drop(&mut self) {
            if let Some(thread) = self.thread.take() {
                thread.join().expect("status server joins");
            }
        }
    }

    struct FakeProcesses {
        starts: HashMap<u32, chrono::DateTime<Utc>>,
    }

    impl ProcessTable for FakeProcesses {
        fn start_time(&self, pid: u32) -> Option<chrono::DateTime<Utc>> {
            self.starts.get(&pid).copied()
        }

        fn executable(&self, _pid: u32) -> Option<PathBuf> {
            None
        }

        fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
            descendant == ancestor
        }
    }

    struct FakeLifecycle {
        processes: FakeProcesses,
        instance: RuntimeInstance,
        drain: Mutex<VecDeque<DrainStatus>>,
        stops: AtomicUsize,
        starts: AtomicUsize,
        fail_start: bool,
    }

    impl RuntimeLifecycle for FakeLifecycle {
        fn processes(&self) -> &dyn ProcessTable {
            &self.processes
        }

        fn probe(&self) -> Result<RuntimeInstance, RuntimeUpdaterError> {
            Ok(self.instance.clone())
        }

        fn quiesce(&self, _origin: &Url) -> Result<Box<dyn QuiesceTransport>, RuntimeUpdaterError> {
            Ok(Box::new(FakeQuiesce {
                statuses: Mutex::new(
                    self.drain
                        .lock()
                        .unwrap_or_else(|poisoned| poisoned.into_inner())
                        .clone(),
                ),
            }))
        }

        fn stop(&self, _pid: u32) -> Result<(), RuntimeUpdaterError> {
            self.stops.fetch_add(1, Ordering::AcqRel);
            Ok(())
        }

        fn start_and_verify(
            &self,
            _executable: &Path,
            _expected: &Version,
        ) -> Result<(), RuntimeUpdaterError> {
            self.starts.fetch_add(1, Ordering::AcqRel);
            if self.fail_start {
                Err(RuntimeUpdaterError::Lifecycle)
            } else {
                Ok(())
            }
        }
    }

    struct FakeQuiesce {
        statuses: Mutex<VecDeque<DrainStatus>>,
    }

    impl QuiesceTransport for FakeQuiesce {
        fn drain(&self) -> Result<DrainStatus, QuiesceError> {
            self.statuses
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .pop_front()
                .ok_or(QuiesceError::Transport)
        }

        fn undrain(&self) -> Result<DrainStatus, QuiesceError> {
            Ok(DrainStatus {
                state: "active".to_owned(),
                admission_closed: false,
                safe_to_stop: false,
                ..safe_status()
            })
        }
    }

    struct FakeStager {
        staged: Mutex<Option<StagedRuntime>>,
        error: bool,
        calls: AtomicUsize,
    }

    impl RuntimeStager for FakeStager {
        fn stage(&self, _installed: &Version) -> Result<Option<StagedRuntime>, ProvisionError> {
            self.calls.fetch_add(1, Ordering::AcqRel);
            if self.error {
                return Err(ProvisionError::Artifact(
                    crate::runtime::artifacts::ArtifactError::DigestMismatch,
                ));
            }
            Ok(self
                .staged
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .take())
        }
    }

    fn status(method: &str, available: bool) -> String {
        serde_json::json!({
            "supported": true,
            "managed": method != "desktop-app",
            "install_method": method,
            "current_version": "0.3.0",
            "latest_version": "0.4.0",
            "available": available,
            "status": if available { "available" } else { "current" },
            "recommendation": if method == "desktop-app" { "" } else { "Update with the owning installer." },
            "last_error": ""
        })
        .to_string()
    }

    fn safe_status() -> DrainStatus {
        DrainStatus {
            state: "draining".to_owned(),
            admission_closed: true,
            active_session_executions: 0,
            active_task_claims: 0,
            authoritative_claims_settled: true,
            session_execution_settled: true,
            safe_to_stop: true,
        }
    }

    fn staged(path: PathBuf) -> StagedRuntime {
        StagedRuntime {
            verified: VerifiedArtifact {
                version: Version::parse("0.4.0").expect("version parses"),
                artifact: PlatformArtifact {
                    url: Url::parse("http://127.0.0.1/runtime.tar.gz")
                        .expect("artifact URL parses"),
                    sha256: "a".repeat(64),
                    size: 3,
                },
                schema_heads: BTreeMap::from([("global".to_owned(), 12)]),
            },
            binary_path: path,
        }
    }

    fn write_staged_binary(home: &CompozyHome) -> PathBuf {
        let path = home
            .app_binary
            .parent()
            .expect("binary has parent")
            .join(".runtime-staged");
        fs::write(&path, b"new runtime").expect("staged binary writes");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;

            fs::set_permissions(&path, fs::Permissions::from_mode(0o755))
                .expect("staged binary permissions set");
        }
        path
    }

    fn owned_home() -> (tempfile::TempDir, CompozyHome) {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        fs::create_dir_all(home.app_binary.parent().expect("binary has parent"))
            .expect("bin creates");
        fs::write(&home.app_binary, b"old runtime").expect("binary writes");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;

            fs::set_permissions(&home.app_binary, fs::Permissions::from_mode(0o755))
                .expect("binary permissions set");
        }
        let digest = provenance::sha256_file(&home.app_binary).expect("binary hashes");
        provenance::write_atomic(
            &home.provenance,
            &ProvenanceRecord::desktop("0.3.0", "0.3.0", "beta", Utc::now(), digest),
        )
        .expect("marker writes");
        (directory, home)
    }

    fn lifecycle(executable: PathBuf) -> Box<FakeLifecycle> {
        Box::new(FakeLifecycle {
            processes: FakeProcesses {
                starts: HashMap::new(),
            },
            instance: RuntimeInstance {
                pid: 42,
                executable,
                origin: Url::parse("http://127.0.0.1:2123/").expect("origin parses"),
                version: Version::parse("0.3.0").expect("version parses"),
            },
            drain: Mutex::new(VecDeque::from([safe_status(), safe_status()])),
            stops: AtomicUsize::new(0),
            starts: AtomicUsize::new(0),
            fail_start: false,
        })
    }

    #[test]
    fn should_surface_managed_recommendation_without_staging_or_writing() {
        let server = StatusServer::start(&status("homebrew", true), 1);
        let (_directory, home) = owned_home();
        let stager = FakeStager {
            staged: Mutex::new(None),
            error: false,
            calls: AtomicUsize::new(0),
        };
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(stager),
            lifecycle(home.app_binary.clone()),
            "0.3.0",
            "beta",
        )
        .expect("updater creates");
        assert_eq!(
            updater.check().expect("status checks"),
            RuntimeUpdateEvent::Managed {
                version: "0.4.0".to_owned(),
                recommendation: "Update with the owning installer.".to_owned(),
            }
        );
        assert!(!home.update_journal.exists());
    }

    #[test]
    fn should_keep_declined_owned_offer_visible_without_mutation() {
        let server = StatusServer::start(&status("desktop-app", true), 1);
        let (_directory, home) = owned_home();
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(FakeStager {
                staged: Mutex::new(None),
                error: false,
                calls: AtomicUsize::new(0),
            }),
            lifecycle(home.app_binary.clone()),
            "0.3.0",
            "beta",
        )
        .expect("updater creates");
        assert_eq!(
            updater.apply(false).expect("decline records"),
            RuntimeUpdateEvent::Declined {
                version: "0.4.0".to_owned()
            }
        );
        assert!(updater.was_declined("0.4.0"));
        assert!(!home.update_journal.exists());
    }

    #[test]
    fn should_stage_before_quiesce_and_leave_old_runtime_untouched_on_verification_failure() {
        let server = StatusServer::start(&status("desktop-app", true), 1);
        let (_directory, home) = owned_home();
        let lifecycle = lifecycle(home.app_binary.clone());
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(FakeStager {
                staged: Mutex::new(None),
                error: true,
                calls: AtomicUsize::new(0),
            }),
            lifecycle,
            "0.3.0",
            "beta",
        )
        .expect("updater creates");
        assert!(matches!(
            updater.apply(true),
            Err(RuntimeUpdaterError::Stage(_))
        ));
        assert_eq!(
            fs::read(&home.app_binary).expect("old binary reads"),
            b"old runtime"
        );
        assert!(!home.update_journal.exists());
    }

    #[test]
    fn should_apply_owned_update_inside_transaction_and_clear_journal() {
        let server = StatusServer::start(&status("desktop-app", true), 1);
        let (_directory, home) = owned_home();
        let staged_path = write_staged_binary(&home);
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(FakeStager {
                staged: Mutex::new(Some(staged(staged_path))),
                error: false,
                calls: AtomicUsize::new(0),
            }),
            lifecycle(home.app_binary.clone()),
            "0.3.0",
            "beta",
        )
        .expect("updater creates");
        assert_eq!(
            updater.apply(true).expect("owned update applies"),
            RuntimeUpdateEvent::Completed {
                version: "0.4.0".to_owned()
            }
        );
        assert_eq!(
            fs::read(&home.app_binary).expect("new binary reads"),
            b"new runtime"
        );
        assert!(!home.update_journal.exists());
        assert!(provenance::owns_executable(&home, &home.app_binary));
    }

    #[test]
    fn should_require_recovery_without_restarting_old_runtime_after_migration_begins() {
        let server = StatusServer::start(&status("desktop-app", true), 1);
        let (_directory, home) = owned_home();
        let staged_path = write_staged_binary(&home);
        let mut failing_lifecycle = lifecycle(home.app_binary.clone());
        failing_lifecycle.fail_start = true;
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(FakeStager {
                staged: Mutex::new(Some(staged(staged_path))),
                error: false,
                calls: AtomicUsize::new(0),
            }),
            failing_lifecycle,
            "0.3.0",
            "beta",
        )
        .expect("updater creates");

        assert_eq!(
            updater.apply(true).expect("failed boot becomes recovery"),
            RuntimeUpdateEvent::RecoveryRequired {
                version: "0.4.0".to_owned()
            }
        );
        assert_eq!(
            fs::read(&home.app_binary).expect("new binary remains"),
            b"new runtime"
        );
        assert_eq!(
            read_journal(&home.update_journal)
                .expect("recovery journal reads")
                .expect("recovery journal exists")
                .phase,
            JournalPhase::Migrating
        );
    }

    #[test]
    fn should_offer_recommendation_when_desktop_ownership_cannot_be_reacquired() {
        let server = StatusServer::start(&status("desktop-app", true), 1);
        let (_directory, home) = owned_home();
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(FakeStager {
                staged: Mutex::new(None),
                error: false,
                calls: AtomicUsize::new(0),
            }),
            lifecycle(home.root.join("operator/compozy")),
            "0.3.0",
            "beta",
        )
        .expect("updater creates");

        assert!(matches!(
            updater.check(),
            Ok(RuntimeUpdateEvent::Managed { recommendation, .. })
                if !recommendation.is_empty()
        ));
        assert!(!home.update_journal.exists());
    }

    #[test]
    fn should_refuse_mutation_when_post_lock_probe_changes_ownership() {
        let server = StatusServer::start(&status("desktop-app", true), 1);
        let (_directory, home) = owned_home();
        let staged_path = write_staged_binary(&home);
        let updater = RuntimeUpdater::new(
            home.clone(),
            &server.origin,
            Box::new(FakeStager {
                staged: Mutex::new(Some(staged(staged_path))),
                error: false,
                calls: AtomicUsize::new(0),
            }),
            lifecycle(home.root.join("operator/compozy")),
            "0.3.0",
            "beta",
        )
        .expect("updater creates");
        assert!(matches!(
            updater.apply(true),
            Err(RuntimeUpdaterError::Mutation(MutationError::NotOwned))
        ));
        assert_eq!(
            fs::read(&home.app_binary).expect("old binary reads"),
            b"old runtime"
        );
        assert!(!home.update_journal.exists());
    }
}
