use std::collections::BTreeMap;
use std::fs;
use std::io::Read;
use std::path::{Path, PathBuf};

use flate2::read::GzDecoder;
use serde_json::json;
use tar::Archive;

use super::*;
use crate::diagnostics::RuntimeDiagnosticMetadata;
use crate::record::AppUpdateStatus;
use crate::state::ShellState;

fn report() -> DiagnosticReport {
    DiagnosticReport::from_snapshot(
        "boot-1",
        "1.2.3",
        None,
        None,
        &RuntimeDiagnosticMetadata::default(),
        &ShellState::Resolving,
        &AppUpdateStatus::default(),
    )
}

fn archive_entries(path: &Path) -> BTreeMap<PathBuf, Vec<u8>> {
    let file = fs::File::open(path).expect("bundle opens");
    let mut archive = Archive::new(GzDecoder::new(file));
    let mut result = BTreeMap::new();
    for entry in archive.entries().expect("bundle entries open") {
        let mut entry = entry.expect("bundle entry opens");
        let path = entry.path().expect("entry path reads").into_owned();
        let mut contents = Vec::new();
        entry.read_to_end(&mut contents).expect("entry reads");
        result.insert(path, contents);
    }
    result
}

#[test]
fn should_require_explicit_consent_before_creating_a_diagnostic_bundle() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let error = export(&home, &report(), false, None).expect_err("consent is required");
    assert!(matches!(error, DiagnosticExportError::ConsentRequired));
    assert!(!home.support_bundles_dir.exists());
}

#[test]
fn should_export_a_bounded_manifest_and_only_redacted_current_boot_log_tails() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    fs::create_dir_all(&home.logs_dir).expect("log directory creates");
    let desktop_log = [
        json!({"timestamp":"2026-08-11T12:00:00Z","level":"info","boot_id":"boot-1","message":format!("Runtime started in {}", home.root.display())}),
        json!({"timestamp":"2026-08-11T12:00:01Z","level":"error","boot_id":"boot-1","message":"config.toml contained an api_key=do-not-copy"}),
        json!({"timestamp":"2026-08-11T12:00:02Z","level":"info","boot_id":"another-boot","message":"Other boot event"}),
    ].into_iter().map(|value| value.to_string()).collect::<Vec<_>>().join("\n");
    fs::write(&home.app_log, desktop_log).expect("desktop log writes");
    fs::write(
        home.logs_dir.join(crate::logging::BOOTSTRAP_LOG_FILE_NAME),
        json!({"timestamp":"2026-08-11T12:00:03Z","level":"error","boot_id":"boot-1","message":"The boot window failed to open"}).to_string(),
    ).expect("bootstrap log writes");

    let exported = export(&home, &report(), true, None).expect("bundle exports");

    assert!(exported.bytes > 0 && exported.bytes <= MAX_DIAGNOSTIC_BUNDLE_BYTES);
    let entries = archive_entries(&exported.bundle_path);
    let manifest: DiagnosticExportManifest = serde_json::from_slice(
        entries
            .get(Path::new(MANIFEST_FILE_NAME))
            .expect("manifest entry exists"),
    )
    .expect("manifest decodes");
    assert_eq!(manifest.report, report());
    let desktop_log = String::from_utf8(
        entries
            .get(Path::new(DESKTOP_LOG_ARCHIVE_ENTRY))
            .expect("desktop log tail exists")
            .clone(),
    )
    .expect("desktop log tail is UTF-8");
    assert!(desktop_log.contains("Runtime started") && desktop_log.contains("[redacted]"));
    for forbidden in ["compozy-home", "config.toml", "do-not-copy", "another-boot"] {
        assert!(!desktop_log.contains(forbidden), "forbidden: {forbidden}");
    }
    assert!(entries.contains_key(Path::new(BOOTSTRAP_LOG_ARCHIVE_ENTRY)));
    assert!(!entries.contains_key(Path::new("compozy.log")));
}

#[cfg(unix)]
#[test]
fn should_create_private_default_bundle_storage_and_file() {
    use std::os::unix::fs::PermissionsExt;

    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let exported = export(&home, &report(), true, None).expect("bundle exports");
    let directory_mode = fs::metadata(&home.support_bundles_dir)
        .expect("bundle directory metadata reads")
        .permissions()
        .mode()
        & 0o777;
    let file_mode = fs::metadata(exported.bundle_path)
        .expect("bundle metadata reads")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(directory_mode, 0o700);
    assert_eq!(file_mode, 0o600);
}

#[cfg(unix)]
#[test]
fn should_repair_permissions_for_a_named_bundle_in_private_storage() {
    use std::os::unix::fs::PermissionsExt;

    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    fs::create_dir_all(&home.support_bundles_dir).expect("bundle parent creates");
    fs::set_permissions(&home.support_bundles_dir, fs::Permissions::from_mode(0o755))
        .expect("bundle parent mode sets");
    export(
        &home,
        &report(),
        true,
        Some(&home.support_bundles_dir.join("diagnostics.tar.gz")),
    )
    .expect("bundle exports");
    let parent_mode = fs::metadata(&home.support_bundles_dir)
        .expect("bundle parent metadata reads")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(parent_mode, 0o700);
}

#[test]
fn should_not_clobber_an_existing_destination_even_when_it_appears_before_commit() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    fs::create_dir_all(&home.support_bundles_dir).expect("bundle parent creates");
    let destination = home.support_bundles_dir.join("existing.tar.gz");
    fs::write(&destination, "existing bundle").expect("existing destination writes");
    let error = export(&home, &report(), true, Some(&destination))
        .expect_err("existing destination rejects export");
    assert!(matches!(error, DiagnosticExportError::AlreadyExists));
    assert_eq!(
        fs::read_to_string(destination).expect("existing destination reads"),
        "existing bundle"
    );
}

#[cfg(unix)]
#[test]
fn should_reject_a_symlinked_default_bundle_directory() {
    use std::os::unix::fs::symlink;

    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let target = directory.path().join("outside");
    fs::create_dir(&target).expect("symlink target creates");
    fs::create_dir_all(&home.root).expect("home directory creates");
    symlink(&target, &home.support_bundles_dir).expect("bundle directory symlink creates");
    let error = export(&home, &report(), true, None).expect_err("symlink rejects export");
    assert!(matches!(error, DiagnosticExportError::InvalidPath));
    assert!(fs::read_dir(target).expect("target reads").next().is_none());
}

#[test]
fn should_reject_an_output_path_outside_private_bundle_storage() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let target = directory.path().join("outside");
    fs::create_dir(&target).expect("outside directory creates");

    let error = export(
        &home,
        &report(),
        true,
        Some(&target.join("diagnostics.tar.gz")),
    )
    .expect_err("outside path rejects export");

    assert!(matches!(error, DiagnosticExportError::InvalidPath));
    assert!(fs::read_dir(target).expect("target reads").next().is_none());
}

#[test]
fn should_remove_an_oversized_diagnostic_bundle_attempt() {
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().join("compozy-home"));
    let mut oversized = report();
    oversized.boot_id = "x".repeat(49 * 1024);

    let error = export(&home, &oversized, true, None).expect_err("oversized bundle rejects");

    assert!(matches!(error, DiagnosticExportError::TooLarge));
    assert!(
        fs::read_dir(&home.support_bundles_dir)
            .expect("bundle directory reads")
            .all(|entry| entry
                .expect("bundle entry reads")
                .path()
                .extension()
                .and_then(|extension| extension.to_str())
                != Some("gz"))
    );
}
