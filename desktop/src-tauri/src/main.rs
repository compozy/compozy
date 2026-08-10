#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::error::Error;
use std::fs;
use std::sync::Arc;
use std::time::Duration;

use chrono::{DateTime, Utc};
use compozyos_desktop::app_state::AppStatePublisher;
use compozyos_desktop::errors::{ShellError, ShellErrorCode, sanitize_public_text};
use compozyos_desktop::home::CompozyHome;
use compozyos_desktop::links::LinkQueue;
use compozyos_desktop::runtime::discovery::SystemProcessTable;
use compozyos_desktop::runtime::probe::{
    BoundDaemonIdentity, BoundProbe, Compatibility, HttpStatusTransport, handshake,
};
use compozyos_desktop::runtime::readiness::DaemonReadiness;
use compozyos_desktop::runtime::resolver::{
    AttachFirstResolver, BinarySource, Resolution, RuntimeResolver, SystemInstallLocator,
};
use compozyos_desktop::runtime::supervisor::{StartResult, Supervisor, SystemSpawner, ThreadDelay};
use compozyos_desktop::state::{ProvisionStage, ShellState};
use compozyos_desktop::{config, health_monitor, logging, windowing};
use semver::VersionReq;
use tauri::{AppHandle, Wry};
use tauri_plugin_deep_link::DeepLinkExt;

const MINIMUM_RUNTIME: &str = ">=0.3.0";

