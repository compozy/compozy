use std::fs;
use std::path::{Path, PathBuf};

use chrono::Utc;
use compozyos_desktop::config;
use compozyos_desktop::diagnostics::{DiagnosticReport, RuntimeDiagnosticMetadata};
use compozyos_desktop::errors::{ShellError, ShellErrorCode};
use compozyos_desktop::home::resolve_with;
use compozyos_desktop::record::AppRecord;
use compozyos_desktop::state::{ProvisionStage, ShellState, UpdateTarget};
use serde::Deserialize;
use serde_json::Value;

#[path = "../release_build_contract.rs"]
mod release_build_contract;

fn manifest_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn read_json(path: impl AsRef<Path>) -> Value {
    let path = path.as_ref();
    let bytes = fs::read(path)
        .unwrap_or_else(|error| panic!("JSON contract {} reads: {error}", path.display()));
    serde_json::from_slice(&bytes)
        .unwrap_or_else(|error| panic!("JSON contract {} parses: {error}", path.display()))
}

fn diagnostic_report(state: &ShellState) -> DiagnosticReport {
    DiagnosticReport::from_snapshot(
        "boot-contract",
        "0.4.1",
        None,
        None,
        &RuntimeDiagnosticMetadata::default(),
        state,
        &compozyos_desktop::record::AppUpdateStatus::default(),
    )
}

#[test]
fn should_validate_shared_records_and_every_transitional_state_against_schema() {
    let schema = read_json(manifest_dir().join("../schema/app-state.schema.json"));
    let validator = jsonschema::validator_for(&schema).expect("app-state schema compiles");
    for fixture in ["product.json", "error.json"] {
        let value = read_json(manifest_dir().join("../schema/fixtures").join(fixture));
        assert!(
            validator.is_valid(&value),
            "fixture {fixture} must validate"
        );
    }

    let states = [
        ShellState::Resolving,
        ShellState::Provisioning {
            stage: ProvisionStage::Download { pct: 45 },
        },
        ShellState::Provisioning {
            stage: ProvisionStage::Verify,
        },
        ShellState::Provisioning {
            stage: ProvisionStage::Install,
        },
        ShellState::Provisioning {
            stage: ProvisionStage::Start,
        },
        ShellState::Provisioning {
            stage: ProvisionStage::Ready,
        },
        ShellState::Starting { attempt: 2 },
        ShellState::Attaching,
        ShellState::Product {
            origin: url::Url::parse("http://localhost:2123").expect("fixture URL parses"),
            owned: true,
        },
        ShellState::Updating {
            target: UpdateTarget::Runtime,
        },
        ShellState::Updating {
            target: UpdateTarget::App,
        },
        ShellState::Disconnected {
            cause: compozyos_desktop::state::DisconnectCause::RuntimeDown,
        },
        ShellState::Disconnected {
            cause: compozyos_desktop::state::DisconnectCause::HealthCheckFailed,
        },
        ShellState::Skew {
            runtime: semver::Version::new(0, 2, 0),
            needed: semver::VersionReq::parse(">=0.3.0").expect("version requirement parses"),
            newer: false,
        },
        ShellState::Skew {
            runtime: semver::Version::new(1, 0, 0),
            needed: semver::VersionReq::parse(">=0.3.0, <1.0.0")
                .expect("version requirement parses"),
            newer: true,
        },
        ShellState::ShellError {
            error: ShellError::from_code(
                ShellErrorCode::RuntimeUnhealthy,
                PathBuf::from("/tmp/compozy.log"),
            ),
        },
    ];
    for state in states {
        let value = serde_json::to_value(AppRecord::new(
            42,
            Utc::now(),
            diagnostic_report(&state),
            state,
        ))
        .expect("record serializes");
        assert!(validator.is_valid(&value), "record must validate: {value}");
    }
    for code in ShellErrorCode::ALL {
        let record = AppRecord::new(
            42,
            Utc::now(),
            diagnostic_report(&ShellState::ShellError {
                error: ShellError::from_code(code, PathBuf::from("/tmp/compozy.log")),
            }),
            ShellState::ShellError {
                error: ShellError::from_code(code, PathBuf::from("/tmp/compozy.log")),
            },
        );
        let value = serde_json::to_value(record).expect("typed error record serializes");
        assert!(
            validator.is_valid(&value),
            "typed error record must validate: {value}"
        );
    }
    let diagnostic = DiagnosticReport::from_snapshot(
        "boot-contract",
        "0.4.1",
        None,
        Some(&ShellError::from_code(
            ShellErrorCode::ConfigInvalid,
            PathBuf::from("/tmp/desktop.log"),
        )),
        &RuntimeDiagnosticMetadata::default(),
        &ShellState::Resolving,
        &compozyos_desktop::record::AppUpdateStatus::default(),
    );
    let value = serde_json::to_value(AppRecord::new(
        42,
        Utc::now(),
        diagnostic,
        ShellState::Resolving,
    ))
    .expect("diagnostic record serializes");
    assert!(validator.is_valid(&value));
    assert_eq!(
        value["diagnostic_report"]["current_error"]["code"],
        "config_invalid"
    );
    assert!(validator.is_valid(&value["diagnostic_report"]));

    let value = serde_json::to_value(
        AppRecord::new(
            42,
            Utc::now(),
            diagnostic_report(&ShellState::Resolving),
            ShellState::Resolving,
        )
        .with_metadata(
            "0.4.0",
            "beta",
            compozyos_desktop::record::AppUpdateStatus {
                app_state: compozyos_desktop::record::AppUpdatePhase::Ready,
                runtime_state: compozyos_desktop::record::RuntimeUpdatePhase::Available,
                app_available: Some("0.5.0".to_owned()),
                runtime_available: Some("0.5.0".to_owned()),
                last_checked_at: Some(Utc::now()),
                last_error: None,
            },
        ),
    )
    .expect("update metadata record serializes");
    assert!(validator.is_valid(&value));
}

