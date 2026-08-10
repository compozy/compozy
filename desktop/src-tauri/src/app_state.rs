use std::sync::{Arc, Mutex};

use chrono::{DateTime, Utc};
use tauri::{AppHandle, Manager, Wry};

use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::record::{AppRecord, AppUpdateStatus, write_atomic};
use crate::state::ShellState;
use crate::{boot_window, logging, windowing};

#[derive(Clone)]
pub struct AppStatePublisher {
    app: AppHandle<Wry>,
    home: CompozyHome,
    started_at: DateTime<Utc>,
    diagnostic: Option<ShellError>,
    state: Arc<Mutex<PublishedState>>,
}

struct PublishedState {
    shell: ShellState,
    update: AppUpdateStatus,
}

impl AppStatePublisher {
    pub fn new(
        app: AppHandle<Wry>,
        home: CompozyHome,
        started_at: DateTime<Utc>,
        diagnostic: Option<ShellError>,
    ) -> Self {
        Self {
            app,
            home,
            started_at,
            diagnostic,
            state: Arc::new(Mutex::new(PublishedState {
                shell: ShellState::Resolving,
                update: AppUpdateStatus::default(),
            })),
        }
    }

    pub fn publish(&self, state: ShellState) {
        let update = {
            let mut published = self
                .state
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner());
            published.shell = state.clone();
            published.update.clone()
        };
        self.persist(state, update);
    }

    pub fn update_status(&self, mutate: impl FnOnce(&mut AppUpdateStatus)) {
        let (shell, update) = {
            let mut published = self
                .state
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner());
            mutate(&mut published.update);
            (published.shell.clone(), published.update.clone())
        };
        self.persist(shell, update);
    }

    pub fn status(&self) -> AppUpdateStatus {
        self.state
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .update
            .clone()
    }

    pub fn shell_state(&self) -> ShellState {
        self.state
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .shell
            .clone()
    }

    fn persist(&self, state: ShellState, update: AppUpdateStatus) {
        let channel = option_env!("COMPOZY_RELEASE_CHANNEL").unwrap_or("development");
        let record = AppRecord::new(std::process::id(), self.started_at, state.clone())
            .with_diagnostic(self.diagnostic.clone())
            .with_metadata(env!("CARGO_PKG_VERSION"), channel, update);
        if let Err(error) = write_atomic(&self.home.app_record, &record) {
            logging::error(format!("write app state: {error}"));
            let write_error = ShellState::ShellError {
                error: ShellError::new(
                    ShellErrorCode::ProvisionPermission,
                    "The app state could not be written.",
                    self.home.app_log.clone(),
                ),
            };
            if let Some(boot) = self.app.get_webview_window("boot")
                && let Err(render_error) = boot_window::render(&boot, &write_error)
            {
                logging::error(format!("render app-state write error: {render_error}"));
            }
            return;
        }
        if let Some(boot) = self.app.get_webview_window("boot")
            && let Err(error) = boot_window::render(&boot, &state)
        {
            logging::error(format!("render boot state: {error}"));
        }
    }

    pub fn load_deadline_handler(&self) -> windowing::LoadDeadlineHandler {
        let publisher = self.clone();
        Arc::new(move |error| {
            publisher.publish(ShellState::ShellError { error });
        })
    }
}
