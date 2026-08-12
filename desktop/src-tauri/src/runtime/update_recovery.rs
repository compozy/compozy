use crate::app_state::AppStatePublisher;
use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
use crate::logging;
use crate::state::ShellState;
use crate::update::runtime_update::{RuntimeLifecycle, RuntimeUpdaterError};

use super::discovery::SystemProcessTable;
use super::mutation::{
    MutationError, RecoveryPlan, UpdateLock, complete_recovery, prepare_recovery, read_journal,
};
use super::update_lifecycle::SystemRuntimeLifecycle;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PendingRuntimeRecovery {
    Clean,
    RecoveryRequired,
}

#[derive(Debug)]
pub enum PendingRuntimeRecoveryError {
    Lock(MutationError),
    Recovery(MutationError),
    Lifecycle(RuntimeUpdaterError),
}

pub fn recover_pending_runtime(
    home: &CompozyHome,
    resume_recovery: bool,
) -> Result<PendingRuntimeRecovery, PendingRuntimeRecoveryError> {
    let Some(_journal) =
        read_journal(&home.update_journal).map_err(PendingRuntimeRecoveryError::Recovery)?
    else {
        return Ok(PendingRuntimeRecovery::Clean);
    };
    let processes = SystemProcessTable;
    let lock = UpdateLock::acquire(&home.update_lock, &processes)
        .map_err(PendingRuntimeRecoveryError::Lock)?;
    let plan = prepare_recovery(home).map_err(PendingRuntimeRecoveryError::Recovery)?;
    let Some(plan) = plan else {
        lock.release()
            .map_err(PendingRuntimeRecoveryError::Recovery)?;
        return Ok(PendingRuntimeRecovery::Clean);
    };
    let recovery_required = matches!(&plan, RecoveryPlan::RecoveryRequired(_));
    let journal = match plan {
        RecoveryPlan::ResumeNewRuntime(journal) => Some(journal),
        RecoveryPlan::RecoveryRequired(journal) if resume_recovery => Some(journal),
        RecoveryPlan::RecoveryRequired(_)
        | RecoveryPlan::CleanRollback
        | RecoveryPlan::Complete => None,
    };
    let Some(journal) = journal else {
        lock.release()
            .map_err(PendingRuntimeRecoveryError::Recovery)?;
        return Ok(if recovery_required {
            PendingRuntimeRecovery::RecoveryRequired
        } else {
            PendingRuntimeRecovery::Clean
        });
    };
    let lifecycle = SystemRuntimeLifecycle::new(home.clone());
    lifecycle
        .start_and_verify(&home.app_binary, &journal.to_version)
        .map_err(PendingRuntimeRecoveryError::Lifecycle)?;
    complete_recovery(home, journal).map_err(PendingRuntimeRecoveryError::Recovery)?;
    lock.release()
        .map_err(PendingRuntimeRecoveryError::Recovery)?;
    Ok(PendingRuntimeRecovery::Clean)
}

pub fn recover_or_publish(
    home: &CompozyHome,
    publisher: &AppStatePublisher,
    resume_recovery: bool,
) -> bool {
    match recover_pending_runtime(home, resume_recovery) {
        Ok(PendingRuntimeRecovery::Clean) => true,
        Ok(PendingRuntimeRecovery::RecoveryRequired) => {
            publish_error(home, publisher, ShellErrorCode::MigrationRecoveryRequired);
            false
        }
        Err(PendingRuntimeRecoveryError::Lock(error)) => {
            logging::error(format!("acquire runtime recovery lock: {error}"));
            publish_error(home, publisher, ShellErrorCode::UpdateLockHeld);
            false
        }
        Err(PendingRuntimeRecoveryError::Recovery(error)) => {
            logging::error(format!("recover runtime update: {error}"));
            publish_error(home, publisher, ShellErrorCode::MigrationRecoveryRequired);
            false
        }
        Err(PendingRuntimeRecoveryError::Lifecycle(error)) => {
            logging::error(format!("resume updated runtime: {error}"));
            publish_error(home, publisher, ShellErrorCode::MigrationRecoveryRequired);
            false
        }
    }
}

fn publish_error(home: &CompozyHome, publisher: &AppStatePublisher, code: ShellErrorCode) {
    publisher.publish(ShellState::ShellError {
        error: ShellError::from_code(code, home.app_log.clone()),
    });
}
