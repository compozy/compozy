use std::sync::Arc;
use std::time::Duration;

use crate::controller::DesktopController;

pub fn spawn(controller: Arc<DesktopController>, interval: Duration) {
    std::thread::spawn(move || {
        loop {
            std::thread::sleep(interval);
            controller.check_updates();
        }
    });
}
