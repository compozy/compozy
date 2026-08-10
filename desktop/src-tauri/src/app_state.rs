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
    publish_order: Arc<Mutex<()>>,
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
            publish_order: Arc::new(Mutex::new(())),
            state: Arc::new(Mutex::new(PublishedState {
                shell: ShellState::Resolving,
                update: AppUpdateStatus::default(),
            })),
        }
    }

    pub fn publish(&self, state: ShellState) {
        publish_ordered(
            &self.publish_order,
            &self.state,
            |published| {
                published.shell = state.clone();
                (state, published.update.clone())
            },
            |(state, update)| self.persist(state, update),
        );
    }

    pub fn update_status(&self, mutate: impl FnOnce(&mut AppUpdateStatus)) {
        publish_ordered(
            &self.publish_order,
            &self.state,
            |published| {
                mutate(&mut published.update);
                (published.shell.clone(), published.update.clone())
            },
            |(shell, update)| self.persist(shell, update),
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
}