#[derive(Deserialize)]
struct HomeFixture {
    cases: Vec<HomeCase>,
}

#[derive(Deserialize)]
struct HomeCase {
    name: String,
    compozy_home: Option<String>,
    user_home: String,
    cwd: String,
    want: String,
}

#[test]
fn should_resolve_every_shared_home_fixture_byte_for_byte() {
    let fixture: HomeFixture = serde_json::from_value(read_json(
        manifest_dir().join("../schema/fixtures/home-resolution.json"),
    ))
    .expect("home fixture parses");
    for case in fixture.cases {
        let actual = resolve_with(
            case.compozy_home.as_deref(),
            Some(Path::new(&case.user_home)),
            Path::new(&case.cwd),
        )
        .expect("fixture resolves");
        assert_eq!(actual.root, PathBuf::from(case.want), "case {}", case.name);
    }
}

#[derive(Deserialize)]
struct ConfigFixture {
    defaults: ConfigDefaults,
    valid_intervals: Vec<String>,
    invalid_intervals: Vec<String>,
    invalid_documents: Vec<InvalidConfigDocument>,
}

#[derive(Deserialize)]
struct ConfigDefaults {
    update_check: bool,
    update_check_interval: String,
}

#[derive(Deserialize)]
struct InvalidConfigDocument {
    name: String,
    toml: String,
}

#[test]
fn should_apply_the_shared_config_defaults_and_interval_bounds() {
    let fixture: ConfigFixture = serde_json::from_value(read_json(
        manifest_dir().join("../schema/fixtures/app-config.json"),
    ))
    .expect("config fixture parses");
    let defaults = config::parse("")
        .expect("empty config parses")
        .unwrap_or_default();
    assert_eq!(defaults.update_check, fixture.defaults.update_check);
    let configured_defaults = config::parse(&format!(
        "[app]\nupdate_check = {}\nupdate_check_interval = \"{}\"",
        fixture.defaults.update_check, fixture.defaults.update_check_interval
    ))
    .expect("fixture defaults parse")
    .expect("fixture app section exists");
    assert_eq!(defaults, configured_defaults);
    for interval in fixture.valid_intervals {
        assert!(
            config::parse(&format!(
                "[app]\nupdate_check = true\nupdate_check_interval = \"{interval}\""
            ))
            .is_ok(),
            "valid interval {interval}"
        );
    }
    for interval in fixture.invalid_intervals {
        assert!(
            config::parse(&format!(
                "[app]\nupdate_check = true\nupdate_check_interval = \"{interval}\""
            ))
            .is_err(),
            "invalid interval {interval}"
        );
    }
    for document in fixture.invalid_documents {
        assert!(
            config::parse(&document.toml).is_err(),
            "invalid document {}",
            document.name
        );
    }
}

#[test]
fn should_grant_only_desktop_control_to_the_local_boot_window() {
    let config = read_json(manifest_dir().join("tauri.conf.json"));
    assert_eq!(config["identifier"], "com.compozy.os");
    assert_eq!(config["productName"], "CompozyOS");
    assert_eq!(config["app"]["withGlobalTauri"], false);
    assert_eq!(config["app"]["windows"].as_array().map(Vec::len), Some(1));
    assert_eq!(config["app"]["windows"][0]["label"], "boot");

    let capability = read_json(manifest_dir().join("capabilities/boot.json"));
    assert_eq!(capability["windows"], serde_json::json!(["boot"]));
    assert_eq!(
        capability["permissions"],
        serde_json::json!(["allow-shell-control"])
    );
    assert!(capability.get("remote").is_none());
}

