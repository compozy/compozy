use std::fmt::Display;

use crate::app_state::AppStatePublisher;
use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::logging;
use crate::state::ShellState;

/// Maps setup failures to diagnostics that can be persisted before the runtime starts.
pub(crate) fn control_server_error(home: &CompozyHome) -> ShellError {
    ShellError::new(
        ShellErrorCode::DesktopControlFailed,
        "The desktop app could not start its local control service.",
        home.app_log.clone(),
    )
}

pub(crate) fn boot_presentation_error(home: &CompozyHome) -> ShellError {
    ShellError::new(
        ShellErrorCode::BootWindowFailed,
        "The desktop startup screen could not be shown.",
        home.app_log.clone(),
    )
}

pub(crate) fn publish_boot_presentation_failure(
    publisher: &AppStatePublisher,
    home: &CompozyHome,
    error: impl Display,
) {
    let shell_error = boot_presentation_error(home);
    logging::error(format!(
        "show desktop startup screen ({:?}): {error}",
        shell_error.code
    ));
    publisher.publish(ShellState::ShellError { error: shell_error });
}

pub(crate) fn development_runtime_error(home: &CompozyHome) -> ShellError {
    ShellError::new(
        ShellErrorCode::RuntimeStartFailed,
        "Run `make desktop-dev` to start the development stack.",
        home.app_log.clone(),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_direct_a_development_build_without_release_configuration_to_desktop_dev() {
        let home = CompozyHome::from_root(std::path::PathBuf::from("/tmp/compozy-home"));

        assert_eq!(
            development_runtime_error(&home).safe_message,
            "Run `make desktop-dev` to start the development stack."
        );
    }

    #[test]
    fn should_use_typed_errors_for_control_and_boot_setup_failures() {
        let home = CompozyHome::from_root(std::path::PathBuf::from("/tmp/compozy-home"));

        assert_eq!(
            control_server_error(&home).code,
            ShellErrorCode::DesktopControlFailed
        );
        assert_eq!(
            boot_presentation_error(&home).code,
            ShellErrorCode::BootWindowFailed
        );
    }
}
