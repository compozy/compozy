use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use chrono::{DateTime, Utc};
use semver::VersionReq;
use tauri::{AppHandle, Manager, Wry};

use crate::app_state::AppStatePublisher;
use crate::config;
use crate::control::ControlServer;
use crate::controller::DesktopController;
use crate::errors::{ShellError, ShellErrorCode};
use crate::health_monitor;
use crate::home::CompozyHome;
use crate::links::LinkQueue;
use crate::release::ReleaseConfig;
use crate::runtime::artifacts::current_target;
use crate::runtime::discovery::{Discovery, SystemProcessTable, discover};
use crate::runtime::mutation::{
    RecoveryAction, UpdateJournal, UpdateLock, complete_recovery, prepare_recovery, read_journal,
};
use crate::runtime::probe::{
    BoundDaemonIdentity, BoundProbe, Compatibility, HttpStatusTransport, Identity, IdentityProbe,
    handshake,
};
use crate::runtime::provision::{
    ProvisionOutcome, ProvisionRequest, ResumeChoice, incomplete_stage, provision, shell_error,
};
use crate::runtime::readiness::DaemonReadiness;
use crate::runtime::resolver::{
    AttachFirstResolver, BinarySource, Resolution, RuntimeResolver, SystemInstallLocator,
};
use crate::runtime::supervisor::{StartResult, Supervisor, SystemSpawner, ThreadDelay};
use crate::runtime::update_lifecycle::SystemRuntimeLifecycle;
use crate::state::ShellState;
use crate::update::app_update::{AppUpdater, TauriAppUpdateBackend, recover_intent};
use crate::update::runtime_update::RuntimeLifecycle;
use crate::update::runtime_update::{ManifestRuntimeStager, RuntimeUpdater};
use crate::{logging, windowing};

const MINIMUM_RUNTIME: &str = ">=0.3.0";
const RESOLUTION_ATTEMPTS: usize = 4;

pub fn setup(
    app: &AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    started_at: DateTime<Utc>,
) -> Result<(), Box<dyn std::error::Error>> {
    let settings = config::load(&home.config_file, &home);
    if let Some(diagnostic) = &settings.diagnostic {
        logging::error(&diagnostic.safe_message);
    }
    let publisher =
        AppStatePublisher::new(app.clone(), home.clone(), started_at, settings.diagnostic);
    let release = ReleaseConfig::compiled(app.config());
    let app_updater = release.as_ref().map(|release| {
        let backend = Arc::new(TauriAppUpdateBackend::new(
            app.clone(),
            release.app_endpoint.clone(),
            home.app_update_intent.clone(),
        ));
        Arc::new(AppUpdater::new(
            backend,
            release.current_public_key.clone(),
            release.previous_public_key.clone(),
        ))
    });
    let updates_enabled = settings.consumer_enabled;
    let controller = Arc::new(DesktopController::new(
        app.clone(),
        home.clone(),
        Arc::clone(&links),
        publisher.clone(),
        app_updater,
        updates_enabled,
    ));
    match recover_intent(
        &home.app_update_intent,
        env!("CARGO_PKG_VERSION"),
        Utc::now(),
    ) {
        Ok(Some(event)) => controller.record_app_event(&event),
        Ok(None) => {}
        Err(error) => {
            logging::error(format!("recover app update intent: {error}"));
            controller.record_app_event(&crate::update::app_update::AppUpdateEvent::Failed {
                checked_at: Utc::now(),
                manual_fallback: true,
                message: error.to_string(),
            });
        }
    }
    let server = ControlServer::start(&home.app_socket, controller.clone())?;
    let coordinator = Arc::new(ShellCoordinator {
        app: app.clone(),
        home,
        links,
        publisher,
        controller: Arc::clone(&controller),
        release,
        running: AtomicBool::new(false),
    });
    let retry_coordinator = Arc::downgrade(&coordinator);
    controller.set_retry(Arc::new(move || {
        if let Some(coordinator) = retry_coordinator.upgrade() {
            std::thread::spawn(move || coordinator.run(true));
        }
    }));

    app.manage(server);
    app.manage(Arc::clone(&controller));
    app.manage(Arc::clone(&coordinator));

    let startup = Arc::clone(&coordinator);
    let interval = settings.config.update_check_interval;
    std::thread::spawn(move || {
        if updates_enabled {
            startup.controller.check_app_update();
        }
        Arc::clone(&startup).run(false);
        if updates_enabled {
            spawn_update_cadence(startup.controller.clone(), interval);
        }
    });
    Ok(())
}

struct ShellCoordinator {
    app: AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    publisher: AppStatePublisher,
    controller: Arc<DesktopController>,
    release: Option<ReleaseConfig>,
    running: AtomicBool,
}

