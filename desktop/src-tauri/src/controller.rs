use std::sync::{Arc, Mutex};

use chrono::Utc;
use serde_json::{Value, json};
use tauri::{AppHandle, Wry};

use crate::app_state::AppStatePublisher;
use crate::boot_window;
use crate::control::{ControlError, ControlHandler};
use crate::control_events::{app_event_json, runtime_event_json, runtime_update_error_code};
use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::links::{LinkQueue, LinkTarget, parse};
use crate::record::{AppUpdatePhase, RuntimeUpdatePhase};
use crate::state::{ShellState, UpdateTarget};
use crate::update::app_update::{AppUpdateEvent, AppUpdater};
use crate::update::runtime_update::{RuntimeUpdateEvent, RuntimeUpdater};
use crate::windowing;

type RetryAction = Arc<dyn Fn() + Send + Sync>;

pub struct DesktopController {
    app: AppHandle<Wry>,
    home: CompozyHome,
    links: Arc<LinkQueue>,
    publisher: AppStatePublisher,
    updates_enabled: bool,
    app_updater: Option<Arc<AppUpdater>>,
    runtime_updater: Mutex<Option<Arc<RuntimeUpdater>>>,
    runtime_recommendation: Mutex<Option<String>>,
    app_fallback_required: Mutex<bool>,
    retry: Mutex<Option<RetryAction>>,
}

impl DesktopController {
    pub fn new(
        app: AppHandle<Wry>,
        home: CompozyHome,
        links: Arc<LinkQueue>,
        publisher: AppStatePublisher,
        app_updater: Option<Arc<AppUpdater>>,
        updates_enabled: bool,
    ) -> Self {
        Self {
            app,
            home,
            links,
            publisher,
            updates_enabled,
            app_updater,
            runtime_updater: Mutex::new(None),
            runtime_recommendation: Mutex::new(None),
            app_fallback_required: Mutex::new(false),
            retry: Mutex::new(None),
        }
    }

    pub fn set_runtime_updater(&self, updater: Option<Arc<RuntimeUpdater>>) {
        *self
            .runtime_updater
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = updater;
    }

    pub fn automatic_updates_enabled(&self) -> bool {
        self.updates_enabled
    }

    pub fn set_retry(&self, retry: RetryAction) {
        *self
            .retry
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = Some(retry);
    }

    pub fn check_updates(&self) -> Value {
        if !self.updates_enabled {
            return json!({"app": app_event_json(&AppUpdateEvent::Disabled), "runtime": null});
        }
        let app = self.check_app_update_with(true);
        let runtime = self.check_runtime_update();
        self.present_pending_updates();
        json!({"app": app, "runtime": runtime})
    }

    pub fn check_app_update(&self) -> Option<Value> {
        self.check_app_update_with(self.updates_enabled)
    }

    fn check_app_update_with(&self, enabled: bool) -> Option<Value> {
        self.app_updater.as_ref().map(|updater| {
            let event = updater.check(enabled);
            self.record_app_event(&event);
            app_event_json(&event)
        })
    }

    pub fn check_runtime_update(&self) -> Option<Value> {
        let updater = self
            .runtime_updater
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .clone();
        updater.map(|updater| match updater.check() {
            Ok(event) => {
                self.record_runtime_event(&event);
                runtime_event_json(&event)
            }
            Err(error) => {
                let event = RuntimeUpdateEvent::Failed {
                    message: error.to_string(),
                };
                self.record_runtime_event(&event);
                runtime_event_json(&event)
            }
        })
    }

