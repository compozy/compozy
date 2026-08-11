use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use chrono::{DateTime, Utc};
use semver::VersionReq;
use tauri::{AppHandle, Manager, Wry};

use crate::app_state::{AppStatePublisher, AppStateStartup};
use crate::config;
use crate::controller::DesktopController;
use crate::errors::{ShellError, ShellErrorCode};
use crate::health_monitor;
use crate::home::CompozyHome;
use crate::links::LinkQueue;
use crate::release::ReleaseConfig;
use crate::runtime::artifacts::current_target;
use crate::runtime::control_server_startup::{
    ControlServerStartup, show_boot_before_starting_control,
};
use crate::runtime::discovery::SystemProcessTable;
use crate::runtime::mutation::UpdateLock;
use crate::runtime::probe::{
    BoundDaemonIdentity, BoundProbe, Compatibility, HttpStatusTransport, MINIMUM_RUNTIME, handshake,
};
use crate::runtime::provision::{
    ProvisionOutcome, ProvisionRequest, ResumeChoice, incomplete_stage, provision, shell_error,
};
use crate::runtime::readiness::{DaemonReadiness, live_bound_runtime};
use crate::runtime::recorded_start::{
    RecordedDaemonStart, await_or_recover_boot_timestamp_drift_daemon,
    await_or_recover_recorded_daemon,
};
use crate::runtime::resolver::{
    AttachFirstResolver, BinarySource, Resolution, RuntimeResolver, SystemInstallLocator,
};
use crate::runtime::startup_failure::{
    development_runtime_error, publish_boot_presentation_failure,
};
use crate::runtime::supervisor::{StartResult, Supervisor, SystemSpawner, ThreadDelay};
use crate::runtime::update_cadence;
use crate::runtime::update_lifecycle::SystemRuntimeLifecycle;
use crate::runtime::update_recovery::recover_or_publish;
use crate::state::ShellState;
use crate::update::app_update::{AppUpdater, TauriAppUpdateBackend, recover_intent};
use crate::update::runtime_update::{ManifestRuntimeStager, RuntimeUpdater};
use crate::{boot_window, logging, windowing};

const RESOLUTION_ATTEMPTS: usize = 4;

pub struct ShellStartup {
    pub started_at: DateTime<Utc>,
    pub app_version: String,
    pub boot_id: String,
    pub previous_crash: Option<crate::diagnostics::PreviousCrash>,
    pub startup_marker: Option<crate::diagnostics::startup_marker::StartupMarker>,
}