impl ShellCoordinator {
    fn run(self: Arc<Self>, resume_recovery: bool) {
        if self
            .running
            .compare_exchange(false, true, Ordering::AcqRel, Ordering::Acquire)
            .is_err()
        {
            return;
        }
        self.controller.set_runtime_updater(None);
        self.publisher.publish(ShellState::Resolving);
        if !self.recover_runtime(resume_recovery) {
            self.running.store(false, Ordering::Release);
            return;
        }
        self.resolve_loop();
        self.running.store(false, Ordering::Release);
    }

    fn recover_runtime(&self, resume_recovery: bool) -> bool {
        match read_journal(&self.home.update_journal) {
            Ok(None) => return true,
            Ok(Some(_journal)) => {}
            Err(error) => {
                logging::error(format!("read runtime recovery journal: {error}"));
                self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
                return false;
            }
        }
        let processes = SystemProcessTable;
        let lock = match UpdateLock::acquire(&self.home.update_lock, &processes) {
            Ok(lock) => lock,
            Err(error) => {
                logging::error(format!("acquire runtime recovery lock: {error}"));
                self.publish_error(ShellErrorCode::UpdateLockHeld);
                return false;
            }
        };
        let plan = match prepare_recovery(&self.home) {
            Ok(plan) => plan,
            Err(error) => {
                logging::error(format!("prepare runtime recovery: {error}"));
                self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
                return false;
            }
        };
        let Some(plan) = plan else {
            if let Err(error) = lock.release() {
                logging::error(format!("release empty runtime recovery lock: {error}"));
            }
            return true;
        };
        let should_resume = plan.action == RecoveryAction::ResumeNewRuntime
            || (plan.action == RecoveryAction::RecoveryRequired && resume_recovery);
        if should_resume {
            let Some(journal) = plan.journal else {
                self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
                return false;
            };
            return self.resume_runtime(journal, lock);
        }
        if let Err(error) = lock.release() {
            logging::error(format!("release runtime recovery lock: {error}"));
            self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
            return false;
        }
        if plan.action == RecoveryAction::RecoveryRequired {
            self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
            return false;
        }
        true
    }

    fn resume_runtime(&self, journal: UpdateJournal, lock: UpdateLock) -> bool {
        let lifecycle = SystemRuntimeLifecycle::new(self.home.clone());
        if let Err(error) = lifecycle.start_and_verify(&self.home.app_binary, &journal.to_version) {
            logging::error(format!("resume updated runtime: {error}"));
            self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
            return false;
        }
        if let Err(error) = complete_recovery(&self.home, journal) {
            logging::error(format!("complete runtime recovery: {error}"));
            self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
            return false;
        }
        if let Err(error) = lock.release() {
            logging::error(format!("release completed runtime recovery lock: {error}"));
            self.publish_error(ShellErrorCode::MigrationRecoveryRequired);
            return false;
        }
        self.publisher.publish(ShellState::Resolving);
        true
    }

    fn resolve_loop(&self) {
        for _attempt in 0..RESOLUTION_ATTEMPTS {
            let processes = SystemProcessTable;
            let probe = BoundProbe::new(HttpStatusTransport, Duration::from_secs(2));
            let installs = SystemInstallLocator;
            let resolver = AttachFirstResolver {
                processes: &processes,
                probe: &probe,
                installs: &installs,
            };
            match resolver.resolve(&self.home) {
                Resolution::Attached { identity, owned } => {
                    self.finish_attach(identity, owned);
                    return;
                }
                Resolution::NeedsStart { binary, source } => {
                    self.start_runtime(&binary, source == BinarySource::AppOwned);
                    return;
                }
                Resolution::NeedsProvision => match self.provision_runtime() {
                    Ok(ProvisionOutcome::Installed { .. } | ProvisionOutcome::AttachExternally) => {
                        self.publisher.publish(ShellState::Resolving);
                    }
                    Ok(ProvisionOutcome::UpToDate) => {
                        self.publish_error(ShellErrorCode::RuntimeStartFailed);
                        return;
                    }
                    Err(error) => {
                        self.publisher.publish(ShellState::ShellError {
                            error: shell_error(&self.home, &error),
                        });
                        return;
                    }
                },
                Resolution::Failed { error } => {
                    self.publisher.publish(ShellState::ShellError { error });
                    return;
                }
            }
        }
        self.publish_error(ShellErrorCode::RuntimeStartFailed);
    }

    fn provision_runtime(
        &self,
    ) -> Result<ProvisionOutcome, crate::runtime::provision::ProvisionError> {
        let release = self.release.as_ref().ok_or({
            crate::runtime::provision::ProvisionError::Artifact(
                crate::runtime::artifacts::ArtifactError::Network,
            )
        })?;
        let target =
            current_target().map_err(crate::runtime::provision::ProvisionError::Artifact)?;
        let request = ProvisionRequest {
            home: &self.home,
            manifest_url: &release.runtime_manifest,
            public_key: &release.current_public_key,
            target,
            installed: None,
            app_version: env!("CARGO_PKG_VERSION"),
            channel: &release.channel,
            resume: if incomplete_stage(&self.home) {
                ResumeChoice::Continue
            } else {
                ResumeChoice::StartOver
            },
        };
        provision(&request, &|| live_bound_runtime(&self.home), &mut |stage| {
            self.publisher.publish(ShellState::Provisioning { stage })
        })
    }

