use serde_json::{Value, json};

use crate::errors::ShellErrorCode;
use crate::runtime::mutation::MutationError;
use crate::update::app_update::AppUpdateEvent;
use crate::update::runtime_update::{RuntimeUpdateEvent, RuntimeUpdaterError};

pub(crate) fn app_event_json(event: &AppUpdateEvent) -> Value {
    match event {
        AppUpdateEvent::Disabled => json!({"state": "idle", "enabled": false}),
        AppUpdateEvent::UpToDate { checked_at } => {
            json!({"state": "idle", "checked_at": checked_at})
        }
        AppUpdateEvent::Ready {
            version,
            checked_at,
        } => json!({"state": "ready", "version": version, "checked_at": checked_at}),
        AppUpdateEvent::Failed {
            checked_at,
            manual_fallback,
            message,
        } => json!({
            "state": "failed",
            "checked_at": checked_at,
            "manual_fallback": manual_fallback,
            "message": message,
        }),
        AppUpdateEvent::Applying { version } => {
            json!({"state": "applying", "version": version})
        }
    }
}

pub(crate) fn runtime_event_json(event: &RuntimeUpdateEvent) -> Value {
    match event {
        RuntimeUpdateEvent::Idle => json!({"state": "idle"}),
        RuntimeUpdateEvent::Available { version } => {
            json!({"state": "available", "version": version})
        }
        RuntimeUpdateEvent::Managed {
            version,
            recommendation,
        } => json!({
            "state": "managed",
            "version": version,
            "recommendation": recommendation,
        }),
        RuntimeUpdateEvent::Declined { version } => {
            json!({"state": "available", "version": version, "declined": true})
        }
        RuntimeUpdateEvent::Applying { version } => {
            json!({"state": "applying", "version": version})
        }
        RuntimeUpdateEvent::Completed { version } => {
            json!({"state": "idle", "version": version})
        }
        RuntimeUpdateEvent::Failed { message } => {
            json!({"state": "failed", "message": message})
        }
        RuntimeUpdateEvent::RecoveryRequired { version } => {
            json!({"state": "recovery_required", "version": version})
        }
    }
}

pub(crate) fn runtime_update_error_code(error: &RuntimeUpdaterError) -> ShellErrorCode {
    match error {
        RuntimeUpdaterError::Mutation(MutationError::Locked) => ShellErrorCode::UpdateLockHeld,
        RuntimeUpdaterError::Mutation(MutationError::NotOwned) => ShellErrorCode::NotOwned,
        RuntimeUpdaterError::Quiesce(_) => ShellErrorCode::QuiesceFailed,
        _ => ShellErrorCode::RuntimeStartFailed,
    }
}
