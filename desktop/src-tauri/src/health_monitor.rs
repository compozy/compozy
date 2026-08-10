use std::sync::Arc;
use std::time::Duration;

use semver::VersionReq;
use tauri::{AppHandle, Manager, Wry};

use crate::app_state::AppStatePublisher;
use crate::controller::DesktopController;
use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::links::LinkQueue;
use crate::runtime::discovery::{Discovery, ProcessTable, SystemProcessTable, discover};
use crate::runtime::health::{HealthTracker, HealthTransition};
use crate::runtime::probe::{
    BoundProbe, Compatibility, HttpStatusTransport, Identity, IdentityProbe, MINIMUM_RUNTIME,
    handshake,
};
use crate::runtime::resolver::{InstallLocator, SystemInstallLocator};
use crate::state::{DisconnectCause, ShellState};
use crate::{boot_window, logging, windowing};

#[derive(Debug, Default)]
struct CompatibilityRetry {
    pending: bool,
}

impl CompatibilityRetry {
    fn should_check(&self, transition: HealthTransition, identity_present: bool) -> bool {
        identity_present && (transition == HealthTransition::Reconnected || self.pending)
    }

    fn record(&mut self, compatible: bool) {
        self.pending = !compatible;
    }

    fn reset(&mut self) {
        self.pending = false;
    }
}

pub fn spawn(
    app: AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    publisher: AppStatePublisher,
    controller: Arc<DesktopController>,
) {
    std::thread::spawn(move || monitor(app, home, links, publisher, controller));
}

fn monitor(
    app: AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    publisher: AppStatePublisher,
    controller: Arc<DesktopController>,
) {
    let processes = SystemProcessTable;
    let installs = SystemInstallLocator;
    let probe = BoundProbe::new(HttpStatusTransport::default(), Duration::from_secs(2));
    let minimum = match VersionReq::parse(MINIMUM_RUNTIME) {
        Ok(minimum) => minimum,
        Err(error) => {
            logging::error(format!("parse health-check runtime version: {error}"));
            return;
        }
    };
    let mut tracker = HealthTracker::with_failure_threshold(3);
    let mut compatibility_retry = CompatibilityRetry::default();
    loop {
        std::thread::sleep(Duration::from_secs(2));
        let identity = match discover(&home.daemon_info, &processes) {
            Discovery::Live(record) => match probe.probe(&record, None) {
                Identity::Compozy(identity) => Some(*identity),
                Identity::Foreign | Identity::Unreachable => None,
            },
            Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => None,
        };
        let transition = tracker.observe(identity.is_some());
        if transition == HealthTransition::Disconnected {
            compatibility_retry.reset();
            disconnect(&app, &publisher);
            continue;
        }
        if !compatibility_retry.should_check(transition, identity.is_some()) {
            continue;
        }
        let Some(identity) = identity else {
            logging::error("health tracker requested compatibility without an identity");
            continue;
        };
        let owned = processes
            .executable(identity.record.pid)
            .is_some_and(|path| installs.owns_executable(&home, &path));
        match handshake(&identity, &minimum) {
            Compatibility::Compatible { .. } => {
                compatibility_retry.record(true);
                let state = ShellState::Product {
                    origin: identity.origin.clone(),
                    owned,
                };
                publisher.publish(state);
                let update_controller = Arc::clone(&controller);
                if let Err(error) = windowing::create_main_window(
                    &app,
                    identity.origin,
                    Arc::clone(&links),
                    home.app_log.clone(),
                    publisher.load_deadline_handler(),
                    Arc::new(move || update_controller.present_pending_updates()),
                ) {
                    logging::error(format!("reopen product window: {error}"));
                    let state = ShellState::ShellError {
                        error: ShellError::from_code(
                            ShellErrorCode::LoadDeadlineExceeded,
                            home.app_log.clone(),
                        ),
                    };
                    publisher.publish(state.clone());
                    if let Err(show_error) = boot_window::show(&app, state) {
                        logging::error(format!("show reconnect error: {show_error}"));
                    }
                }
            }
            Compatibility::SkewOlder {
                runtime, needed, ..
            } => {
                let first_skew = !compatibility_retry.pending;
                compatibility_retry.record(false);
                if first_skew {
                    show_skew(&app, &publisher, runtime, needed, false);
                }
            }
            Compatibility::SkewNewer {
                runtime, needed, ..
            } => {
                let first_skew = !compatibility_retry.pending;
                compatibility_retry.record(false);
                if first_skew {
                    show_skew(&app, &publisher, runtime, needed, true);
                }
            }
        }
    }
}

fn disconnect(app: &AppHandle<Wry>, publisher: &AppStatePublisher) {
    let state = ShellState::Disconnected {
        cause: DisconnectCause::RuntimeDown,
    };
    publisher.publish(state.clone());
    if let Some(main) = app.get_webview_window("main")
        && let Err(error) = main.close()
    {
        logging::error(format!("close disconnected product window: {error}"));
    }
    if let Err(error) = boot_window::show(app, state) {
        logging::error(format!("show disconnected boot window: {error}"));
    }
}

fn show_skew(
    app: &AppHandle<Wry>,
    publisher: &AppStatePublisher,
    runtime: semver::Version,
    needed: VersionReq,
    newer: bool,
) {
    let state = ShellState::Skew {
        runtime,
        needed,
        newer,
    };
    publisher.publish(state.clone());
    if let Err(error) = boot_window::show(app, state) {
        logging::error(format!("show reconnect skew state: {error}"));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_retry_compatibility_after_skew_until_the_runtime_matches() {
        let mut retry = CompatibilityRetry::default();
        assert!(retry.should_check(HealthTransition::Reconnected, true));

        retry.record(false);
        assert!(retry.should_check(HealthTransition::None, true));
        retry.reset();
        assert!(!retry.should_check(HealthTransition::None, true));
        assert!(retry.should_check(HealthTransition::Reconnected, true));

        retry.record(true);
        assert!(!retry.should_check(HealthTransition::None, true));
    }
}
