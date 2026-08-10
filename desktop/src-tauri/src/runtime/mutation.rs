use std::collections::BTreeMap;
use std::fs::{self, File, OpenOptions, TryLockError};
use std::io::{Read, Seek, SeekFrom, Write};
use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use semver::Version;
use serde::{Deserialize, Serialize};
use thiserror::Error;

use crate::durability::sync_directory;
use crate::home::CompozyHome;

use super::discovery::ProcessTable;
use super::provenance;
use super::provision::{DOWNLOAD_NAME, STAGED_NAME};

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct LockOwner {
    pub pid: u32,
    pub started_at: DateTime<Utc>,
}

pub struct UpdateLock {
    file: File,
    path: PathBuf,
    locked: bool,
    pub recovered_stale_owner: Option<LockOwner>,
}

#[derive(Debug, Error)]
pub enum MutationError {
    #[error("another runtime mutation is in progress")]
    Locked,
    #[error("runtime mutation lock could not be read or written")]
    LockIo(#[source] std::io::Error),
    #[error("runtime ownership could not be verified")]
    NotOwned,
    #[error("runtime update journal is invalid")]
    InvalidJournal,
    #[error("runtime update journal could not be persisted")]
    JournalIo(#[source] std::io::Error),
}

impl UpdateLock {
    pub fn acquire(path: &Path, processes: &dyn ProcessTable) -> Result<Self, MutationError> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(MutationError::LockIo)?;
        }
        let mut file = OpenOptions::new()
            .create(true)
            .read(true)
            .truncate(false)
            .write(true)
            .open(path)
            .map_err(MutationError::LockIo)?;
        match file.try_lock() {
            Ok(()) => {}
            Err(TryLockError::WouldBlock) => return Err(MutationError::Locked),
            Err(TryLockError::Error(error)) => return Err(MutationError::LockIo(error)),
        }
        let previous = read_owner(&mut file).map_err(MutationError::LockIo)?;
        let recovered_stale_owner = previous.filter(|owner| !owner_matches(owner, processes));
        let pid = std::process::id();
        let started_at = processes.start_time(pid).unwrap_or_else(Utc::now);
        write_owner(&mut file, &LockOwner { pid, started_at }).map_err(MutationError::LockIo)?;
        Ok(Self {
            file,
            path: path.to_path_buf(),
            locked: true,
            recovered_stale_owner,
        })
    }

    pub fn release(mut self) -> Result<(), MutationError> {
        self.release_inner().map_err(MutationError::LockIo)
    }

    pub fn path(&self) -> &Path {
        &self.path
    }

    fn release_inner(&mut self) -> std::io::Result<()> {
        if !self.locked {
            return Ok(());
        }
        let clear_result = clear_owner(&mut self.file);
        let unlock_result = self.file.unlock();
        self.locked = false;
        clear_result.and(unlock_result)
    }
}