    fn apply_update(&self, target: &str) -> Result<Value, ControlError> {
        let previous_shell = self.publisher.shell_state();
        match target {
            "app" => {
                let updater = self.app_updater.as_ref().ok_or_else(|| {
                    ControlError::new("app_update_unavailable", "App updates are disabled.")
                })?;
                if let Some(version) = updater.ready_version() {
                    self.record_app_event(&AppUpdateEvent::Applying { version });
                    self.publisher.publish(ShellState::Updating {
                        target: UpdateTarget::App,
                    });
                }
                let event = match updater.apply(true) {
                    Ok(event) => event,
                    Err(error) => {
                        self.record_app_event(&AppUpdateEvent::Failed {
                            checked_at: Utc::now(),
                            manual_fallback: true,
                            message: error.to_string(),
                        });
                        self.publisher.publish(previous_shell);
                        if let Some(error) = self.publisher.status().last_error
                            && let Err(render_error) =
                                boot_window::show_error_notice(&self.app, &error)
                        {
                            crate::logging::error(format!(
                                "present failed app update: {render_error}"
                            ));
                        }
                        return Err(ControlError::new("app_update_failed", error.to_string()));
                    }
                };
                self.record_app_event(&event);
                Ok(app_event_json(&event))
            }
            "runtime" => {
                let updater = self
                    .runtime_updater
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner())
                    .clone()
                    .ok_or_else(|| {
                        ControlError::new(
                            "runtime_update_unavailable",
                            "The runtime is not ready for an update.",
                        )
                    })?;
                if let Ok(RuntimeUpdateEvent::Available { version }) = updater.check() {
                    self.record_runtime_event(&RuntimeUpdateEvent::Applying { version });
                    self.publisher.publish(ShellState::Updating {
                        target: UpdateTarget::Runtime,
                    });
                }
                let event = match updater.apply(true) {
                    Ok(event) => event,
                    Err(error) => {
                        let event = RuntimeUpdateEvent::Failed {
                            message: error.to_string(),
                        };
                        self.record_runtime_event(&event);
                        self.publisher.publish(ShellState::ShellError {
                            error: ShellError::new(
                                runtime_update_error_code(&error),
                                error.to_string(),
                                self.home.app_log.clone(),
                            ),
                        });
                        return Err(ControlError::new(
                            "runtime_update_failed",
                            error.to_string(),
                        ));
                    }
                };
                self.record_runtime_event(&event);
                match &event {
                    RuntimeUpdateEvent::Completed { .. } => {
                        self.publisher.publish(previous_shell);
                        if let Err(error) = boot_window::close(&self.app) {
                            crate::logging::error(format!(
                                "close completed update window: {error}"
                            ));
                        }
                        if let Err(error) = windowing::focus_existing(&self.app, &self.links) {
                            crate::logging::error(format!("focus product after update: {error}"));
                        }
                    }
                    RuntimeUpdateEvent::RecoveryRequired { .. } => {
                        self.publisher.publish(ShellState::ShellError {
                            error: ShellError::from_code(
                                ShellErrorCode::MigrationRecoveryRequired,
                                self.home.app_log.clone(),
                            ),
                        });
                    }
                    _ => {}
                }
                Ok(runtime_event_json(&event))
            }
            _ => Err(ControlError::new(
                "invalid_update_target",
                "The update target must be app or runtime.",
            )),
        }
    }

    fn navigate(&self, params: &Value) -> Result<Value, ControlError> {
        let path = params.get("path").and_then(Value::as_str).ok_or_else(|| {
            ControlError::new("invalid_target_path", "A product path is required.")
        })?;
        let target = parse(&format!("compozyos://open{path}"));
        let LinkTarget::ProductPath(path) = target else {
            return Err(ControlError::new(
                "invalid_target_path",
                "The product path is invalid.",
            ));
        };
        self.links.push(&format!("compozyos://open{path}"));
        windowing::focus_existing(&self.app, &self.links).map_err(|_| {
            ControlError::new(
                "app_control_unavailable",
                "The app window could not be focused.",
            )
        })?;
        Ok(json!({"navigated": path}))
    }

    fn retry(&self) -> Result<Value, ControlError> {
        let status = self.publisher.status();
        if status.app_state == AppUpdatePhase::Failed
            || status.runtime_state == RuntimeUpdatePhase::Failed
        {
            return Ok(json!({"accepted": true, "update": self.check_updates()}));
        }
        if !shell_retry_available(&self.publisher.shell_state()) {
            return Err(ControlError::new(
                "retry_unavailable",
                "There is no operation to retry.",
            ));
        }
        let retry = self
            .retry
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .clone()
            .ok_or_else(|| {
                ControlError::new("retry_unavailable", "There is no operation to retry.")
            })?;
        retry();
        Ok(json!({"accepted": true}))
    }

    pub fn present_pending_updates(&self) {
        if !matches!(self.publisher.shell_state(), ShellState::Product { .. }) {
            return;
        }
        let status = self.publisher.status();
        let app_fallback_required = *self
            .app_fallback_required
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        let result = if matches!(
            status.runtime_state,
            RuntimeUpdatePhase::RecoveryRequired | RuntimeUpdatePhase::Failed
        ) || (status.app_state == AppUpdatePhase::Failed && app_fallback_required)
        {
            status
                .last_error
                .as_ref()
                .map(|error| boot_window::show_error_notice(&self.app, error))
        } else if status.app_state == AppUpdatePhase::Ready {
            Some(boot_window::show_update_offer(&self.app, UpdateTarget::App))
        } else if status.runtime_state == RuntimeUpdatePhase::Available {
            Some(boot_window::show_update_offer(
                &self.app,
                UpdateTarget::Runtime,
            ))
        } else if status.runtime_state == RuntimeUpdatePhase::Managed {
            self.runtime_recommendation
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .as_deref()
                .map(|recommendation| boot_window::show_managed_update(&self.app, recommendation))
        } else {
            None
        };
        if let Some(Err(error)) = result {
            crate::logging::error(format!("present update state: {error}"));
        }
    }

    fn diagnose(&self) -> Value {
        let status = self.publisher.status();
        json!({
            "app_log": self.home.app_log,
            "runtime_log": self.home.runtime_log,
            "last_error": status.last_error,
        })
    }

    pub fn record_app_event(&self, event: &AppUpdateEvent) {
        *self
            .app_fallback_required
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = matches!(
            event,
            AppUpdateEvent::Failed {
                manual_fallback: true,
                ..
            }
        );
        self.publisher.update_status(|status| match event {
            AppUpdateEvent::Disabled => {}
            AppUpdateEvent::UpToDate { checked_at } => {
                status.app_state = AppUpdatePhase::Idle;
                status.app_available = None;
                status.last_checked_at = Some(*checked_at);
                status.last_error = None;
            }
            AppUpdateEvent::Ready {
                version,
                checked_at,
            } => {
                status.app_state = AppUpdatePhase::Ready;
                status.app_available = Some(version.clone());
                status.last_checked_at = Some(*checked_at);
                status.last_error = None;
            }
            AppUpdateEvent::Failed {
                checked_at,
                message,
                ..
            } => {
                status.app_state = AppUpdatePhase::Failed;
                status.last_checked_at = Some(*checked_at);
                status.last_error = Some(ShellError::new(
                    ShellErrorCode::ProvisionNetwork,
                    message,
                    self.home.app_log.clone(),
                ));
            }
            AppUpdateEvent::Applying { version } => {
                status.app_state = AppUpdatePhase::Ready;
                status.app_available = Some(version.clone());
            }
        });
    }

    pub fn record_runtime_event(&self, event: &RuntimeUpdateEvent) {
        *self
            .runtime_recommendation
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = match event {
            RuntimeUpdateEvent::Managed { recommendation, .. } => Some(recommendation.clone()),
            _ => None,
        };
        self.publisher.update_status(|status| match event {
            RuntimeUpdateEvent::Idle => {
                status.runtime_state = RuntimeUpdatePhase::Idle;
                status.runtime_available = None;
                status.last_error = None;
            }
            RuntimeUpdateEvent::Available { version }
            | RuntimeUpdateEvent::Declined { version } => {
                status.runtime_state = RuntimeUpdatePhase::Available;
                status.runtime_available = Some(version.clone());
            }
            RuntimeUpdateEvent::Managed { version, .. } => {
                status.runtime_state = RuntimeUpdatePhase::Managed;
                status.runtime_available = Some(version.clone());
            }
            RuntimeUpdateEvent::Applying { version } => {
                status.runtime_state = RuntimeUpdatePhase::Applying;
                status.runtime_available = Some(version.clone());
            }
            RuntimeUpdateEvent::Completed { .. } => {
                status.runtime_state = RuntimeUpdatePhase::Idle;
                status.runtime_available = None;
                status.last_error = None;
            }
            RuntimeUpdateEvent::Failed { message } => {
                status.runtime_state = RuntimeUpdatePhase::Failed;
                status.last_error = Some(ShellError::new(
                    ShellErrorCode::RuntimeStartFailed,
                    message,
                    self.home.app_log.clone(),
                ));
            }
            RuntimeUpdateEvent::RecoveryRequired { version } => {
                status.runtime_state = RuntimeUpdatePhase::RecoveryRequired;
                status.runtime_available = Some(version.clone());
                status.last_error = Some(ShellError::from_code(
                    ShellErrorCode::MigrationRecoveryRequired,
                    self.home.app_log.clone(),
                ));
            }
        });
    }
}

