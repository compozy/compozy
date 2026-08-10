use std::fs;
use std::path::{Path, PathBuf};

use chrono::Utc;
use compozyos_desktop::config;
use compozyos_desktop::errors::{ShellError, ShellErrorCode};
use compozyos_desktop::home::resolve_with;
use compozyos_desktop::record::AppRecord;
use compozyos_desktop::state::{ProvisionStage, ShellState, UpdateTarget};
use serde::Deserialize;
use serde_json::Value;

fn manifest_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn read_json(path: impl AsRef<Path>) -> Value {
    serde_json::from_slice(&fs::read(path).expect("JSON contract reads"))
        .expect("JSON contract parses")
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
        ShellState::Starting { attempt: 2 },
        ShellState::Attaching,
        ShellState::Product {
            origin: url::Url::parse("http://localhost:2123").expect("fixture URL parses"),
            owned: true,
        },
        ShellState::Updating {
            target: UpdateTarget::Runtime,
        },
        ShellState::Disconnected {
            cause: compozyos_desktop::state::DisconnectCause::RuntimeDown,
        },
        ShellState::Skew {
            runtime: semver::Version::new(0, 2, 0),
            needed: semver::VersionReq::parse(">=0.3.0").expect("version requirement parses"),
            newer: false,
        },
        ShellState::ShellError {
            error: ShellError::from_code(
                ShellErrorCode::RuntimeUnhealthy,
                PathBuf::from("/tmp/compozy.log"),
            ),
        },
    ];
    for state in states {
        let value =
            serde_json::to_value(AppRecord::new(42, Utc::now(), state)).expect("record serializes");
        assert!(validator.is_valid(&value), "record must validate: {value}");
    }
    let diagnostic = ShellError::from_code(
        ShellErrorCode::ConfigInvalid,
        PathBuf::from("/tmp/desktop.log"),
    );
    let value = serde_json::to_value(
        AppRecord::new(42, Utc::now(), ShellState::Resolving).with_diagnostic(Some(diagnostic)),
    )
    .expect("diagnostic record serializes");
    assert!(validator.is_valid(&value));
    assert_eq!(value["diagnostic"]["code"], "config_invalid");
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
fn should_grant_no_capability_or_global_api_to_the_remote_main_window() {
    let config = read_json(manifest_dir().join("tauri.conf.json"));
    assert_eq!(config["identifier"], "com.compozy.os");
    assert_eq!(config["productName"], "CompozyOS");
    assert_eq!(config["app"]["withGlobalTauri"], false);
    assert_eq!(config["app"]["windows"].as_array().map(Vec::len), Some(1));
    assert_eq!(config["app"]["windows"][0]["label"], "boot");

    let capability = read_json(manifest_dir().join("capabilities/boot.json"));
    assert_eq!(capability["windows"], serde_json::json!(["boot"]));
    assert_eq!(capability["permissions"], serde_json::json!([]));
    assert!(capability.get("remote").is_none());
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