impl Drop for UpdateLock {
    fn drop(&mut self) {
        if let Err(error) = self.release_inner() {
            crate::logging::error(format!(
                "release runtime mutation {}: {error}",
                self.path.display()
            ));
        }
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum JournalPhase {
    Staged,
    Stopped,
    Swapped,
    Migrating,
    Started,
    Done,
}

impl JournalPhase {
    fn next(self) -> Option<Self> {
        match self {
            Self::Staged => Some(Self::Stopped),
            Self::Stopped => Some(Self::Swapped),
            Self::Swapped => Some(Self::Migrating),
            Self::Migrating => Some(Self::Started),
            Self::Started => Some(Self::Done),
            Self::Done => None,
        }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct UpdateJournal {
    pub phase: JournalPhase,
    pub from_version: Version,
    pub to_version: Version,
    pub staged_path: PathBuf,
    pub schema_heads: BTreeMap<String, u64>,
    pub written_at: DateTime<Utc>,
}

impl UpdateJournal {
    pub fn staged(
        from_version: Version,
        to_version: Version,
        staged_path: PathBuf,
        schema_heads: BTreeMap<String, u64>,
    ) -> Self {
        Self {
            phase: JournalPhase::Staged,
            from_version,
            to_version,
            staged_path,
            schema_heads,
            written_at: Utc::now(),
        }
    }

    pub fn advance(&mut self, phase: JournalPhase) -> Result<(), MutationError> {
        if self.phase.next() != Some(phase) {
            return Err(MutationError::InvalidJournal);
        }
        self.phase = phase;
        self.written_at = Utc::now();
        Ok(())
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RecoveryAction {
    CleanRollback,
    ResumeNewRuntime,
    RecoveryRequired,
    Complete,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SwapCommit {
    pub backup_path: PathBuf,
    provenance_backup_path: PathBuf,
}

#[derive(Debug)]
pub struct SwapFailure {
    pub committed: Option<SwapCommit>,
    pub error: MutationError,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RecoveryPlan {
    CleanRollback,
    ResumeNewRuntime(UpdateJournal),
    RecoveryRequired(UpdateJournal),
    Complete,
}

pub fn classify_recovery(journal: &UpdateJournal) -> RecoveryAction {
    match journal.phase {
        JournalPhase::Staged | JournalPhase::Stopped => RecoveryAction::CleanRollback,
        JournalPhase::Swapped => RecoveryAction::ResumeNewRuntime,
        JournalPhase::Migrating | JournalPhase::Started => RecoveryAction::RecoveryRequired,
        JournalPhase::Done => RecoveryAction::Complete,
    }
}

pub fn verify_ownership(home: &CompozyHome, executable: &Path) -> Result<(), MutationError> {
    provenance::owns_executable(home, executable)
        .then_some(())
        .ok_or(MutationError::NotOwned)
}

pub fn prepare_recovery(home: &CompozyHome) -> Result<Option<RecoveryPlan>, MutationError> {
    let Some(mut journal) = read_journal(&home.update_journal)? else {
        return Ok(None);
    };
    let action = classify_recovery(&journal);
    match action {
        RecoveryAction::CleanRollback => {
            let swap = recovery_swap(home);
            if swap.backup_path.exists() {
                rollback_swap(home, &swap)?;
            }
            remove_recovery_stage(home)?;
            clear_journal(&home.update_journal)?;
            Ok(Some(RecoveryPlan::CleanRollback))
        }
        RecoveryAction::ResumeNewRuntime => {
            restore_new_provenance(home, &journal)?;
            journal.advance(JournalPhase::Migrating)?;
            write_journal(&home.update_journal, &journal)?;
            Ok(Some(RecoveryPlan::ResumeNewRuntime(journal)))
        }
        RecoveryAction::RecoveryRequired => Ok(Some(RecoveryPlan::RecoveryRequired(journal))),
        RecoveryAction::Complete => {
            finalize_swap(&recovery_swap(home))?;
            clear_journal(&home.update_journal)?;
            Ok(Some(RecoveryPlan::Complete))
        }
    }
}

pub fn complete_recovery(
    home: &CompozyHome,
    mut journal: UpdateJournal,
) -> Result<(), MutationError> {
    journal.advance(JournalPhase::Started)?;
    write_journal(&home.update_journal, &journal)?;
    finalize_swap(&recovery_swap(home))?;
    journal.advance(JournalPhase::Done)?;
    write_journal(&home.update_journal, &journal)?;
    clear_journal(&home.update_journal)
}

pub fn swap_binary(home: &CompozyHome, staged: &Path) -> Result<SwapCommit, SwapFailure> {
    let parent = home.app_binary.parent().ok_or_else(|| SwapFailure {
        committed: None,
        error: MutationError::InvalidJournal,
    })?;
    let paths = recovery_swap(home);
    let backup_path = paths.backup_path;
    let provenance_backup_path = paths.provenance_backup_path;
    match fs::remove_file(&backup_path) {
        Ok(()) => {}
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
        Err(error) => return Err(swap_not_committed(error)),
    }
    match fs::remove_file(&provenance_backup_path) {
        Ok(()) => {}
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
        Err(error) => return Err(swap_not_committed(error)),
    }
    fs::copy(&home.provenance, &provenance_backup_path).map_err(swap_not_committed)?;
    File::open(&provenance_backup_path)
        .and_then(|file| file.sync_all())
        .map_err(swap_not_committed)?;
    fs::rename(&home.app_binary, &backup_path).map_err(swap_not_committed)?;
    if let Err(error) = fs::rename(staged, &home.app_binary) {
        if let Err(rollback_error) = fs::rename(&backup_path, &home.app_binary) {
            return Err(swap_not_committed(std::io::Error::other(format!(
                "swap failed: {error}; rollback failed: {rollback_error}"
            ))));
        }
        if let Err(cleanup_error) = fs::remove_file(&provenance_backup_path) {
            return Err(swap_not_committed(std::io::Error::other(format!(
                "swap failed: {error}; provenance cleanup failed: {cleanup_error}"
            ))));
        }
        return Err(swap_not_committed(error));
    }
    let commit = SwapCommit {
        backup_path,
        provenance_backup_path,
    };
    if let Err(error) = sync_directory(parent) {
        return Err(SwapFailure {
            committed: Some(commit),
            error: MutationError::JournalIo(error),
        });
    }
    Ok(commit)
}

fn swap_not_committed(error: std::io::Error) -> SwapFailure {
    SwapFailure {
        committed: None,
        error: MutationError::JournalIo(error),
    }
}

pub fn rollback_swap(home: &CompozyHome, swap: &SwapCommit) -> Result<(), MutationError> {
    let marker: provenance::ProvenanceRecord = serde_json::from_slice(
        &fs::read(&swap.provenance_backup_path).map_err(MutationError::JournalIo)?,
    )
    .map_err(|error| invalid_journal("decode previous runtime provenance", error))?;
    provenance::write_atomic(&home.provenance, &marker).map_err(MutationError::JournalIo)?;
    if home.app_binary.exists() {
        fs::remove_file(&home.app_binary).map_err(MutationError::JournalIo)?;
    }
    fs::rename(&swap.backup_path, &home.app_binary).map_err(MutationError::JournalIo)?;
    if let Some(parent) = home.app_binary.parent() {
        sync_directory(parent).map_err(MutationError::JournalIo)?;
    }
    fs::remove_file(&swap.provenance_backup_path).map_err(MutationError::JournalIo)?;
    if let Some(parent) = swap.provenance_backup_path.parent() {
        sync_directory(parent).map_err(MutationError::JournalIo)?;
    }
    Ok(())
}

pub fn finalize_swap(swap: &SwapCommit) -> Result<(), MutationError> {
    for path in [&swap.backup_path, &swap.provenance_backup_path] {
        match fs::remove_file(path) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => return Err(MutationError::JournalIo(error)),
        }
    }
    if let Some(parent) = swap.backup_path.parent() {
        sync_directory(parent).map_err(MutationError::JournalIo)?;
    }
    Ok(())
}

pub fn write_journal(path: &Path, journal: &UpdateJournal) -> Result<(), MutationError> {
    let parent = path.parent().ok_or(MutationError::InvalidJournal)?;
    fs::create_dir_all(parent).map_err(MutationError::JournalIo)?;
    let temporary = parent.join(".update-journal.json.tmp");
    let mut file = File::create(&temporary).map_err(MutationError::JournalIo)?;
    serde_json::to_writer(&mut file, journal)
        .map_err(|error| MutationError::JournalIo(std::io::Error::other(error)))?;
    file.write_all(b"\n").map_err(MutationError::JournalIo)?;
    file.sync_all().map_err(MutationError::JournalIo)?;
    fs::rename(&temporary, path).map_err(MutationError::JournalIo)?;
    sync_directory(parent).map_err(MutationError::JournalIo)
}

pub fn read_journal(path: &Path) -> Result<Option<UpdateJournal>, MutationError> {
    let bytes = match fs::read(path) {
        Ok(bytes) => bytes,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(MutationError::JournalIo(error)),
    };
    let journal: UpdateJournal = serde_json::from_slice(&bytes)
        .map_err(|error| invalid_journal("decode runtime update journal", error))?;
    let expected_stage = path
        .parent()
        .ok_or(MutationError::InvalidJournal)?
        .join(STAGED_NAME);
    if journal.staged_path != expected_stage
        || journal.schema_heads.is_empty()
        || journal.to_version <= journal.from_version
    {
        return Err(MutationError::InvalidJournal);
    }
    Ok(Some(journal))
}

pub fn clear_journal(path: &Path) -> Result<(), MutationError> {
    match fs::remove_file(path) {
        Ok(()) => {}
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(()),
        Err(error) => return Err(MutationError::JournalIo(error)),
    }
    if let Some(parent) = path.parent() {
        sync_directory(parent).map_err(MutationError::JournalIo)?;
    }
    Ok(())
}

fn owner_matches(owner: &LockOwner, processes: &dyn ProcessTable) -> bool {
    processes.start_time(owner.pid).is_some_and(|started_at| {
        (started_at.timestamp() - owner.started_at.timestamp()).abs() <= 1
    })
}

fn read_owner(file: &mut File) -> std::io::Result<Option<LockOwner>> {
    file.seek(SeekFrom::Start(0))?;
    let mut bytes = Vec::new();
    file.read_to_end(&mut bytes)?;
    if bytes.iter().all(u8::is_ascii_whitespace) {
        return Ok(None);
    }
    Ok(serde_json::from_slice(&bytes).ok())
}

fn write_owner(file: &mut File, owner: &LockOwner) -> std::io::Result<()> {
    file.seek(SeekFrom::Start(0))?;
    file.set_len(0)?;
    serde_json::to_writer(&mut *file, owner).map_err(std::io::Error::other)?;
    file.write_all(b"\n")?;
    file.sync_all()
}

fn clear_owner(file: &mut File) -> std::io::Result<()> {
    file.seek(SeekFrom::Start(0))?;
    file.set_len(0)?;
    file.sync_all()
}

fn recovery_swap(home: &CompozyHome) -> SwapCommit {
    let parent = home.app_binary.parent().unwrap_or(home.root.as_path());
    SwapCommit {
        backup_path: parent.join(".runtime-previous"),
        provenance_backup_path: parent.join(".runtime-provenance-previous"),
    }
}

fn restore_new_provenance(
    home: &CompozyHome,
    journal: &UpdateJournal,
) -> Result<(), MutationError> {
    let swap = recovery_swap(home);
    let previous: provenance::ProvenanceRecord = serde_json::from_slice(
        &fs::read(&swap.provenance_backup_path).map_err(MutationError::JournalIo)?,
    )
    .map_err(|error| invalid_journal("decode recovery provenance", error))?;
    if previous.installed_by != "desktop-app" {
        return Err(MutationError::NotOwned);
    }
    let digest = provenance::sha256_file(&home.app_binary).map_err(MutationError::JournalIo)?;
    let marker = provenance::ProvenanceRecord::desktop(
        previous.app_version,
        journal.to_version.to_string(),
        previous.channel,
        Utc::now(),
        digest,
    );
    provenance::write_atomic(&home.provenance, &marker).map_err(MutationError::JournalIo)
}

fn remove_recovery_stage(home: &CompozyHome) -> Result<(), MutationError> {
    let parent = home.app_binary.parent().unwrap_or(home.root.as_path());
    for path in [parent.join(STAGED_NAME), parent.join(DOWNLOAD_NAME)] {
        match fs::remove_file(path) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => return Err(MutationError::JournalIo(error)),
        }
    }
    Ok(())
}

fn invalid_journal(context: &str, error: serde_json::Error) -> MutationError {
    crate::logging::error(format!("{context}: {error}"));
    MutationError::InvalidJournal
}

#[cfg(test)]
mod tests {
    use std::collections::{BTreeMap, HashMap};

    use super::*;

    #[derive(Default)]
    struct FakeProcesses {
        starts: HashMap<u32, DateTime<Utc>>,
    }

    impl ProcessTable for FakeProcesses {
        fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
            self.starts.get(&pid).copied()
        }

        fn executable(&self, _pid: u32) -> Option<PathBuf> {
            None
        }

        fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
            descendant == ancestor
        }
    }

    fn journal(home: &CompozyHome, phase: JournalPhase) -> UpdateJournal {
        UpdateJournal {
            phase,
            from_version: Version::parse("0.3.0").expect("version parses"),
            to_version: Version::parse("0.4.0").expect("version parses"),
            staged_path: home
                .app_binary
                .parent()
                .expect("binary has parent")
                .join(STAGED_NAME),
            schema_heads: BTreeMap::from([("global".to_owned(), 12)]),
            written_at: Utc::now(),
        }
    }

    #[test]
    fn should_refuse_a_second_runtime_mutator_while_lock_is_held() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("update.lock");
        let processes = FakeProcesses::default();
        let first = UpdateLock::acquire(&path, &processes).expect("first lock acquires");
        assert!(matches!(
            UpdateLock::acquire(&path, &processes),
            Err(MutationError::Locked)
        ));
        first.release().expect("first lock releases");
        UpdateLock::acquire(&path, &processes)
            .expect("lock reacquires")
            .release()
            .expect("reacquired lock releases");
    }

    #[test]
    fn should_report_and_replace_a_stale_lock_identity() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("update.lock");
        let stale = LockOwner {
            pid: 42,
            started_at: Utc::now() - chrono::Duration::hours(1),
        };
        fs::write(&path, serde_json::to_vec(&stale).expect("owner serializes"))
            .expect("stale owner writes");

        let lock =
            UpdateLock::acquire(&path, &FakeProcesses::default()).expect("stale lock is reclaimed");
        assert_eq!(lock.recovered_stale_owner, Some(stale));
        lock.release().expect("lock releases");
    }

    #[test]
    fn should_enforce_the_durable_journal_phase_order() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let mut journal = journal(&home, JournalPhase::Staged);
        for phase in [
            JournalPhase::Stopped,
            JournalPhase::Swapped,
            JournalPhase::Migrating,
            JournalPhase::Started,
            JournalPhase::Done,
        ] {
            journal.advance(phase).expect("next phase advances");
            write_journal(&home.update_journal, &journal).expect("journal persists");
            assert_eq!(
                read_journal(&home.update_journal)
                    .expect("journal reads")
                    .expect("journal exists")
                    .phase,
                phase
            );
        }
        assert!(matches!(
            journal.advance(JournalPhase::Stopped),
            Err(MutationError::InvalidJournal)
        ));
    }

    #[test]
    fn should_classify_every_crash_boundary_without_restarting_old_after_migration() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let cases = [
            (JournalPhase::Staged, RecoveryAction::CleanRollback),
            (JournalPhase::Stopped, RecoveryAction::CleanRollback),
            (JournalPhase::Swapped, RecoveryAction::ResumeNewRuntime),
            (JournalPhase::Migrating, RecoveryAction::RecoveryRequired),
            (JournalPhase::Started, RecoveryAction::RecoveryRequired),
            (JournalPhase::Done, RecoveryAction::Complete),
        ];
        for (phase, expected) in cases {
            assert_eq!(classify_recovery(&journal(&home, phase)), expected);
        }
    }