fn shell_retry_available(state: &ShellState) -> bool {
    matches!(state, ShellState::ShellError { .. })
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;
    use crate::errors::ShellErrorCode;
    use url::Url;

    #[test]
    fn should_offer_shell_retry_only_for_a_designed_error_state() {
        assert!(shell_retry_available(&ShellState::ShellError {
            error: ShellError::from_code(
                ShellErrorCode::LoadDeadlineExceeded,
                PathBuf::from("/tmp/desktop.log"),
            ),
        }));
        assert!(!shell_retry_available(&ShellState::Resolving));
        assert!(!shell_retry_available(&ShellState::Product {
            origin: Url::parse("http://127.0.0.1:2123").expect("fixture origin is valid"),
            owned: true,
        }));
    }
}

impl ControlHandler for DesktopController {
    fn handle(&self, method: &str, params: Value) -> Result<Value, ControlError> {
        match method {
            "navigate" => self.navigate(&params),
            "update.check" => Ok(self.check_updates()),
            "update.apply" => {
                let target = params
                    .get("target")
                    .and_then(Value::as_str)
                    .ok_or_else(|| {
                        ControlError::new("invalid_update_target", "An update target is required.")
                    })?;
                self.apply_update(target)
            }
            "retry" => self.retry(),
            "diagnose" => Ok(self.diagnose()),
            _ => Err(ControlError::new(
                "app_control_unknown_method",
                "The app control method is not supported.",
            )),
        }
    }
}

#[tauri::command]
pub async fn shell_control(
    controller: tauri::State<'_, Arc<DesktopController>>,
    method: String,
    params: Value,
) -> Result<Value, ControlError> {
    let controller = Arc::clone(controller.inner());
    let method = method.trim().to_owned();
    tauri::async_runtime::spawn_blocking(move || controller.handle(&method, params))
        .await
        .map_err(|error| {
            crate::logging::error(format!("join app control command: {error}"));
            ControlError::new(
                "app_control_unavailable",
                "The app control command could not be completed.",
            )
        })?
}