fn main() {
    if let Err(error) = run() {
        eprintln!(
            "{}",
            sanitize_public_text(&format!("CompozyOS desktop failed: {error}"))
        );
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn Error>> {
    let home = CompozyHome::resolve()?;
    fs::create_dir_all(&home.logs_dir)?;
    if let Err(error) = fix_path_env::fix() {
        eprintln!(
            "{}",
            sanitize_public_text(&format!("refresh desktop PATH: {error}"))
        );
    }

    let links = Arc::new(LinkQueue::default());
    let single_instance_links = Arc::clone(&links);
    let setup_links = Arc::clone(&links);
    let setup_home = home.clone();
    let started_at = Utc::now();

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(
            move |app, arguments, _cwd| {
                single_instance_links.push_argv(&arguments);
                if let Err(error) = windowing::focus_existing(app, &single_instance_links) {
                    logging::error(format!("focus forwarded app instance: {error}"));
                }
            },
        ))
        .plugin(tauri_plugin_deep_link::init())
        .plugin(
            tauri_plugin_window_state::Builder::new()
                .with_denylist(&["boot"])
                .build(),
        )
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(logging::plugin(home.logs_dir.clone()))
        .setup(move |app| {
            register_deep_links(app.handle(), &setup_links);
            let app_handle = app.handle().clone();
            let startup_links = Arc::clone(&setup_links);
            let startup_home = setup_home.clone();
            std::thread::spawn(move || {
                start_shell(app_handle, startup_home, startup_links, started_at);
            });
            Ok(())
        })
        .run(tauri::generate_context!())?;
    Ok(())
}

fn register_deep_links(app: &AppHandle<Wry>, links: &Arc<LinkQueue>) {
    #[cfg(target_os = "linux")]
    if let Err(error) = app.deep_link().register_all() {
        logging::error(format!("register configured deep links: {error}"));
    }

    #[cfg(all(debug_assertions, any(target_os = "linux", target_os = "windows")))]
    if let Err(error) = app.deep_link().register("compozyos-dev") {
        logging::error(format!("register development deep link: {error}"));
    }

    match app.deep_link().get_current() {
        Ok(Some(urls)) => {
            for url in urls {
                links.push(url.as_str());
            }
        }
        Ok(None) => {}
        Err(error) => logging::error(format!("read launch deep link: {error}")),
    }
    let event_links = Arc::clone(links);
    app.deep_link().on_open_url(move |event| {
        for url in event.urls() {
            event_links.push(url.as_str());
        }
    });
}

fn start_shell(
    app: AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    started_at: DateTime<Utc>,
) {
    let settings = config::load(&home.config_file, &home);
    if let Some(diagnostic) = &settings.diagnostic {
        logging::error(&diagnostic.safe_message);
    }
    let publisher =
        AppStatePublisher::new(app.clone(), home.clone(), started_at, settings.diagnostic);
    publisher.publish(ShellState::Resolving);

    let processes = SystemProcessTable;
    let probe = BoundProbe::new(HttpStatusTransport, Duration::from_secs(2));
    let installs = SystemInstallLocator;
    let resolver = AttachFirstResolver {
        processes: &processes,
        probe: &probe,
        installs: &installs,
    };
    match resolver.resolve(&home) {
        Resolution::Attached { identity, owned } => {
            finish_attach(&app, &home, &links, &publisher, identity, owned);
        }
        Resolution::NeedsStart { binary, source } => {
            publisher.publish(ShellState::Starting { attempt: 1 });
            let readiness = DaemonReadiness {
                daemon_record: &home.daemon_info,
                home: &home.root,
                processes: &processes,
                probe: &probe,
            };
            let spawner = SystemSpawner;
            let delay = ThreadDelay;
            let supervisor = Supervisor::system(&spawner, &readiness, &delay);
            match supervisor.start_owned(&binary, &home) {
                Ok(StartResult::Owned(runtime)) => finish_attach(
                    &app,
                    &home,
                    &links,
                    &publisher,
                    runtime.identity,
                    source == BinarySource::AppOwned,
                ),
                Ok(StartResult::Attached(identity)) => finish_attach(
                    &app,
                    &home,
                    &links,
                    &publisher,
                    identity,
                    source == BinarySource::AppOwned,
                ),
                Err(error) => publisher.publish(ShellState::ShellError { error }),
            }
        }
        Resolution::NeedsProvision => publisher.publish(ShellState::Provisioning {
            stage: ProvisionStage::Download { pct: 0 },
        }),
        Resolution::Failed { error } => publisher.publish(ShellState::ShellError { error }),
    }
}

fn finish_attach(
    app: &AppHandle<Wry>,
    home: &CompozyHome,
    links: &Arc<LinkQueue>,
    publisher: &AppStatePublisher,
    identity: BoundDaemonIdentity,
    owned: bool,
) {
    publisher.publish(ShellState::Attaching);
    let minimum = match VersionReq::parse(MINIMUM_RUNTIME) {
        Ok(minimum) => minimum,
        Err(error) => {
            logging::error(format!("parse minimum runtime version: {error}"));
            publisher.publish(ShellState::ShellError {
                error: ShellError::from_code(ShellErrorCode::HandshakeFailed, home.app_log.clone()),
            });
            return;
        }
    };
    match handshake(&identity, &minimum) {
        Compatibility::Compatible { .. } => {
            publisher.publish(ShellState::Product {
                origin: identity.origin.clone(),
                owned,
            });
            if let Err(error) = windowing::create_main_window(
                app,
                identity.origin,
                Arc::clone(links),
                home.app_log.clone(),
                publisher.load_deadline_handler(),
            ) {
                logging::error(format!("create product window: {error}"));
                publisher.publish(ShellState::ShellError {
                    error: ShellError::from_code(
                        ShellErrorCode::LoadDeadlineExceeded,
                        home.app_log.clone(),
                    ),
                });
            } else {
                health_monitor::spawn(
                    app.clone(),
                    home.clone(),
                    Arc::clone(links),
                    publisher.clone(),
                );
            }
        }
        Compatibility::SkewOlder {
            runtime, needed, ..
        } => publisher.publish(ShellState::Skew {
            runtime,
            needed,
            newer: false,
        }),
        Compatibility::SkewNewer {
            runtime, needed, ..
        } => publisher.publish(ShellState::Skew {
            runtime,
            needed,
            newer: true,
        }),
    }
}
