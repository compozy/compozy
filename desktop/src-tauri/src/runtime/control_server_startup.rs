use std::sync::{Arc, Mutex};

use tauri::{AppHandle, Manager, Wry};

use crate::app_state::AppStatePublisher;
use crate::control::ControlServer;
use crate::controller::DesktopController;
use crate::home::CompozyHome;
use crate::logging;
use crate::state::ShellState;

use super::startup_failure::control_server_error;

pub(crate) fn show_boot_before_starting_control<E>(
    show_boot: impl FnOnce() -> Result<(), E>,
    show_boot_failure: impl FnOnce(E),
    start_control: impl FnOnce() -> bool,
) -> bool {
    match show_boot() {
        Ok(()) => start_control(),
        Err(error) => {
            show_boot_failure(error);
            false
        }
    }
}

pub(crate) struct ControlServerStartup {
    app: AppHandle<Wry>,
    home: CompozyHome,
    controller: Arc<DesktopController>,
    publisher: AppStatePublisher,
    start_lock: Mutex<()>,
}

impl ControlServerStartup {
    pub(crate) fn new(
        app: AppHandle<Wry>,
        home: CompozyHome,
        controller: Arc<DesktopController>,
        publisher: AppStatePublisher,
    ) -> Self {
        Self {
            app,
            home,
            controller,
            publisher,
            start_lock: Mutex::new(()),
        }
    }

    pub(crate) fn ensure_started(&self) -> bool {
        let _startup = self
            .start_lock
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        if self.app.try_state::<ControlServer>().is_some() {
            return true;
        }
        let server = match ControlServer::start(&self.home.app_socket, self.controller.clone()) {
            Ok(server) => server,
            Err(error) => {
                self.publish_failure(error);
                return false;
            }
        };
        if self.app.manage(server) {
            return true;
        }
        if self.app.try_state::<ControlServer>().is_some() {
            log::debug!("desktop control server state was already managed");
            return true;
        }
        self.publish_failure("control server state was already managed");
        false
    }

    fn publish_failure(&self, error: impl std::fmt::Display) {
        let shell_error = control_server_error(&self.home);
        logging::error(format!(
            "start desktop control server ({:?}): {error}",
            shell_error.code
        ));
        self.publisher
            .publish(ShellState::ShellError { error: shell_error });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_keep_boot_available_when_control_start_fails() {
        let mut boot_visible = false;

        let can_start_runtime = show_boot_before_starting_control(
            || {
                boot_visible = true;
                Ok::<_, ()>(())
            },
            |_| panic!("boot presentation succeeds"),
            || false,
        );

        assert!(boot_visible);
        assert!(!can_start_runtime);
    }
}