#[test]
fn should_grant_zoom_only_to_the_daemon_served_main_window() {
    let config = read_json(manifest_dir().join("tauri.conf.json"));
    assert_eq!(
        config["app"]["security"]["capabilities"],
        serde_json::json!(["boot", "main-zoom"])
    );

    let capability = read_json(manifest_dir().join("capabilities/main-zoom.json"));
    assert_eq!(capability["local"], false);
    assert_eq!(capability["windows"], serde_json::json!(["main"]));
    assert_eq!(
        capability["remote"]["urls"],
        serde_json::json!(["http://localhost:*", "http://127.0.0.1:*"])
    );
    assert_eq!(
        capability["permissions"],
        serde_json::json!(["core:webview:allow-set-webview-zoom"])
    );

    let windowing_source = fs::read_to_string(manifest_dir().join("src/windowing.rs"))
        .expect("window builder source reads");
    assert!(
        windowing_source.contains(".zoom_hotkeys_enabled(true)"),
        "daemon-served main window must enable native zoom hotkeys"
    );
}

#[test]
fn should_initialize_updater_plugin_from_base_tauri_config() {
    let config = serde_json::from_value(read_json(manifest_dir().join("tauri.conf.json")))
        .expect("base Tauri config parses");
    let mut context = tauri::test::mock_context(tauri::test::noop_assets());
    *context.config_mut() = config;

    tauri::test::mock_builder()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .build(context)
        .expect("development shell initializes updater plugin");
}

#[test]
fn should_require_complete_release_build_configuration() {
    let complete_config = r#"{
        "version": "0.4.0-beta.2",
        "plugins": {
            "updater": {
                "endpoints": ["https://releases.compozy.com/desktop/beta/latest.json"],
                "pubkey": "base64-minisign-public-key"
            }
        }
    }"#;
    let valid = || {
        release_build_contract::validate_release_build(
            Some("beta"),
            Some(complete_config),
            Some("private-key"),
            Some("private-key-password"),
        )
    };

    assert_eq!(
        valid().expect("complete release configuration validates"),
        "beta"
    );

    for (channel, config, private_key, password, expected) in [
        (
            None,
            Some(complete_config),
            Some("private-key"),
            Some("private-key-password"),
            "COMPOZY_RELEASE_CHANNEL is required",
        ),
        (
            Some("stable"),
            Some(complete_config),
            Some("private-key"),
            Some("private-key-password"),
            "COMPOZY_RELEASE_CHANNEL must be beta or legacy",
        ),
        (
            Some("beta"),
            None,
            Some("private-key"),
            Some("private-key-password"),
            "TAURI_CONFIG is required",
        ),
        (
            Some("beta"),
            Some(
                r#"{"version":"0.4.0-beta.2","plugins":{"updater":{"endpoints":[],"pubkey":"key"}}}"#,
            ),
            Some("private-key"),
            Some("private-key-password"),
            "TAURI_CONFIG updater feed must contain exactly one endpoint",
        ),
        (
            Some("beta"),
            Some(
                r#"{"version":"0.4.0-beta.2","plugins":{"updater":{"endpoints":["https://releases.compozy.com/desktop/legacy/latest.json"],"pubkey":"key"}}}"#,
            ),
            Some("private-key"),
            Some("private-key-password"),
            "TAURI_CONFIG updater feed must be https://releases.compozy.com/desktop/beta/latest.json",
        ),
        (
            Some("beta"),
            Some(
                r#"{"version":"0.4.0-beta.2","plugins":{"updater":{"endpoints":["https://releases.compozy.com/desktop/beta/latest.json"],"pubkey":""}}}"#,
            ),
            Some("private-key"),
            Some("private-key-password"),
            "TAURI_CONFIG updater public key is missing",
        ),
        (
            Some("beta"),
            Some(complete_config),
            None,
            Some("private-key-password"),
            "TAURI_SIGNING_PRIVATE_KEY is required",
        ),
        (
            Some("beta"),
            Some(complete_config),
            Some("private-key"),
            None,
            "TAURI_SIGNING_PRIVATE_KEY_PASSWORD is required",
        ),
    ] {
        let error =
            release_build_contract::validate_release_build(channel, config, private_key, password)
                .expect_err("incomplete release configuration must fail");
        assert!(
            error.contains(expected),
            "error {error:?} must contain {expected:?}"
        );
    }
}
// Suite: desktop shared contracts
// Invariant: every Rust-produced desktop record and shared fixture remains readable by the
// daemon-owned schemas and preserves the Tauri capability boundary.
