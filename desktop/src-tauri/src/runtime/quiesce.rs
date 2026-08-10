use std::thread;
use std::time::Duration;

use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};
use thiserror::Error;
use url::Url;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DrainStatus {
    pub state: String,
    pub admission_closed: bool,
    pub active_session_executions: u64,
    pub active_task_claims: u64,
    pub authoritative_claims_settled: bool,
    pub session_execution_settled: bool,
    pub safe_to_stop: bool,
}

impl DrainStatus {
    pub fn is_safe(&self) -> bool {
        self.state == "draining"
            && self.admission_closed
            && self.authoritative_claims_settled
            && self.session_execution_settled
            && self.active_session_executions == 0
            && self.active_task_claims == 0
            && self.safe_to_stop
    }
}

#[derive(Debug, Error)]
pub enum QuiesceError {
    #[error("daemon quiesce request failed")]
    Transport,
    #[error("daemon quiesce readout is inconsistent")]
    InvalidReadout,
    #[error("daemon work did not settle before the deadline")]
    NotSettled,
    #[error("daemon was no longer safe to stop at revalidation")]
    RevalidationFailed,
}

pub trait QuiesceTransport: Send + Sync {
    fn drain(&self) -> Result<DrainStatus, QuiesceError>;
    fn undrain(&self) -> Result<DrainStatus, QuiesceError>;
}

pub struct HttpQuiesceTransport {
    client: Client,
    drain_url: Url,
    undrain_url: Url,
}

impl HttpQuiesceTransport {
    pub fn new(origin: &Url, timeout: Duration) -> Result<Self, QuiesceError> {
        let client = Client::builder()
            .timeout(timeout)
            .build()
            .map_err(|_| QuiesceError::Transport)?;
        let drain_url = origin
            .join("api/drain")
            .map_err(|_| QuiesceError::Transport)?;
        let undrain_url = origin
            .join("api/undrain")
            .map_err(|_| QuiesceError::Transport)?;
        Ok(Self {
            client,
            drain_url,
            undrain_url,
        })
    }

    fn post(&self, url: &Url) -> Result<DrainStatus, QuiesceError> {
        let response = self
            .client
            .post(url.clone())
            .send()
            .map_err(|_| QuiesceError::Transport)?;
        if !response.status().is_success() {
            return Err(QuiesceError::Transport);
        }
        response.json().map_err(|_| QuiesceError::InvalidReadout)
    }
}

impl QuiesceTransport for HttpQuiesceTransport {
    fn drain(&self) -> Result<DrainStatus, QuiesceError> {
        self.post(&self.drain_url)
    }

    fn undrain(&self) -> Result<DrainStatus, QuiesceError> {
        self.post(&self.undrain_url)
    }
}

pub struct QuiesceGuard<'a> {
    transport: &'a dyn QuiesceTransport,
    active: bool,
}

impl<'a> QuiesceGuard<'a> {
    pub fn prepare(
        transport: &'a dyn QuiesceTransport,
        attempts: usize,
        poll_interval: Duration,
    ) -> Result<Self, QuiesceError> {
        if attempts == 0 {
            return Err(QuiesceError::NotSettled);
        }
        let mut guard = Self {
            transport,
            active: true,
        };
        for attempt in 0..attempts {
            let status = transport.drain()?;
            validate_readout(&status)?;
            if status.is_safe() {
                return Ok(guard);
            }
            if attempt + 1 < attempts {
                thread::sleep(poll_interval);
            }
        }
        guard.abort()?;
        Err(QuiesceError::NotSettled)
    }

    pub fn revalidate(&mut self) -> Result<(), QuiesceError> {
        let status = self.transport.drain()?;
        validate_readout(&status)?;
        if status.is_safe() {
            return Ok(());
        }
        self.abort()?;
        Err(QuiesceError::RevalidationFailed)
    }

    pub fn commit(mut self) {
        self.active = false;
    }

    pub fn abort(&mut self) -> Result<(), QuiesceError> {
        if !self.active {
            return Ok(());
        }
        let status = self.transport.undrain()?;
        if status.state != "active" || status.admission_closed {
            return Err(QuiesceError::InvalidReadout);
        }
        self.active = false;
        Ok(())
    }
}

impl Drop for QuiesceGuard<'_> {
    fn drop(&mut self) {
        if self.active
            && let Err(error) = self.transport.undrain()
        {
            crate::logging::error(format!(
                "restore daemon admission after update abort: {error}"
            ));
        }
    }
}

fn validate_readout(status: &DrainStatus) -> Result<(), QuiesceError> {
    let derived_safe = status.state == "draining"
        && status.admission_closed
        && status.authoritative_claims_settled
        && status.session_execution_settled
        && status.active_session_executions == 0
        && status.active_task_claims == 0;
    if status.safe_to_stop != derived_safe {
        return Err(QuiesceError::InvalidReadout);
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::collections::VecDeque;
    use std::sync::Mutex;

    use super::*;

    struct FakeTransport {
        drain: Mutex<VecDeque<DrainStatus>>,
        undrain_calls: Mutex<usize>,
    }

    impl FakeTransport {
        fn new(statuses: Vec<DrainStatus>) -> Self {
            Self {
                drain: Mutex::new(statuses.into()),
                undrain_calls: Mutex::new(0),
            }
        }

        fn undrain_calls(&self) -> usize {
            *self
                .undrain_calls
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
        }
    }

    impl QuiesceTransport for FakeTransport {
        fn drain(&self) -> Result<DrainStatus, QuiesceError> {
            self.drain
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner())
                .pop_front()
                .ok_or(QuiesceError::Transport)
        }

        fn undrain(&self) -> Result<DrainStatus, QuiesceError> {
            *self
                .undrain_calls
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner()) += 1;
            Ok(active())
        }
    }

    fn safe() -> DrainStatus {
        DrainStatus {
            state: "draining".to_owned(),
            admission_closed: true,
            active_session_executions: 0,
            active_task_claims: 0,
            authoritative_claims_settled: true,
            session_execution_settled: true,
            safe_to_stop: true,
        }
    }

    fn unsettled() -> DrainStatus {
        DrainStatus {
            active_task_claims: 1,
            authoritative_claims_settled: false,
            safe_to_stop: false,
            ..safe()
        }
    }

    fn active() -> DrainStatus {
        DrainStatus {
            state: "active".to_owned(),
            admission_closed: false,
            safe_to_stop: false,
            ..safe()
        }
    }

    #[test]
    fn should_wait_for_authoritative_work_to_settle_and_undrain_on_decline() {
        let transport = FakeTransport::new(vec![unsettled(), safe()]);
        let guard = QuiesceGuard::prepare(&transport, 2, Duration::ZERO)
            .expect("quiesce reaches safe state");
        drop(guard);
        assert_eq!(transport.undrain_calls(), 1);
    }

    #[test]
    fn should_abort_and_restore_admission_when_revalidation_turns_unsafe() {
        let transport = FakeTransport::new(vec![safe(), unsettled()]);
        let mut guard =
            QuiesceGuard::prepare(&transport, 1, Duration::ZERO).expect("initial quiesce is safe");
        assert!(matches!(
            guard.revalidate(),
            Err(QuiesceError::RevalidationFailed)
        ));
        assert_eq!(transport.undrain_calls(), 1);
    }
}
