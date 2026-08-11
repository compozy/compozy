use std::sync::Arc;
use std::time::Duration;

use crate::controller::DesktopController;

pub fn spawn(controller: Arc<DesktopController>, interval: Duration) {
    let controller = Arc::downgrade(&controller);
    std::thread::spawn(move || {
        loop {
            std::thread::sleep(interval);
            let Some(controller) = controller.upgrade() else {
                break;
            };
            controller.check_updates();
        }
    });
}
