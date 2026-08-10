use std::fs::{self, File, OpenOptions};
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::Duration;

use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;
#[cfg(not(windows))]
use sysinfo::Signal;
use sysinfo::{Pid, ProcessesToUpdate, System};

use super::probe::BoundDaemonIdentity;

const READY_DEADLINE: Duration = Duration::from_secs(30);
const POLL_INTERVAL: Duration = Duration::from_millis(100);
const ATTEMPT_BACKOFF: [Option<Duration>; 3] = [
    Some(Duration::from_millis(250)),
    Some(Duration::from_secs(1)),
    None,
];

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OwnedRuntime {
    pub pid: u32,
    pub identity: BoundDaemonIdentity,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum StartResult {
    Owned(OwnedRuntime),
    Attached(BoundDaemonIdentity),
}

pub trait ProcessSpawner: Send + Sync {
    fn spawn_detached(&self, binary: &Path, home: &CompozyHome) -> io::Result<u32>;
    fn terminate(&self, pid: u32) -> io::Result<()>;
}

pub trait ReadinessProbe: Send + Sync {
    fn ready(&self, expected_child_pid: Option<u32>) -> Option<BoundDaemonIdentity>;
    fn process_alive(&self, pid: u32) -> bool;
}

pub trait Delay: Send + Sync {
    fn sleep(&self, duration: Duration);
}

#[derive(Debug, Default)]
pub struct ThreadDelay;

impl Delay for ThreadDelay {
    fn sleep(&self, duration: Duration) {
        std::thread::sleep(duration);
    }
}

#[derive(Debug, Default)]
pub struct SystemSpawner;

impl ProcessSpawner for SystemSpawner {
    fn spawn_detached(&self, binary: &Path, home: &CompozyHome) -> io::Result<u32> {
        fs::create_dir_all(&home.logs_dir)?;
        let stdout = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&home.runtime_log)?;
        let stderr = stdout.try_clone()?;
        let mut command = daemon_command(binary, home);
        command
            .stdin(Stdio::null())
            .stdout(Stdio::from(stdout))
            .stderr(Stdio::from(stderr));
        configure_detached(&mut command);
        command.spawn().map(|child| child.id())
    }

    fn terminate(&self, pid: u32) -> io::Result<()> {
        let mut system = System::new();
        let pid = Pid::from_u32(pid);
        system.refresh_processes(ProcessesToUpdate::Some(&[pid]), true);
        let process = system
            .process(pid)
            .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "runtime process is absent"))?;
        #[cfg(windows)]
        let signaled = process.kill();
        #[cfg(not(windows))]
        let signaled = process.kill_with(Signal::Term) == Some(true);
        signaled
            .then_some(())
            .ok_or_else(|| io::Error::other("runtime process rejected termination"))
    }
}

fn daemon_command(binary: &Path, home: &CompozyHome) -> Command {
    let mut command = Command::new(binary);
    command
        .args(["daemon", "start", "--foreground", "--internal-child"])
        .env("COMPOZY_HOME", &home.root);
    command
}

pub struct Supervisor<'a> {
    pub spawner: &'a dyn ProcessSpawner,
    pub readiness: &'a dyn ReadinessProbe,
    pub delay: &'a dyn Delay,
    pub readiness_deadline: Duration,
    pub poll_interval: Duration,
}

impl<'a> Supervisor<'a> {
    pub fn system(
        spawner: &'a dyn ProcessSpawner,
        readiness: &'a dyn ReadinessProbe,
        delay: &'a dyn Delay,
    ) -> Self {
        Self {
            spawner,
            readiness,
            delay,
            readiness_deadline: READY_DEADLINE,
            poll_interval: POLL_INTERVAL,
        }
    }

    pub fn start_owned(
        &self,
        binary: &Path,
        home: &CompozyHome,
    ) -> Result<StartResult, ShellError> {
        let lock_path = home.root.join("app-start.lock");
        let mut attempt = 0;
        while attempt < ATTEMPT_BACKOFF.len() {
            if let Some(identity) = self.readiness.ready(None) {
                return Ok(StartResult::Attached(identity));
            }
            let lock = match StartLock::acquire(&lock_path) {
                Ok(lock) => lock,
                Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {
                    match StartLock::reclaim_if_stale(&lock_path, self.readiness) {
                        Ok(true) => continue,
                        Ok(false) => {}
                        Err(error) => {
                            crate::logging::error(format!(
                                "reclaim runtime start lock {}: {error}",
                                lock_path.display()
                            ));
                            return Err(start_error(home));
                        }
                    }
                    if let Some(identity) = self.wait_for_competing_start(&lock_path) {
                        return Ok(StartResult::Attached(identity));
                    }
                    if lock_path.exists() {
                        return Err(start_error(home));
                    }
                    continue;
                }
                Err(error) => {
                    crate::logging::error(format!(
                        "acquire runtime start lock {}: {error}",
                        lock_path.display()
                    ));
                    return Err(start_error(home));
                }
            };
            if let Some(identity) = self.readiness.ready(None) {
                lock.release();
                return Ok(StartResult::Attached(identity));
            }
            attempt += 1;
            let pid = match self.spawner.spawn_detached(binary, home) {
                Ok(pid) if pid > 0 => pid,
                Ok(_) => {
                    crate::logging::error("spawned runtime returned an invalid process id");
                    lock.release();
                    if let Some(backoff) = ATTEMPT_BACKOFF[attempt - 1] {
                        self.delay.sleep(backoff);
                    }
                    continue;
                }
                Err(error) => {
                    crate::logging::error(format!(
                        "spawn owned runtime {}: {error}",
                        binary.display()
                    ));
                    lock.release();
                    if let Some(backoff) = ATTEMPT_BACKOFF[attempt - 1] {
                        self.delay.sleep(backoff);
                    }
                    continue;
                }
            };
            for _ in 0..poll_limit(self.readiness_deadline, self.poll_interval) {
                if let Some(identity) = self.readiness.ready(Some(pid)) {
                    lock.release();
                    return Ok(StartResult::Owned(OwnedRuntime { pid, identity }));
                }
                if !self.readiness.process_alive(pid) {
                    break;
                }
                self.delay.sleep(self.poll_interval);
            }
            if self.readiness.process_alive(pid) {
                if let Err(error) = self.spawner.terminate(pid) {
                    crate::logging::error(format!("terminate unready runtime {pid}: {error}"));
                    lock.release();
                    return Err(start_error(home));
                }
                for _ in 0..poll_limit(self.readiness_deadline, self.poll_interval) {
                    if !self.readiness.process_alive(pid) {
                        break;
                    }
                    self.delay.sleep(self.poll_interval);
                }
                if self.readiness.process_alive(pid) {
                    lock.release();
                    return Err(start_error(home));
                }
            }
            lock.release();
            if let Some(backoff) = ATTEMPT_BACKOFF[attempt - 1] {
                self.delay.sleep(backoff);
            }
        }
        Err(start_error(home))
    }

