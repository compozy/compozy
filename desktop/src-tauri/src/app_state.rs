use std::sync::Arc;

use chrono::{DateTime, Utc};
use tauri::{AppHandle, Manager, Wry};

use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::record::{AppRecord, write_atomic};
use crate::state::ShellState;
use crate::{logging, windowing};

#[derive(Clone)]
pub struct AppStatePublisher {
    app: AppHandle<Wry>,
    home: CompozyHome,
    started_at: DateTime<Utc>,
    diagnostic: Option<ShellError>,
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
        }
    }

    pub fn publish(&self, state: ShellState) {
        let record = AppRecord::new(std::process::id(), self.started_at, state.clone())
            .with_diagnostic(self.diagnostic.clone());
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
                && let Err(render_error) = windowing::render_boot(&boot, &write_error)
            {
                logging::error(format!("render app-state write error: {render_error}"));
            }
            return;
        }
        if let Some(boot) = self.app.get_webview_window("boot")
            && let Err(error) = windowing::render_boot(&boot, &state)
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