    fn start_runtime(&self, binary: &std::path::Path, owned: bool) {
        self.publisher.publish(ShellState::Starting { attempt: 1 });
        let processes = SystemProcessTable;
        let probe = BoundProbe::new(HttpStatusTransport, Duration::from_secs(2));
        let readiness = DaemonReadiness {
            daemon_record: &self.home.daemon_info,
            home: &self.home.root,
            processes: &processes,
            probe: &probe,
        };
        let spawner = SystemSpawner;
        let delay = ThreadDelay;
        let supervisor = Supervisor::system(&spawner, &readiness, &delay);
        match supervisor.start_owned(binary, &self.home) {
            Ok(StartResult::Owned(runtime)) => self.finish_attach(runtime.identity, owned),
            Ok(StartResult::Attached(identity)) => self.finish_attach(identity, owned),
            Err(error) => self.publisher.publish(ShellState::ShellError { error }),
        }
    }

    fn finish_attach(&self, identity: BoundDaemonIdentity, owned: bool) {
        self.publisher.publish(ShellState::Attaching);
        let minimum = match VersionReq::parse(MINIMUM_RUNTIME) {
            Ok(minimum) => minimum,
            Err(error) => {
                logging::error(format!("parse minimum runtime version: {error}"));
                self.publish_error(ShellErrorCode::HandshakeFailed);
                return;
            }
        };
        match handshake(&identity, &minimum) {
            Compatibility::Compatible { .. } => self.open_product(identity, owned),
            Compatibility::SkewOlder {
                runtime, needed, ..
            } => {
                self.publisher.publish(ShellState::Skew {
                    runtime,
                    needed,
                    newer: false,
                });
            }
            Compatibility::SkewNewer {
                runtime, needed, ..
            } => {
                self.publisher.publish(ShellState::Skew {
                    runtime,
                    needed,
                    newer: true,
                });
            }
        }
    }

    fn open_product(&self, identity: BoundDaemonIdentity, owned: bool) {
        self.publisher.publish(ShellState::Product {
            origin: identity.origin.clone(),
            owned,
        });
        self.install_runtime_updater(&identity);
        let update_controller = Arc::clone(&self.controller);
        if let Err(error) = windowing::create_main_window(
            &self.app,
            identity.origin,
            Arc::clone(&self.links),
            self.home.app_log.clone(),
            self.publisher.load_deadline_handler(),
            Arc::new(move || update_controller.present_pending_updates()),
        ) {
            logging::error(format!("create product window: {error}"));
            self.publish_error(ShellErrorCode::LoadDeadlineExceeded);
            return;
        }
        health_monitor::spawn(
            self.app.clone(),
            self.home.clone(),
            Arc::clone(&self.links),
            self.publisher.clone(),
            Arc::clone(&self.controller),
        );
    }

    fn install_runtime_updater(&self, identity: &BoundDaemonIdentity) {
        let Some(release) = &self.release else {
            return;
        };
        let stager = match ManifestRuntimeStager::new(
            self.home.clone(),
            release.runtime_manifest.clone(),
            release.current_public_key.clone(),
        ) {
            Ok(stager) => stager,
            Err(error) => {
                logging::error(format!("configure runtime updater: {error}"));
                return;
            }
        };
        let updater = RuntimeUpdater::new(
            self.home.clone(),
            &identity.origin,
            Box::new(stager),
            Box::new(SystemRuntimeLifecycle::new(self.home.clone())),
            env!("CARGO_PKG_VERSION"),
            release.channel.clone(),
        );
        match updater {
            Ok(updater) => {
                self.controller.set_runtime_updater(Some(Arc::new(updater)));
                if self.controller.automatic_updates_enabled() {
                    self.controller.check_runtime_update();
                }
            }
            Err(error) => logging::error(format!("configure runtime update status: {error}")),
        }
    }

    fn publish_error(&self, code: ShellErrorCode) {
        self.publisher.publish(ShellState::ShellError {
            error: ShellError::from_code(code, self.home.app_log.clone()),
        });
    }
}

fn live_bound_runtime(home: &CompozyHome) -> bool {
    let processes = SystemProcessTable;
    let Discovery::Live(record) = discover(&home.daemon_info, &processes) else {
        return false;
    };
    let probe = BoundProbe::new(HttpStatusTransport, Duration::from_secs(2));
    matches!(probe.probe(&record, &home.root, None), Identity::Compozy(_))
}

fn spawn_update_cadence(controller: Arc<DesktopController>, interval: Duration) {
    std::thread::spawn(move || {
        loop {
            std::thread::sleep(interval);
            controller.check_updates();
        }
    });
}
