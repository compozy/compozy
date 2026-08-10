use std::sync::Mutex;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};

use chrono::Utc;

use super::*;
use crate::runtime::discovery::DaemonRecord;
use crate::runtime::probe::{DaemonStatusPayload, StatusPayload};

struct FakeSpawner {
    calls: AtomicUsize,
    pid: u32,
}

impl ProcessSpawner for FakeSpawner {
    fn spawn_detached(&self, _binary: &Path, _home: &CompozyHome) -> io::Result<u32> {
        self.calls.fetch_add(1, Ordering::SeqCst);
        Ok(self.pid)
    }
}

struct FakeReadiness {
    polls: AtomicUsize,
    ready_after: usize,
    identity: BoundDaemonIdentity,
    alive: bool,
}

impl ReadinessProbe for FakeReadiness {
    fn ready(&self, expected: Option<u32>) -> Option<BoundDaemonIdentity> {
        let poll = self.polls.fetch_add(1, Ordering::SeqCst) + 1;
        if expected.is_some() && poll >= self.ready_after {
            Some(self.identity.clone())
        } else {
            None
        }
    }

    fn process_alive(&self, _pid: u32) -> bool {
        self.alive
    }
}

#[derive(Default)]
struct FakeDelay {
    calls: AtomicUsize,
}

impl Delay for FakeDelay {
    fn sleep(&self, _duration: Duration) {
        self.calls.fetch_add(1, Ordering::SeqCst);
    }
}

fn identity() -> BoundDaemonIdentity {
    let started_at = Utc::now();
    let record = DaemonRecord {
        pid: 42,
        port: 2123,
        started_at,
    };
    BoundDaemonIdentity {
        status: StatusPayload {
            schema_version: "2026-07-16".to_owned(),
            daemon: DaemonStatusPayload {
                pid: 42,
                started_at,
                http_host: "localhost".to_owned(),
                http_port: 2123,
                user_home_dir: PathBuf::from("/tmp/compozy"),
                version: Some("0.3.0".to_owned()),
            },
        },
        record,
        origin: url::Url::parse("http://localhost:2123").expect("fixture URL parses"),
    }
}

#[test]
fn should_spawn_detached_and_poll_until_bound_ready() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let spawner = FakeSpawner {
        calls: AtomicUsize::new(0),
        pid: 42,
    };
    let readiness = FakeReadiness {
        polls: AtomicUsize::new(0),
        ready_after: 3,
        identity: identity(),
        alive: true,
    };
    let delay = FakeDelay::default();
    let supervisor = Supervisor {
        spawner: &spawner,
        readiness: &readiness,
        delay: &delay,
        readiness_deadline: Duration::from_secs(1),
        poll_interval: Duration::from_millis(10),
    };
    let result = supervisor
        .start_owned(Path::new("/tmp/compozy"), &home)
        .expect("runtime starts");
    assert!(matches!(
        result,
        StartResult::Owned(OwnedRuntime { pid: 42, .. })
    ));
    assert_eq!(spawner.calls.load(Ordering::SeqCst), 1);
}

#[test]
fn should_stop_after_three_failed_spawn_attempts() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let spawner = FakeSpawner {
        calls: AtomicUsize::new(0),
        pid: 42,
    };
    let readiness = FakeReadiness {
        polls: AtomicUsize::new(0),
        ready_after: usize::MAX,
        identity: identity(),
        alive: false,
    };
    let delay = FakeDelay::default();
    let supervisor = Supervisor {
        spawner: &spawner,
        readiness: &readiness,
        delay: &delay,
        readiness_deadline: Duration::from_millis(1),
        poll_interval: Duration::from_millis(1),
    };
    fs::create_dir_all(&home.logs_dir).expect("logs directory creates");
    fs::write(
        &home.runtime_log,
        "panic: failed to bind claim_token=compozy_claim_secret",
    )
    .expect("runtime log writes");
    let error = supervisor
        .start_owned(Path::new("/tmp/compozy"), &home)
        .expect_err("runtime should fail");
    assert_eq!(error.code, ShellErrorCode::RuntimeStartFailed);
    assert!(error.safe_message.contains("panic: failed to bind"));
    assert!(!error.safe_message.contains("compozy_claim_secret"));
    assert_eq!(spawner.calls.load(Ordering::SeqCst), 3);
}

