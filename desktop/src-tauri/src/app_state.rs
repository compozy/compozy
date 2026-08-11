use std::sync::{Arc, Mutex};

use chrono::{DateTime, Utc};
use tauri::{AppHandle, Manager, Wry};

use crate::diagnostics::startup_marker::StartupMarker;
use crate::diagnostics::{
    DiagnosticReport, PreviousCrash, RuntimeDiagnosticMetadata, boot_phase_from_state,
};
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
    startup_diagnostic: Option<ShellError>,
    app_version: String,
    boot_id: String,
    previous_crash: Option<PreviousCrash>,
    startup_marker: Option<StartupMarker>,
    state: Arc<Mutex<PublishedState>>,
    publish_order: Arc<Mutex<()>>,
}

pub struct AppStateStartup {
    pub started_at: DateTime<Utc>,
    pub diagnostic: Option<ShellError>,
    pub app_version: String,
    pub boot_id: String,
    pub previous_crash: Option<PreviousCrash>,
    pub startup_marker: Option<StartupMarker>,
}

struct PublishedState {
    shell: ShellState,
    update: AppUpdateStatus,
    runtime: RuntimeDiagnosticMetadata,
}

impl AppStatePublisher {
    pub fn new(app: AppHandle<Wry>, home: CompozyHome, startup: AppStateStartup) -> Self {
        Self {
            app,
            home,
            started_at: startup.started_at,
            startup_diagnostic: startup.diagnostic,
            app_version: startup.app_version,
            boot_id: startup.boot_id,
            previous_crash: startup.previous_crash,
            startup_marker: startup.startup_marker,
            publish_order: Arc::new(Mutex::new(())),
            state: Arc::new(Mutex::new(PublishedState {
                shell: ShellState::Resolving,
                update: AppUpdateStatus::default(),
                runtime: RuntimeDiagnosticMetadata::default(),
            })),
        }
    }

    pub fn publish(&self, state: ShellState) {
        publish_ordered(
            &self.publish_order,
            &self.state,
            |published| {
                published.shell = state.clone();
                (state, published.update.clone(), published.runtime.clone())
            },
            |(state, update, runtime)| self.persist(state, update, runtime),
        );
    }

