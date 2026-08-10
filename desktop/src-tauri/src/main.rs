#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::error::Error;
use std::fs;
use std::sync::Arc;

use chrono::Utc;
use compozyos_desktop::controller;
use compozyos_desktop::errors::sanitize_public_text;
use compozyos_desktop::home::CompozyHome;
use compozyos_desktop::links::LinkQueue;
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
            shell::setup(
                app.handle(),
                setup_home.clone(),
                Arc::clone(&setup_links),
                started_at,
            )
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