#[test]
fn should_reclaim_a_start_lock_owned_by_a_dead_process() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    fs::create_dir_all(&home.root).expect("home directory creates");
    fs::write(home.root.join("app-start.lock"), "999999").expect("stale start lock writes");
    let spawner = FakeSpawner {
        calls: AtomicUsize::new(0),
        pid: 42,
    };
    let readiness = FakeReadiness {
        polls: AtomicUsize::new(0),
        ready_after: 4,
        identity: identity(),
        alive: false,
    };
    let delay = FakeDelay::default();
    let supervisor = Supervisor {
        spawner: &spawner,
        readiness: &readiness,
        delay: &delay,
        readiness_deadline: Duration::from_secs(1),
        poll_interval: Duration::from_millis(10),
    };

    assert!(matches!(
        supervisor.start_owned(Path::new("/tmp/compozy"), &home),
        Ok(StartResult::Owned(_))
    ));
    assert_eq!(spawner.calls.load(Ordering::SeqCst), 1);
    assert!(!home.root.join("app-start.lock").exists());
}

struct RaceSpawner {
    calls: AtomicUsize,
    running: AtomicBool,
}

impl ProcessSpawner for RaceSpawner {
    fn spawn_detached(&self, _binary: &Path, _home: &CompozyHome) -> io::Result<u32> {
        self.calls.fetch_add(1, Ordering::SeqCst);
        self.running.store(true, Ordering::SeqCst);
        Ok(42)
    }
}

struct RaceReadiness<'a> {
    running: &'a AtomicBool,
    identity: BoundDaemonIdentity,
}

impl ReadinessProbe for RaceReadiness<'_> {
    fn ready(&self, _expected_child_pid: Option<u32>) -> Option<BoundDaemonIdentity> {
        self.running
            .load(Ordering::SeqCst)
            .then(|| self.identity.clone())
    }

    fn process_alive(&self, _pid: u32) -> bool {
        self.running.load(Ordering::SeqCst)
    }
}

#[test]
fn should_serialize_concurrent_start_and_attach_to_the_winner() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let spawner = RaceSpawner {
        calls: AtomicUsize::new(0),
        running: AtomicBool::new(false),
    };
    let readiness = RaceReadiness {
        running: &spawner.running,
        identity: identity(),
    };
    let delay = ThreadDelay;
    let supervisor = Supervisor {
        spawner: &spawner,
        readiness: &readiness,
        delay: &delay,
        readiness_deadline: Duration::from_secs(1),
        poll_interval: Duration::from_millis(5),
    };
    let results = std::thread::scope(|scope| {
        let first = scope.spawn(|| supervisor.start_owned(Path::new("/tmp/compozy"), &home));
        let second = scope.spawn(|| supervisor.start_owned(Path::new("/tmp/compozy"), &home));
        [
            first.join().expect("first caller joins"),
            second.join().expect("second caller joins"),
        ]
    });

    assert!(results.iter().all(Result::is_ok), "results: {results:?}");
    assert_eq!(spawner.calls.load(Ordering::SeqCst), 1);
    assert_eq!(
        results
            .iter()
            .filter(|result| matches!(result, Ok(StartResult::Owned(_))))
            .count(),
        1
    );
    assert_eq!(
        results
            .iter()
            .filter(|result| matches!(result, Ok(StartResult::Attached(_))))
            .count(),
        1
    );
}

struct FakeVerifier(bool);

impl OwnershipVerifier for FakeVerifier {
    fn owns(&self, _pid: u32, _executable: &Path) -> bool {
        self.0
    }
}

#[derive(Default)]
struct FakeSignaler(Mutex<Vec<u32>>);

impl ProcessSignaler for FakeSignaler {
    fn terminate(&self, pid: u32) -> io::Result<()> {
        self.0
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .push(pid);
        Ok(())
    }
}

#[test]
fn should_never_signal_on_quit_and_guard_update_stop_by_durable_ownership() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let spawner = FakeSpawner {
        calls: AtomicUsize::new(0),
        pid: 42,
    };
    let readiness = FakeReadiness {
        polls: AtomicUsize::new(0),
        ready_after: usize::MAX,
        identity: identity(),
        alive: true,
    };
    let delay = FakeDelay::default();
    let supervisor = Supervisor::system(&spawner, &readiness, &delay);
    fs::write(&home.daemon_info, b"durable daemon record").expect("daemon record writes");
    supervisor.on_quit(Some(42));
    assert_eq!(
        fs::read(&home.daemon_info).expect("daemon record remains"),
        b"durable daemon record"
    );

    let signaler = FakeSignaler::default();
    let error = stop_for_update(
        42,
        Path::new("/tmp/compozy"),
        &FakeVerifier(false),
        &signaler,
        &home,
    )
    .expect_err("unowned runtime is refused");
    assert_eq!(error.code, ShellErrorCode::NotOwned);
    assert!(signaler.0.lock().expect("signals lock").is_empty());

    stop_for_update(
        42,
        Path::new("/tmp/compozy"),
        &FakeVerifier(true),
        &signaler,
        &home,
    )
    .expect("owned runtime can stop");
    assert_eq!(*signaler.0.lock().expect("signals lock"), vec![42]);
}