pub fn setup(
    app: &AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    startup: ShellStartup,
) -> Result<(), Box<dyn std::error::Error>> {
    let ShellStartup {
        started_at,
        app_version,
        boot_id,
        previous_crash,
        startup_marker,
    } = startup;
    let settings = config::load(&home.config_file, &home);
    let mut startup_diagnostic = settings.diagnostic.clone();
    if let Some(diagnostic) = &startup_diagnostic {
        logging::error(&diagnostic.safe_message);
    }
    let release = match ReleaseConfig::compiled(app.config()) {
        Ok(release) => release,
        Err(error) => {
            logging::error(format!("load release update configuration: {error:?}"));
            if startup_diagnostic.is_none() {
                startup_diagnostic = Some(ShellError::new(
                    ShellErrorCode::ConfigInvalid,
                    "The release update settings are invalid.",
                    home.app_log.clone(),
                ));
            }
            None
        }
    };
    let publisher = AppStatePublisher::new(
        app.clone(),
        home.clone(),
        AppStateStartup {
            started_at,
            diagnostic: startup_diagnostic,
            app_version: app_version.clone(),
            boot_id,
            previous_crash,
            startup_marker,
        },
    );
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
    match recover_intent(&home.app_update_intent, &app_version, Utc::now()) {
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
    let control_startup = ControlServerStartup::new(
        app.clone(),
        home.clone(),
        Arc::clone(&controller),
        publisher.clone(),
    );
    let coordinator = Arc::new(ShellCoordinator {
        app: app.clone(),
        home,
        links,
        publisher,
        controller: Arc::clone(&controller),
        release,
        app_version,
        control_startup,
        running: AtomicBool::new(false),
    });
    let retry_coordinator = Arc::downgrade(&coordinator);
    controller.set_retry(Arc::new(move || {
        if let Some(coordinator) = retry_coordinator.upgrade() {
            std::thread::spawn(move || coordinator.retry());
        }
    }));

    app.manage(Arc::clone(&controller));
    app.manage(Arc::clone(&coordinator));
    if !show_boot_before_starting_control(
        || boot_window::show(app, ShellState::Resolving),
        |error| publish_boot_presentation_failure(&coordinator.publisher, &coordinator.home, error),
        || coordinator.control_startup.ensure_started(),
    ) {
        return Ok(());
    }

    let startup = Arc::clone(&coordinator);
    let interval = settings.config.update_check_interval;
    std::thread::spawn(move || {
        if updates_enabled {
            startup.controller.check_app_update();
        }
        Arc::clone(&startup).run(false);
        if updates_enabled {
            update_cadence::spawn(startup.controller.clone(), interval);
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
    app_version: String,
    control_startup: ControlServerStartup,
    running: AtomicBool,
}

impl ShellCoordinator {
    fn retry(self: Arc<Self>) {
        if self.control_startup.ensure_started() {
            self.run(true);
        }
    }

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
        if recover_or_publish(&self.home, &self.publisher, resume_recovery) {
            self.resolve_loop();
        }
        self.running.store(false, Ordering::Release);
    }

    fn resolve_loop(&self) {
        let mut recovery_attempted = false;
        for _attempt in 0..RESOLUTION_ATTEMPTS {
            let processes = SystemProcessTable;
            let probe = BoundProbe::new(HttpStatusTransport::default(), Duration::from_secs(2));
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
                Resolution::Awaiting(record) => {
                    if self.handle_recorded_start(&mut recovery_attempted, || {
                        await_or_recover_recorded_daemon(&self.home, &record, &processes, &probe)
                    }) {
                        continue;
                    }
                    return;
                }
                Resolution::StartTimeMismatch {
                    record,
                    observed_start,
                } => {
                    if self.handle_recorded_start(&mut recovery_attempted, || {
                        await_or_recover_boot_timestamp_drift_daemon(
                            &self.home,
                            &record,
                            observed_start,
                            &processes,
                            &probe,
                        )
                    }) {
                        continue;
                    }
                    return;
                }
                Resolution::NeedsStart { binary, source } => {
                    self.start_runtime(&binary, source == BinarySource::AppOwned);
                    return;
                }
                Resolution::NeedsProvision => {
                    if self.release.is_none() {
                        self.publisher.publish(ShellState::ShellError {
                            error: development_runtime_error(&self.home),
                        });
                        return;
                    }
                    let lock = match UpdateLock::acquire(&self.home.update_lock, &processes) {
                        Ok(lock) => lock,
                        Err(error) => {
                            logging::error(format!("acquire runtime provision lock: {error}"));
                            self.publish_error(ShellErrorCode::UpdateLockHeld);
                            return;
                        }
                    };
                    let outcome = self.provision_runtime();
                    if let Err(error) = lock.release() {
                        logging::error(format!("release runtime provision lock: {error}"));
                        self.publish_error(ShellErrorCode::UpdateLockHeld);
                        return;
                    }
                    match outcome {
                        Ok(
                            ProvisionOutcome::Installed { .. } | ProvisionOutcome::AttachExternally,
                        ) => {
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
                    }
                }
                Resolution::Failed { error } => {
                    self.publisher.publish(ShellState::ShellError { error });
                    return;
                }
            }
        }
        self.publish_error(ShellErrorCode::RuntimeStartFailed);
    }

    fn handle_recorded_start(
        &self,
        recovery_attempted: &mut bool,
        start: impl FnOnce() -> RecordedDaemonStart,
    ) -> bool {
        if *recovery_attempted {
            self.publish_error(ShellErrorCode::RuntimeStartFailed);
            return false;
        }
        match start() {
            RecordedDaemonStart::Attached { identity, owned } => {
                self.finish_attach(*identity, owned);
                false
            }
            RecordedDaemonStart::Recovered => {
                *recovery_attempted = true;
                true
            }
            RecordedDaemonStart::Unverified => {
                self.publish_error(ShellErrorCode::RuntimeUnhealthy);
                false
            }
            RecordedDaemonStart::Failed => {
                self.publish_error(ShellErrorCode::RuntimeStartFailed);
                false
            }
        }
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
            app_version: &self.app_version,
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
        let probe = BoundProbe::new(HttpStatusTransport::default(), Duration::from_secs(2));
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
        self.controller
            .set_runtime_metadata(identity.status.daemon.version.clone(), owned);
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
            self.app_version.clone(),
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