    #[test]
    fn should_rollback_an_ambiguous_pre_swap_crash_and_keep_recovery_required_sticky() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        let bin = home.app_binary.parent().expect("binary has parent");
        fs::create_dir_all(bin).expect("bin directory creates");
        fs::write(&home.app_binary, b"new").expect("new binary writes");
        fs::write(bin.join(".runtime-previous"), b"old").expect("old binary writes");
        let old_marker = provenance::ProvenanceRecord::desktop(
            "0.3.0",
            "0.3.0",
            "beta",
            Utc::now(),
            provenance::sha256_file(&bin.join(".runtime-previous")).expect("old binary hashes"),
        );
        let new_marker = provenance::ProvenanceRecord::desktop(
            "0.3.0",
            "0.4.0",
            "beta",
            Utc::now(),
            provenance::sha256_file(&home.app_binary).expect("new binary hashes"),
        );
        provenance::write_atomic(&home.provenance, &new_marker).expect("new marker writes");
        fs::write(
            bin.join(".runtime-provenance-previous"),
            serde_json::to_vec(&old_marker).expect("old marker serializes"),
        )
        .expect("old marker backup writes");
        write_journal(&home.update_journal, &journal(&home, JournalPhase::Stopped))
            .expect("stopped journal writes");

        let plan = prepare_recovery(&home)
            .expect("recovery prepares")
            .expect("recovery plan exists");
        assert_eq!(plan, RecoveryPlan::CleanRollback);
        assert_eq!(fs::read(&home.app_binary).expect("binary reads"), b"old");
        let restored: provenance::ProvenanceRecord =
            serde_json::from_slice(&fs::read(&home.provenance).expect("restored marker reads"))
                .expect("restored marker decodes");
        assert_eq!(restored, old_marker);
        assert!(
            read_journal(&home.update_journal)
                .expect("journal checks")
                .is_none()
        );

