#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::error::Error;
use std::fs;
use std::panic;
use std::path::PathBuf;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use chrono::Utc;
use compozyos_desktop::controller;
use compozyos_desktop::diagnostics::startup_marker::StartupMarker;
use compozyos_desktop::errors::sanitize_public_text;
use compozyos_desktop::home::CompozyHome;
use compozyos_desktop::links::LinkQueue;
use compozyos_desktop::runtime::discovery::{ProcessTable, SystemProcessTable};
use compozyos_desktop::{boot_window, logging, shell, windowing};
use tauri::{AppHandle, Wry, webview::PageLoadEvent};
use tauri_plugin_deep_link::DeepLinkExt;

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
    let observed_at = Utc::now();
    let process_id = std::process::id();
    let boot_id = format!(
        "desktop-{}-{}",
        process_id,
        observed_at
            .timestamp_nanos_opt()
            .unwrap_or_else(|| observed_at.timestamp_millis())
    );
    let process_table = SystemProcessTable;
    let started_at = process_table.start_time(process_id).unwrap_or_else(|| {
        let message = "The operating system did not report the desktop process start time.";
        eprintln!("{}", sanitize_public_text(message));
        if let Err(error) = logging::bootstrap_error(
            &home.logs_dir,
            &boot_id,
            "read desktop process start time",
            message,
        ) {
            eprintln!(
                "{}",
                sanitize_public_text(&format!("write fallback desktop log: {error}"))
            );
        }
        observed_at
    });
    install_panic_hook(home.logs_dir.clone(), boot_id.clone());
    let startup_marker = match StartupMarker::begin(&home, boot_id.clone(), started_at) {
        Ok(marker) => Some(marker),
        Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => None,
        Err(error) => {
            eprintln!(
                "{}",
                sanitize_public_text(&format!("create desktop startup marker: {error}"))
            );
            if let Err(write_error) = logging::bootstrap_error(
                &home.logs_dir,
                &boot_id,
                "create desktop startup marker",
                error.to_string(),
            ) {
                eprintln!(
                    "{}",
                    sanitize_public_text(&format!("write fallback desktop log: {write_error}"))
                );
            }
            None
        }
    };
    if let Err(error) = fix_path_env::fix() {
        eprintln!(
            "{}",
            sanitize_public_text(&format!("refresh desktop PATH: {error}"))
        );
        if let Err(write_error) = logging::bootstrap_error(
            &home.logs_dir,
            &boot_id,
            "refresh desktop PATH",
            error.to_string(),
        ) {
            eprintln!(
                "{}",
                sanitize_public_text(&format!("write fallback desktop log: {write_error}"))
            );
        }
    }

    let links = Arc::new(LinkQueue::default());
    let single_instance_links = Arc::clone(&links);
    let setup_links = Arc::clone(&links);
    let setup_home = home.clone();
    let setup_boot_id = boot_id.clone();
    let setup_previous_crash = startup_marker
        .as_ref()
        .and_then(StartupMarker::previous_crash);
    let setup_startup_marker = startup_marker.clone();

    let run_result = tauri::Builder::default()
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
        .plugin(logging::plugin(home.logs_dir.clone(), boot_id.clone()))
        .plugin(tauri_plugin_clipboard_manager::init())
        .invoke_handler(tauri::generate_handler![controller::shell_control])
        .on_page_load(|webview, payload| {
            if webview.label() == "boot"
                && payload.event() == PageLoadEvent::Finished
                && let Err(error) = boot_window::install_control_bridge(webview)
            {
                logging::error(format!("install boot control bridge: {error}"));
            }
        })
        .setup(move |app| {
            register_deep_links(app.handle(), &setup_links);
            let app_version = app.package_info().version.to_string();
            shell::setup(
                app.handle(),
                setup_home.clone(),
                Arc::clone(&setup_links),
                shell::ShellStartup {
                    started_at,
                    app_version,
                    boot_id: setup_boot_id.clone(),
                    previous_crash: setup_previous_crash.clone(),
                    startup_marker: setup_startup_marker.clone(),
                },
            )
        })
        .run(tauri::generate_context!());
    match run_result {
        Ok(()) => {
            if let Some(marker) = startup_marker
                && let Err(error) = marker.clear()
            {
                logging::error(format!("clear desktop startup marker: {error}"));
            }
        }
        Err(error) => {
            if let Err(write_error) = logging::bootstrap_error(
                &home.logs_dir,
                &boot_id,
                "run desktop application",
                error.to_string(),
            ) {
                eprintln!(
                    "{}",
                    sanitize_public_text(&format!("write fallback desktop log: {write_error}"))
                );
            }
            return Err(Box::new(error));
        }
    }
    Ok(())
}

fn install_panic_hook(logs_dir: PathBuf, boot_id: String) {
    let previous_hook = panic::take_hook();
    let writing_panic = Arc::new(AtomicBool::new(false));
    panic::set_hook(Box::new(move |info| {
        if !writing_panic.swap(true, Ordering::AcqRel) {
            let message = sanitize_public_text(&info.to_string());
            if let Err(error) = logging::bootstrap_error(&logs_dir, &boot_id, "panic", &message) {
                eprintln!(
                    "{}",
                    sanitize_public_text(&format!("write panic fallback log: {error}"))
                );
            }
            writing_panic.store(false, Ordering::Release);
        }
        previous_hook(info);
    }));
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
    let event_app = app.clone();
    app.deep_link().on_open_url(move |event| {
        for url in event.urls() {
            event_links.push(url.as_str());
        }
        if let Err(error) = windowing::focus_existing(&event_app, &event_links) {
            logging::error(format!("focus deep-link target: {error}"));
        }
    });
}
