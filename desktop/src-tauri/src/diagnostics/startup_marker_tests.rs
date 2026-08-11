use std::sync::{Arc, Barrier};

use chrono::TimeZone;

use super::*;

fn owner(pid: u32, started_at: DateTime<Utc>) -> MarkerOwner {
    MarkerOwner { pid, started_at }
}

#[test]
fn should_preserve_a_previous_uncleared_boot_without_exposing_a_home_path() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let first_started_at = Utc
        .with_ymd_and_hms(2026, 8, 11, 12, 0, 0)
        .single()
        .expect("fixture timestamp is valid");
    let first = StartupMarker::begin_with_owner(
        &home,
        "boot-first".to_owned(),
        owner(100, first_started_at),
        |_| None,
    )
    .expect("first marker writes");
    first
        .update_phase("starting")
        .expect("marker phase updates");

    let second = StartupMarker::begin_with_owner(
        &home,
        "boot-second".to_owned(),
        owner(200, Utc::now()),
        |_| None,
    )
    .expect("second marker writes");
    assert_eq!(
        second.previous_crash(),
        Some(PreviousCrash {
            boot_id: "boot-first".to_owned(),
            boot_phase: "starting".to_owned(),
            started_at: first_started_at,
        })
    );
    second.clear().expect("marker clears after clean exit");
    assert!(!home.support_bundles_dir.join(MARKER_FILE_NAME).exists());
}

#[test]
fn should_not_overwrite_or_clear_a_marker_owned_by_a_live_primary_process() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let started_at = Utc::now();
    let primary_owner = owner(100, started_at);
    let primary = StartupMarker::begin_with_owner(
        &home,
        "boot-primary".to_owned(),
        primary_owner.clone(),
        |_| None,
    )
    .expect("primary marker writes");

    let error = StartupMarker::begin_with_owner(
        &home,
        "boot-secondary".to_owned(),
        owner(200, Utc::now()),
        |pid| (pid == primary_owner.pid).then_some(primary_owner.started_at),
    )
    .expect_err("active primary marker prevents secondary overwrite");

    assert_eq!(error.kind(), io::ErrorKind::AlreadyExists);
    primary.clear().expect("primary can clear its own marker");
    assert!(!home.support_bundles_dir.join(MARKER_FILE_NAME).exists());
}

#[test]
fn should_serialize_competing_marker_claims_without_selecting_a_fixed_winner() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let first_owner = owner(
        100,
        Utc.with_ymd_and_hms(2026, 8, 11, 12, 0, 0)
            .single()
            .expect("fixture timestamp is valid"),
    );
    let second_owner = owner(
        200,
        Utc.with_ymd_and_hms(2026, 8, 11, 12, 1, 0)
            .single()
            .expect("fixture timestamp is valid"),
    );
    let barrier = Arc::new(Barrier::new(2));
    let first_home = home.clone();
    let first_barrier = Arc::clone(&barrier);
    let first_owner_for_thread = first_owner.clone();
    let first_other_owner = second_owner.clone();
    let first = std::thread::spawn(move || {
        first_barrier.wait();
        StartupMarker::begin_with_owner(
            &first_home,
            "boot-first".to_owned(),
            first_owner_for_thread.clone(),
            move |pid| {
                (pid == first_owner_for_thread.pid)
                    .then_some(first_owner_for_thread.started_at)
                    .or_else(|| {
                        (pid == first_other_owner.pid).then_some(first_other_owner.started_at)
                    })
            },
        )
        .map_err(|error| error.kind())
    });
    let second_home = home.clone();
    let second_barrier = Arc::clone(&barrier);
    let second_owner_for_thread = second_owner.clone();
    let second_other_owner = first_owner.clone();
    let second = std::thread::spawn(move || {
        second_barrier.wait();
        StartupMarker::begin_with_owner(
            &second_home,
            "boot-second".to_owned(),
            second_owner_for_thread.clone(),
            move |pid| {
                (pid == second_owner_for_thread.pid)
                    .then_some(second_owner_for_thread.started_at)
                    .or_else(|| {
                        (pid == second_other_owner.pid).then_some(second_other_owner.started_at)
                    })
            },
        )
        .map_err(|error| error.kind())
    });

    let first = first.join().expect("first claimant joins");
    let second = second.join().expect("second claimant joins");
    let marker_path = home.support_bundles_dir.join(MARKER_FILE_NAME);
    let lock_path = home.support_bundles_dir.join(MARKER_LOCK_FILE_NAME);
    let (winner, loser_boot_id, loser_owner) = match (first, second) {
        (Ok(marker), Err(io::ErrorKind::AlreadyExists)) => {
            (marker, "boot-second".to_owned(), second_owner)
        }
        (Err(io::ErrorKind::AlreadyExists), Ok(marker)) => {
            (marker, "boot-first".to_owned(), first_owner)
        }
        (first, second) => {
            panic!("one claim must win and one must be active: {first:?}, {second:?}")
        }
    };
    let before = fs::read(&marker_path).expect("winning marker reads");
    let loser = StartupMarker {
        path: marker_path.clone(),
        lock_path,
        boot_id: loser_boot_id,
        owner: loser_owner,
        previous_crash: None,
    };

    loser
        .update_phase("error")
        .expect("loser update is a no-op");
    loser.clear().expect("loser clear is a no-op");

    assert_eq!(fs::read(&marker_path).expect("marker still reads"), before);
    winner.clear().expect("winner clears its own marker");
}