    fn wait_for_competing_start(&self, lock_path: &Path) -> Option<BoundDaemonIdentity> {
        for _ in 0..poll_limit(self.readiness_deadline, self.poll_interval) {
            if let Some(identity) = self.readiness.ready(None) {
                return Some(identity);
            }
            if !lock_path.exists() {
                return None;
            }
            self.delay.sleep(self.poll_interval);
        }
        None
    }

    pub fn on_quit(&self, _owned_pid: Option<u32>) {
        // BR-2: the daemon outlives the shell. Quit intentionally sends no signal.
    }
}

pub trait OwnershipVerifier {
    fn owns(&self, pid: u32, executable: &Path) -> bool;
}

pub trait ProcessSignaler {
    fn terminate(&self, pid: u32) -> io::Result<()>;
}

pub fn stop_for_update(
    pid: u32,
    executable: &Path,
    verifier: &dyn OwnershipVerifier,
    signaler: &dyn ProcessSignaler,
    home: &CompozyHome,
) -> Result<(), ShellError> {
    if !verifier.owns(pid, executable) {
        return Err(ShellError::from_code(
            ShellErrorCode::NotOwned,
            home.app_log.clone(),
        ));
    }
    signaler.terminate(pid).map_err(|_| {
        ShellError::new(
            ShellErrorCode::RuntimeStartFailed,
            "The owned runtime could not be stopped for update.",
            home.app_log.clone(),
        )
    })
}

struct StartLock {
    path: PathBuf,
    file: Option<File>,
}

impl StartLock {
    fn acquire(path: &Path) -> io::Result<Self> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let mut file = OpenOptions::new().create_new(true).write(true).open(path)?;
        if let Err(error) = write!(file, "{}", std::process::id()).and_then(|()| file.sync_all()) {
            drop(file);
            if let Err(cleanup_error) = fs::remove_file(path)
                && cleanup_error.kind() != io::ErrorKind::NotFound
            {
                crate::logging::error(format!(
                    "remove incomplete start lock {}: {cleanup_error}",
                    path.display()
                ));
            }
            return Err(error);
        }
        Ok(Self {
            path: path.to_path_buf(),
            file: Some(file),
        })
    }

    fn reclaim_if_stale(path: &Path, readiness: &dyn ReadinessProbe) -> io::Result<bool> {
        let owner = match fs::read_to_string(path) {
            Ok(owner) => owner.trim().parse::<u32>().ok(),
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(true),
            Err(error) => return Err(error),
        };
        if owner.is_some_and(|pid| pid == std::process::id() || readiness.process_alive(pid)) {
            return Ok(false);
        }
        match fs::remove_file(path) {
            Ok(()) => Ok(true),
            Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(true),
            Err(error) => Err(error),
        }
    }

    fn release(mut self) {
        self.file.take();
        if let Err(error) = fs::remove_file(&self.path)
            && error.kind() != io::ErrorKind::NotFound
        {
            crate::logging::error(format!(
                "remove start lock {}: {error}",
                self.path.display()
            ));
        }
    }
}

impl Drop for StartLock {
    fn drop(&mut self) {
        if self.file.take().is_none() {
            return;
        }
        if let Err(error) = fs::remove_file(&self.path)
            && error.kind() != io::ErrorKind::NotFound
        {
            crate::logging::error(format!(
                "remove start lock {}: {error}",
                self.path.display()
            ));
        }
    }
}

fn start_error(home: &CompozyHome) -> ShellError {
    ShellError::from_code(ShellErrorCode::RuntimeStartFailed, home.runtime_log.clone())
}

fn poll_limit(deadline: Duration, interval: Duration) -> usize {
    if interval.is_zero() {
        return 1;
    }
    let deadline_millis = deadline.as_millis();
    let interval_millis = interval.as_millis().max(1);
    deadline_millis.div_ceil(interval_millis).max(1) as usize
}

#[cfg(unix)]
fn configure_detached(command: &mut Command) {
    use std::os::unix::process::CommandExt;

    // SAFETY: pre_exec invokes only the async-signal-safe setsid syscall before exec.
    unsafe {
        command.pre_exec(|| {
            if libc::setsid() == -1 {
                return Err(io::Error::last_os_error());
            }
            Ok(())
        });
    }
}

#[cfg(windows)]
fn configure_detached(command: &mut Command) {
    use std::os::windows::process::CommandExt;

    const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;
    command.creation_flags(CREATE_NEW_PROCESS_GROUP);
}

#[cfg(test)]
#[path = "supervisor_tests.rs"]
mod tests;
