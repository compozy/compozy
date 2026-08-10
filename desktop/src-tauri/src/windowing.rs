use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use tauri::webview::{NewWindowResponse, PageLoadEvent};
use tauri::{
    AppHandle, Manager, PhysicalPosition, PhysicalSize, Runtime, WebviewUrl, WebviewWindow,
    WebviewWindowBuilder, Wry,
};
use tauri_plugin_opener::OpenerExt;
use url::Url;

use crate::boot_window;
use crate::errors::{ShellError, ShellErrorCode};
use crate::links::LinkQueue;
use crate::nav::{self, NavigationDecision};
use crate::state::ShellState;

const LOAD_DEADLINE: Duration = Duration::from_secs(10);
const WINDOW_RELEASE_ATTEMPTS: usize = 100;
const WINDOW_RELEASE_POLL: Duration = Duration::from_millis(10);
const MAIN_MIN_WIDTH: u32 = 900;
const MAIN_MIN_HEIGHT: u32 = 600;

pub type LoadDeadlineHandler = Arc<dyn Fn(ShellError) + Send + Sync>;
pub type ProductReadyHandler = Arc<dyn Fn() + Send + Sync>;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Rect {
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RestorePlan {
    pub recover_geometry: bool,
    pub unminimize: bool,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LoadDecision {
    Wait,
    Product,
    Error,
}

pub fn create_main_window(
    app: &AppHandle<Wry>,
    origin: Url,
    links: Arc<LinkQueue>,
    log_path: std::path::PathBuf,
    on_load_deadline: LoadDeadlineHandler,
    on_product_ready: ProductReadyHandler,
) -> tauri::Result<WebviewWindow<Wry>> {
    let loaded = Arc::new(AtomicBool::new(false));
    let loaded_for_page = Arc::clone(&loaded);
    let origin_for_navigation = origin.clone();
    let app_for_navigation = app.clone();
    let origin_for_new_window = origin.clone();
    let app_for_new_window = app.clone();
    let links_for_page = Arc::clone(&links);
    let window = WebviewWindowBuilder::new(app, "main", WebviewUrl::External(origin.clone()))
        .title("CompozyOS")
        .inner_size(1280.0, 800.0)
        .min_inner_size(MAIN_MIN_WIDTH as f64, MAIN_MIN_HEIGHT as f64)
        .visible(false)
        .background_color(tauri::window::Color(19, 18, 17, 255))
        .on_navigation(
            move |target| match nav::classify(target, &origin_for_navigation) {
                NavigationDecision::Allow => true,
                NavigationDecision::OpenExternal => {
                    if let Err(error) = app_for_navigation
                        .opener()
                        .open_url(target.as_str(), None::<&str>)
                    {
                        crate::logging::error(format!("open external URL: {error}"));
                    }
                    false
                }
            },
        )
        .on_new_window(move |target, _features| {
            if nav::classify(&target, &origin_for_new_window) == NavigationDecision::OpenExternal
                && let Err(error) = app_for_new_window
                    .opener()
                    .open_url(target.as_str(), None::<&str>)
            {
                crate::logging::error(format!("open new-window URL externally: {error}"));
            }
            NewWindowResponse::Deny
        })
        .on_page_load(move |window, payload| {
            if payload.event() != PageLoadEvent::Finished {
                return;
            }
            loaded_for_page.store(true, Ordering::Release);
            if let Err(error) = reveal_product(&window, &links_for_page) {
                crate::logging::error(format!("reveal product window: {error}"));
                return;
            }
            on_product_ready();
        })
        .build()?;

    let app_for_deadline = app.clone();
    std::thread::spawn(move || {
        std::thread::sleep(LOAD_DEADLINE);
        if load_decision(LOAD_DEADLINE, loaded.load(Ordering::Acquire)) != LoadDecision::Error {
            return;
        }
        match destroy_timed_out_product(&app_for_deadline) {
            Ok(true) => {}
            Ok(false) => crate::logging::error(
                "timed-out product window label was not released before retry became available",
            ),
            Err(destroy_error) => {
                crate::logging::error(format!("destroy timed-out product window: {destroy_error}"))
            }
        }
        let error = ShellError::from_code(ShellErrorCode::LoadDeadlineExceeded, log_path);
        on_load_deadline(error.clone());
        if let Some(boot) = app_for_deadline.get_webview_window("boot") {
            if let Err(render_error) = boot_window::render(&boot, &ShellState::ShellError { error })
            {
                crate::logging::error(format!("render load deadline: {render_error}"));
            }
            if let Err(show_error) = boot.show() {
                crate::logging::error(format!("show boot deadline state: {show_error}"));
            }
        }
    });
    Ok(window)
}

fn destroy_timed_out_product<R: Runtime>(app: &AppHandle<R>) -> tauri::Result<bool> {
    let Some(main) = app.get_webview_window("main") else {
        return Ok(true);
    };
    main.destroy()?;
    Ok(wait_for_window_release(
        || app.get_webview_window("main").is_some(),
        || std::thread::sleep(WINDOW_RELEASE_POLL),
    ))
}

fn wait_for_window_release(mut is_present: impl FnMut() -> bool, mut delay: impl FnMut()) -> bool {
    for _ in 0..WINDOW_RELEASE_ATTEMPTS {
        if !is_present() {
            return true;
        }
        delay();
    }
    !is_present()
}

pub fn focus_existing(app: &AppHandle<Wry>, links: &LinkQueue) -> tauri::Result<()> {
    if let Some(main) = app.get_webview_window("main") {
        if main.is_minimized()? {
            main.unminimize()?;
        }
        main.show()?;
        main.set_focus()?;
        navigate_pending(&main, links)?;
        return Ok(());
    }
    if let Some(boot) = app.get_webview_window("boot") {
        boot.show()?;
        boot.set_focus()?;
    }
    Ok(())
}

fn reveal_product(window: &WebviewWindow<Wry>, links: &LinkQueue) -> tauri::Result<()> {
    let plan = restore_plan_for_window(window)?;
    if plan.recover_geometry {
        window.set_size(PhysicalSize::new(1280, 800))?;
        window.center()?;
        window.set_position(PhysicalPosition::new(
            window.outer_position()?.x,
            window.outer_position()?.y,
        ))?;
    }
    if plan.unminimize {
        window.unminimize()?;
    }
    window.show()?;
    window.set_focus()?;
    navigate_pending(window, links)?;
    if let Some(boot) = window.app_handle().get_webview_window("boot") {
        boot.close()?;
    }
    Ok(())
}

fn navigate_pending(window: &WebviewWindow<Wry>, links: &LinkQueue) -> tauri::Result<()> {
    let Some(path) = links.take() else {
        return Ok(());
    };
    let mut target = window.url()?;
    target.set_path(path.split('?').next().unwrap_or("/"));
    target.set_query(path.split_once('?').map(|(_, query)| query));
    window.navigate(target)
}

fn restore_plan_for_window(window: &WebviewWindow<Wry>) -> tauri::Result<RestorePlan> {
    let position = window.outer_position()?;
    let size = window.outer_size()?;
    let current = Rect {
        x: position.x,
        y: position.y,
        width: size.width,
        height: size.height,
    };
    let work_areas: Vec<Rect> = window
        .available_monitors()?
        .into_iter()
        .map(|monitor| {
            let position = monitor.position();
            let size = monitor.size();
            Rect {
                x: position.x,
                y: position.y,
                width: size.width,
                height: size.height,
            }
        })
        .collect();
    Ok(restore_plan(current, &work_areas, window.is_minimized()?))
}

pub fn restore_plan(rect: Rect, work_areas: &[Rect], minimized: bool) -> RestorePlan {
    RestorePlan {
        recover_geometry: !valid_geometry(rect, work_areas, MAIN_MIN_WIDTH, MAIN_MIN_HEIGHT),
        unminimize: minimized,
    }
}

pub fn load_decision(elapsed: Duration, loaded: bool) -> LoadDecision {
    if loaded {
        LoadDecision::Product
    } else if elapsed >= LOAD_DEADLINE {
        LoadDecision::Error
    } else {
        LoadDecision::Wait
    }
}

pub fn valid_geometry(rect: Rect, work_areas: &[Rect], min_width: u32, min_height: u32) -> bool {
    rect.width >= min_width
        && rect.height >= min_height
        && work_areas.iter().any(|area| intersects(rect, *area))
}

fn intersects(left: Rect, right: Rect) -> bool {
    let left_right = i64::from(left.x) + i64::from(left.width);
    let left_bottom = i64::from(left.y) + i64::from(left.height);
    let right_right = i64::from(right.x) + i64::from(right.width);
    let right_bottom = i64::from(right.y) + i64::from(right.height);
    i64::from(left.x) < right_right
        && left_right > i64::from(right.x)
        && i64::from(left.y) < right_bottom
        && left_bottom > i64::from(right.y)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_accept_visible_geometry_in_current_work_area() {
        let areas = [Rect {
            x: 0,
            y: 0,
            width: 1920,
            height: 1080,
        }];
        assert!(valid_geometry(
            Rect {
                x: 100,
                y: 100,
                width: 1280,
                height: 800,
            },
            &areas,
            MAIN_MIN_WIDTH,
            MAIN_MIN_HEIGHT,
        ));
        assert_eq!(
            restore_plan(
                Rect {
                    x: 100,
                    y: 100,
                    width: 1280,
                    height: 800,
                },
                &areas,
                false,
            ),
            RestorePlan {
                recover_geometry: false,
                unminimize: false,
            }
        );
    }

    #[test]
    fn should_reject_offscreen_removed_monitor_and_subminimum_geometry() {
        let areas = [Rect {
            x: 0,
            y: 0,
            width: 1920,
            height: 1080,
        }];
        for rect in [
            Rect {
                x: 3000,
                y: 0,
                width: 1280,
                height: 800,
            },
            Rect {
                x: 100,
                y: 100,
                width: 899,
                height: 800,
            },
            Rect {
                x: 100,
                y: 100,
                width: 1280,
                height: 599,
            },
        ] {
            assert!(!valid_geometry(
                rect,
                &areas,
                MAIN_MIN_WIDTH,
                MAIN_MIN_HEIGHT
            ));
            assert!(restore_plan(rect, &areas, false).recover_geometry);
        }
    }

    #[test]
    fn should_hold_load_deadline_at_ten_seconds() {
        assert_eq!(
            load_decision(Duration::from_millis(9_900), false),
            LoadDecision::Wait
        );
        assert_eq!(
            load_decision(Duration::from_secs(10), false),
            LoadDecision::Error
        );
        assert_eq!(
            load_decision(Duration::from_secs(10), true),
            LoadDecision::Product
        );
    }

    #[test]
    fn should_unminimize_a_valid_restored_window() {
        let areas = [Rect {
            x: 0,
            y: 0,
            width: 1920,
            height: 1080,
        }];
        assert_eq!(
            restore_plan(
                Rect {
                    x: 100,
                    y: 100,
                    width: 1280,
                    height: 800,
                },
                &areas,
                true,
            ),
            RestorePlan {
                recover_geometry: false,
                unminimize: true,
            }
        );
    }

    #[test]
    fn should_wait_for_timed_out_product_window_to_release_before_retry() {
        let mut checks = 0;
        let mut delays = 0;

        let released = wait_for_window_release(
            || {
                checks += 1;
                checks < 4
            },
            || delays += 1,
        );

        assert!(released);
        assert_eq!(checks, 4);
        assert_eq!(delays, 3);
    }
}