    pub fn update_status(&self, mutate: impl FnOnce(&mut AppUpdateStatus)) {
        publish_ordered(
            &self.publish_order,
            &self.state,
            |published| {
                mutate(&mut published.update);
                (
                    published.shell.clone(),
                    published.update.clone(),
                    published.runtime.clone(),
                )
            },
            |(shell, update, runtime)| self.persist(shell, update, runtime),
        );
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

    pub fn set_runtime_metadata(&self, version: Option<String>, owned: bool) {
        publish_ordered(
            &self.publish_order,
            &self.state,
            |published| {
                published.runtime = RuntimeDiagnosticMetadata {
                    version,
                    owned: Some(owned),
                };
                (
                    published.shell.clone(),
                    published.update.clone(),
                    published.runtime.clone(),
                )
            },
            |(shell, update, runtime)| self.persist(shell, update, runtime),
        );
    }

    pub fn diagnostic_report(&self) -> DiagnosticReport {
        let state = self
            .state
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        self.diagnostic_report_from(&state.shell, &state.update, &state.runtime)
    }

    fn persist(
        &self,
        state: ShellState,
        update: AppUpdateStatus,
        runtime: RuntimeDiagnosticMetadata,
    ) {
        let channel = option_env!("COMPOZY_RELEASE_CHANNEL").unwrap_or("development");
        if let Some(marker) = &self.startup_marker
            && let Err(error) = marker.update_phase(boot_phase_from_state(&state))
        {
            logging::error(format!("update desktop startup marker: {error}"));
        }
        let diagnostic_report = self.diagnostic_report_from(&state, &update, &runtime);
        let record = AppRecord::new(
            std::process::id(),
            self.started_at,
            diagnostic_report,
            state.clone(),
        )
        .with_metadata(self.app_version.clone(), channel, update);
        if let Err(error) = write_atomic(&self.home.app_record, &record) {
            logging::error(format!("write app state: {error}"));
            let write_error = record_write_failure_state(&self.state, &self.home.app_log);
            if let Some(marker) = &self.startup_marker
                && let Err(marker_error) = marker.update_phase("error")
            {
                logging::error(format!("mark app-state write failure: {marker_error}"));
            }
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

    fn diagnostic_report_from(
        &self,
        shell: &ShellState,
        update: &AppUpdateStatus,
        runtime: &RuntimeDiagnosticMetadata,
    ) -> DiagnosticReport {
        DiagnosticReport::from_snapshot(
            self.boot_id.clone(),
            self.app_version.clone(),
            self.previous_crash.clone(),
            self.startup_diagnostic.as_ref(),
            runtime,
            shell,
            update,
        )
    }
}

fn record_write_failure_state(
    state: &Mutex<PublishedState>,
    log_path: &std::path::Path,
) -> ShellState {
    let write_error = ShellState::ShellError {
        error: ShellError::new(
            ShellErrorCode::ProvisionPermission,
            "The app state could not be written.",
            log_path.to_path_buf(),
        ),
    };
    state
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner())
        .shell = write_error.clone();
    write_error
}

fn publish_ordered<State, Snapshot>(
    order: &Mutex<()>,
    state: &Mutex<State>,
    mutate: impl FnOnce(&mut State) -> Snapshot,
    persist: impl FnOnce(Snapshot),
) {
    let _publication = order
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    let snapshot = {
        let mut state = state
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        mutate(&mut state)
    };
    persist(snapshot);
}

#[cfg(test)]
mod tests {
    use std::sync::mpsc;
    use std::time::Duration;

    use super::*;

    #[test]
    fn should_never_persist_an_older_snapshot_after_a_newer_publication() {
        let order = Arc::new(Mutex::new(()));
        let state = Arc::new(Mutex::new(0_u8));
        let persisted = Arc::new(Mutex::new(0_u8));
        let (first_entered_tx, first_entered_rx) = mpsc::channel();
        let (first_release_tx, first_release_rx) = mpsc::channel();
        let first_order = Arc::clone(&order);
        let first_state = Arc::clone(&state);
        let first_persisted = Arc::clone(&persisted);
        let first = std::thread::spawn(move || {
            publish_ordered(
                &first_order,
                &first_state,
                |state| {
                    *state = 1;
                    *state
                },
                |snapshot| {
                    first_entered_tx.send(()).expect("first persist signals");
                    first_release_rx.recv().expect("first persist releases");
                    *first_persisted
                        .lock()
                        .unwrap_or_else(|poisoned| poisoned.into_inner()) = snapshot;
                },
            );
        });
        first_entered_rx
            .recv_timeout(Duration::from_secs(2))
            .expect("first publication enters persistence");
        let (second_complete_tx, second_complete_rx) = mpsc::channel();
        let second_order = Arc::clone(&order);
        let second_state = Arc::clone(&state);
        let second_persisted = Arc::clone(&persisted);
        let second = std::thread::spawn(move || {
            publish_ordered(
                &second_order,
                &second_state,
                |state| {
                    *state = 2;
                    *state
                },
                |snapshot| {
                    *second_persisted
                        .lock()
                        .unwrap_or_else(|poisoned| poisoned.into_inner()) = snapshot;
                },
            );
            second_complete_tx
                .send(())
                .expect("second publication completes");
        });
        let second_while_first_persisted =
            second_complete_rx.recv_timeout(Duration::from_millis(250));
        first_release_tx
            .send(())
            .expect("first publication releases");
        first.join().expect("first publication joins");
        second.join().expect("second publication joins");

        assert!(matches!(
            second_while_first_persisted,
            Err(mpsc::RecvTimeoutError::Timeout)
        ));
        assert_eq!(
            *persisted
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner()),
            2
        );
    }

    #[test]
    fn should_publish_a_safe_retryable_error_when_app_state_persistence_fails() {
        let state = Mutex::new(PublishedState {
            shell: ShellState::Resolving,
            update: AppUpdateStatus::default(),
            runtime: RuntimeDiagnosticMetadata::default(),
        });
        let write_error =
            record_write_failure_state(&state, std::path::Path::new("/private/home/desktop.log"));
        let published = state.lock().expect("published state lock opens");
        let report = DiagnosticReport::from_snapshot(
            "boot-write-failure",
            "0.4.1",
            None,
            None,
            &published.runtime,
            &published.shell,
            &published.update,
        );

        assert_eq!(published.shell, write_error);
        assert!(crate::controller::shell_retry_available(&published.shell));
        assert_eq!(
            report
                .current_error
                .as_ref()
                .expect("report error exists")
                .code,
            ShellErrorCode::ProvisionPermission
        );
        assert!(
            !serde_json::to_string(&report)
                .expect("report serializes")
                .contains("/private/home")
        );
    }
}