        fs::write(&home.app_binary, b"new").expect("new binary rewrites");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;

            fs::set_permissions(&home.app_binary, fs::Permissions::from_mode(0o755))
                .expect("new binary becomes executable");
        }
        fs::write(bin.join(".runtime-previous"), b"old").expect("old binary rewrites");
        provenance::write_atomic(&home.provenance, &old_marker).expect("old marker rewrites");
        fs::write(
            bin.join(".runtime-provenance-previous"),
            serde_json::to_vec(&old_marker).expect("old marker reserializes"),
        )
        .expect("old marker backup rewrites");
        write_journal(&home.update_journal, &journal(&home, JournalPhase::Swapped))
            .expect("swapped journal writes");

        let plan = prepare_recovery(&home)
            .expect("swapped recovery prepares")
            .expect("swapped recovery plan exists");
        let RecoveryPlan::ResumeNewRuntime(recovered_journal) = plan else {
            panic!("swapped journal should resume the new runtime");
        };
        assert_eq!(recovered_journal.phase, JournalPhase::Migrating);
        assert!(provenance::owns_executable(&home, &home.app_binary));
        let recovered: provenance::ProvenanceRecord =
            serde_json::from_slice(&fs::read(&home.provenance).expect("recovered marker reads"))
                .expect("recovered marker decodes");
        assert_eq!(recovered.runtime_version, "0.4.0");

        write_journal(
            &home.update_journal,
            &journal(&home, JournalPhase::Migrating),
        )
        .expect("migrating journal writes");
        for _attempt in 0..2 {
            let plan = prepare_recovery(&home)
                .expect("sticky recovery reads")
                .expect("sticky plan exists");
            let RecoveryPlan::RecoveryRequired(journal) = plan else {
                panic!("migrating journal should require explicit recovery");
            };
            assert_eq!(journal.phase, JournalPhase::Migrating);
        }
    }

    #[test]
    fn should_refuse_ownership_when_marker_hash_disagrees_with_binary() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let home = CompozyHome::from_root(directory.path().to_path_buf());
        fs::create_dir_all(home.app_binary.parent().expect("binary has parent"))
            .expect("bin directory creates");
        fs::write(&home.app_binary, b"runtime").expect("binary writes");
        let marker = provenance::ProvenanceRecord::desktop(
            "0.3.0",
            "0.3.0",
            "beta",
            Utc::now(),
            "0".repeat(64),
        );
        provenance::write_atomic(&home.provenance, &marker).expect("marker writes");
        assert!(matches!(
            verify_ownership(&home, &home.app_binary),
            Err(MutationError::NotOwned)
        ));
    }
}