#[test]
fn should_treat_reused_pid_with_a_new_start_time_as_a_previous_crash() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let old_started_at = Utc
        .with_ymd_and_hms(2026, 8, 11, 12, 0, 0)
        .single()
        .expect("fixture timestamp is valid");
    StartupMarker::begin_with_owner(
        &home,
        "boot-old".to_owned(),
        owner(100, old_started_at),
        |_| None,
    )
    .expect("stale marker writes");

    let current_started_at = Utc
        .with_ymd_and_hms(2026, 8, 11, 13, 0, 0)
        .single()
        .expect("fixture timestamp is valid");
    let current = StartupMarker::begin_with_owner(
        &home,
        "boot-current".to_owned(),
        owner(200, current_started_at),
        |pid| (pid == 100).then_some(current_started_at),
    )
    .expect("reused PID marker replaces stale marker");

    assert_eq!(
        current
            .previous_crash()
            .expect("previous crash is recorded")
            .boot_id,
        "boot-old"
    );
    current.clear().expect("current marker clears");
}

#[cfg(unix)]
#[test]
fn should_create_owner_only_marker_storage() {
    use std::os::unix::fs::PermissionsExt;

    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let marker =
        StartupMarker::begin(&home, "boot-private".to_owned(), Utc::now()).expect("marker writes");
    let directory_mode = fs::metadata(&home.support_bundles_dir)
        .expect("marker directory metadata reads")
        .permissions()
        .mode()
        & 0o777;
    let marker_mode = fs::metadata(home.support_bundles_dir.join(MARKER_FILE_NAME))
        .expect("marker metadata reads")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(directory_mode, 0o700);
    assert_eq!(marker_mode, 0o600);
    marker.clear().expect("marker clears");
}

#[cfg(unix)]
#[test]
fn should_reject_a_symlinked_marker_directory_without_touching_its_target() {
    use std::os::unix::fs::symlink;

    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let target = directory.path().join("outside");
    fs::create_dir(&target).expect("symlink target creates");
    fs::create_dir_all(&home.root).expect("home directory creates");
    symlink(&target, &home.support_bundles_dir).expect("marker directory symlink creates");

    let error = StartupMarker::begin(&home, "boot-symlink".to_owned(), Utc::now())
        .expect_err("symlinked marker directory must reject");

    assert_eq!(error.kind(), io::ErrorKind::InvalidInput);
    assert!(fs::read_dir(target).expect("target reads").next().is_none());
}

#[test]
fn should_ignore_a_corrupt_previous_marker_that_cannot_match_the_report_schema() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    fs::create_dir_all(&home.support_bundles_dir).expect("marker directory creates");
    let invalid = serde_json::json!({
        "schema_version": MARKER_SCHEMA_VERSION,
        "boot_id": "x".repeat(MAX_BOOT_ID_BYTES + 1),
        "boot_phase": "invented_phase",
        "owner": {"pid": 0, "started_at": Utc::now()},
    });
    fs::write(
        home.support_bundles_dir.join(MARKER_FILE_NAME),
        serde_json::to_vec(&invalid).expect("invalid marker serializes"),
    )
    .expect("invalid marker writes");

    let marker = StartupMarker::begin_with_owner(
        &home,
        "boot-current".to_owned(),
        owner(200, Utc::now()),
        |_| None,
    )
    .expect("current marker writes");

    assert!(marker.previous_crash().is_none());
    marker.clear().expect("marker clears");
}
