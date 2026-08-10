use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{Duration, Instant};

use compozyos_desktop::home::CompozyHome;
use compozyos_desktop::runtime::discovery::{ProcessTable, SystemProcessTable};
use compozyos_desktop::runtime::probe::{BoundProbe, HttpStatusTransport};
use compozyos_desktop::runtime::readiness::DaemonReadiness;
use compozyos_desktop::runtime::supervisor::{
    ReadinessProbe, StartResult, Supervisor, SystemSpawner, ThreadDelay,
};

fn compile_fake_daemon(output_directory: &Path) -> PathBuf {
    let binary = output_directory.join(if cfg!(windows) {
        "fake-compozy.exe"
    } else {
        "fake-compozy"
    });
    let source = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/fake_daemon.rs");
    let output = Command::new("rustc")
        .arg("--edition=2024")
        .arg("-O")
        .arg(source)
        .arg("-o")
        .arg(&binary)
        .output()
        .expect("rustc starts");
    assert!(
        output.status.success(),
        "fake daemon compile failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    binary
}

struct FixtureProcess(Option<u32>);

impl FixtureProcess {
    fn new(pid: u32) -> Self {
        Self(Some(pid))
    }

    fn terminate(&mut self) {
        let pid = self.0.take().expect("fixture PID is still armed");
        assert!(
            terminate_fixture(pid),
            "test fixture process should terminate"
        );
    }
}

impl Drop for FixtureProcess {
    fn drop(&mut self) {
        if let Some(pid) = self.0.take()
            && !terminate_fixture(pid)
        {
            eprintln!("best-effort fake daemon cleanup failed for PID {pid}");
        }
    }
}

#[test]
fn should_spawn_real_detached_daemon_and_leave_it_alive_on_shell_quit() {
    let directory = tempfile::tempdir().expect("temp directory creates");
    let home = CompozyHome::from_root(directory.path().join("home"));
    let binary = compile_fake_daemon(directory.path());
    let processes = SystemProcessTable;
    let probe = BoundProbe::new(HttpStatusTransport, Duration::from_secs(1));
    let readiness = DaemonReadiness {
        daemon_record: &home.daemon_info,
        home: &home.root,
        processes: &processes,
        probe: &probe,
    };
    let spawner = SystemSpawner;
    let delay = ThreadDelay;
    let supervisor = Supervisor {
        spawner: &spawner,
        readiness: &readiness,
        delay: &delay,
        readiness_deadline: Duration::from_secs(5),
        poll_interval: Duration::from_millis(25),
    };
    let result = supervisor
        .start_owned(&binary, &home)
        .expect("fake daemon becomes ready");
    let StartResult::Owned(runtime) = result else {
        panic!("fresh fake daemon should be owned by this start");
    };
    let mut fixture = FixtureProcess::new(runtime.pid);

    supervisor.on_quit(Some(runtime.pid));
    assert!(processes.start_time(runtime.pid).is_some());
    assert!(readiness.ready(Some(runtime.pid)).is_some());

    fixture.terminate();
    let deadline = Instant::now() + Duration::from_secs(3);
    while processes.start_time(runtime.pid).is_some() && Instant::now() < deadline {
        std::thread::sleep(Duration::from_millis(25));
    }
    assert!(processes.start_time(runtime.pid).is_none());
}

#[cfg(unix)]
fn terminate_fixture(pid: u32) -> bool {
    // SAFETY: the PID was returned by the test-owned fake daemon spawn.
    let result = unsafe { libc::kill(pid as i32, libc::SIGTERM) };
    if result != 0 {
        return false;
    }
    let mut status = 0;
    // SAFETY: the fake daemon is a direct child of this test process.
    (unsafe { libc::waitpid(pid as i32, &mut status, 0) }) == pid as i32
}

#[cfg(windows)]
fn terminate_fixture(pid: u32) -> bool {
    match Command::new("taskkill")
        .args(["/PID", &pid.to_string(), "/T", "/F"])
        .status()
    {
        Ok(status) => status.success(),
        Err(error) => {
            eprintln!("start taskkill for fixture: {error}");
            false
        }
    }
}
