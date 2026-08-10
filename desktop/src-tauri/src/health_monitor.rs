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
    let probe = BoundProbe::new(HttpStatusTransport, Duration::from_secs(2));
    let minimum = match VersionReq::parse(MINIMUM_RUNTIME) {
        Ok(minimum) => minimum,
        Err(error) => {
            logging::error(format!("parse health-check runtime version: {error}"));
            return;
        }
    };
    let mut tracker = HealthTracker::connected(3);
    loop {
        std::thread::sleep(Duration::from_secs(2));
        let identity = match discover(&home.daemon_info, &processes) {
            Discovery::Live(record) => match probe.probe(&record, &home.root, None) {
                Identity::Compozy(identity) => Some(*identity),
                Identity::Foreign | Identity::Unreachable => None,
            },
            Discovery::Absent | Discovery::AbsentWithDiagnostic(_) => None,
        };
        match tracker.observe(identity.is_some()) {
            HealthTransition::None => {}
            HealthTransition::Disconnected => disconnect(&app, &publisher),
            HealthTransition::Reconnected => {
                let Some(identity) = identity else {
                    logging::error("health tracker reconnected without an identity");
                    continue;
                };
                let owned = processes
                    .executable(identity.record.pid)
                    .is_some_and(|path| installs.owns_executable(&home, &path));
                match handshake(&identity, &minimum) {
                    Compatibility::Compatible { .. } => {
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
                        show_skew(&app, &publisher, runtime, needed, false);
                        return;
                    }
                    Compatibility::SkewNewer {
                        runtime, needed, ..
                    } => {
                        show_skew(&app, &publisher, runtime, needed, true);
                        return;
                    }
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
