#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum HealthTransition {
    None,
    Disconnected,
    Reconnected,
}

#[derive(Debug)]
pub struct HealthTracker {
    connected: bool,
    failures: u8,
    failure_threshold: u8,
}

impl HealthTracker {
    pub fn connected(failure_threshold: u8) -> Self {
        Self {
            connected: true,
            failures: 0,
            failure_threshold: failure_threshold.max(1),
        }
    }

    pub fn observe(&mut self, healthy: bool) -> HealthTransition {
        if healthy {
            self.failures = 0;
            if !self.connected {
                self.connected = true;
                return HealthTransition::Reconnected;
            }
            return HealthTransition::None;
        }
        self.failures = self.failures.saturating_add(1);
        if self.connected && self.failures >= self.failure_threshold {
            self.connected = false;
            return HealthTransition::Disconnected;
        }
        HealthTransition::None
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_disconnect_after_threshold_and_reconnect_on_first_healthy_probe() {
        let mut tracker = HealthTracker::connected(3);
        assert_eq!(tracker.observe(false), HealthTransition::None);
        assert_eq!(tracker.observe(false), HealthTransition::None);
        assert_eq!(tracker.observe(false), HealthTransition::Disconnected);
        assert_eq!(tracker.observe(false), HealthTransition::None);
        assert_eq!(tracker.observe(true), HealthTransition::Reconnected);
        assert_eq!(tracker.observe(true), HealthTransition::None);
    }
}
