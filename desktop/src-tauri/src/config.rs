use std::fs;
use std::path::Path;
use std::time::Duration;

use serde::Deserialize;

use crate::errors::{ShellError, ShellErrorCode};
use crate::home::CompozyHome;

const DEFAULT_UPDATE_INTERVAL: Duration = Duration::from_secs(6 * 60 * 60);
const MIN_UPDATE_INTERVAL: Duration = Duration::from_secs(15 * 60);
const MAX_UPDATE_INTERVAL: Duration = Duration::from_secs(168 * 60 * 60);

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AppConfig {
    pub update_check: bool,
    pub update_check_interval: Duration,
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            update_check: true,
            update_check_interval: DEFAULT_UPDATE_INTERVAL,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AppConfigLoad {
    pub config: AppConfig,
    pub consumer_enabled: bool,
    pub diagnostic: Option<ShellError>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct AppSection {
    #[serde(default = "default_update_check")]
    update_check: bool,
    #[serde(default = "default_update_interval")]
    update_check_interval: String,
}

fn default_update_check() -> bool {
    true
}

fn default_update_interval() -> String {
    "6h".to_owned()
}

impl AppConfig {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self.update_check_interval < MIN_UPDATE_INTERVAL {
            return Err("app.update_check_interval must be at least 15m");
        }
        if self.update_check_interval > MAX_UPDATE_INTERVAL {
            return Err("app.update_check_interval must not exceed 168h");
        }
        Ok(())
    }
}

pub fn load(path: &Path, home: &CompozyHome) -> AppConfigLoad {
    let content = match fs::read_to_string(path) {
        Ok(content) => content,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return default_load(),
        Err(_) => return invalid_load(home, "The app settings could not be read."),
    };
    match parse(&content) {
        Ok(Some(config)) => AppConfigLoad {
            consumer_enabled: config.update_check,
            config,
            diagnostic: None,
        },
        Ok(None) => default_load(),
        Err(message) => invalid_load(home, message),
    }
}

pub fn parse(content: &str) -> Result<Option<AppConfig>, &'static str> {
    let root: toml::Value =
        toml::from_str(content).map_err(|_| "The app settings file is invalid.")?;
    let Some(app) = root.get("app") else {
        return Ok(None);
    };
    let section: AppSection = app
        .clone()
        .try_into()
        .map_err(|_| "The app settings contain an unknown or invalid value.")?;
    let update_check_interval = parse_duration(&section.update_check_interval)
        .ok_or("app.update_check_interval must be a duration such as 6h.")?;
    let config = AppConfig {
        update_check: section.update_check,
        update_check_interval,
    };
    config.validate()?;
    Ok(Some(config))
}

fn parse_duration(value: &str) -> Option<Duration> {
    let mut remaining = value.trim().strip_prefix('+').unwrap_or(value.trim());
    if remaining == "0" {
        return Some(Duration::ZERO);
    }
    if remaining.is_empty() || remaining.starts_with('-') {
        return None;
    }

    let mut nanoseconds = 0_f64;
    while !remaining.is_empty() {
        let number_end = remaining.char_indices().find_map(|(index, character)| {
            (!character.is_ascii_digit() && character != '.').then_some(index)
        })?;
        let amount: f64 = remaining[..number_end].parse().ok()?;
        if !amount.is_finite() || amount < 0.0 {
            return None;
        }
        remaining = &remaining[number_end..];
        let (unit, scale) = duration_unit(remaining)?;
        nanoseconds += amount * scale;
        if !nanoseconds.is_finite() || nanoseconds > u64::MAX as f64 {
            return None;
        }
        remaining = &remaining[unit.len()..];
    }
    Some(Duration::from_nanos(nanoseconds as u64))
}

fn duration_unit(value: &str) -> Option<(&'static str, f64)> {
    [
        ("ns", 1.0),
        ("us", 1_000.0),
        ("µs", 1_000.0),
        ("μs", 1_000.0),
        ("ms", 1_000_000.0),
        ("s", 1_000_000_000.0),
        ("m", 60_000_000_000.0),
        ("h", 3_600_000_000_000.0),
    ]
    .into_iter()
    .find(|(unit, _)| value.starts_with(unit))
}

fn default_load() -> AppConfigLoad {
    AppConfigLoad {
        config: AppConfig::default(),
        consumer_enabled: true,
        diagnostic: None,
    }
}

fn invalid_load(home: &CompozyHome, message: &str) -> AppConfigLoad {
    AppConfigLoad {
        config: AppConfig::default(),
        consumer_enabled: false,
        diagnostic: Some(ShellError::new(
            ShellErrorCode::ConfigInvalid,
            message,
            home.app_log.clone(),
        )),
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;

    #[test]
    fn should_use_defaults_when_app_section_is_absent() {
        assert_eq!(parse("[http]\nport = 2123").expect("config parses"), None);
        assert_eq!(
            AppConfig::default().update_check_interval,
            Duration::from_secs(21_600)
        );
    }

    #[test]
    fn should_parse_valid_app_section() {
        let config = parse("[app]\nupdate_check = false\nupdate_check_interval = \"12h\"")
            .expect("config parses")
            .expect("app section exists");
        assert!(!config.update_check);
        assert_eq!(config.update_check_interval, Duration::from_secs(43_200));
    }

    #[test]
    fn should_accept_the_daemon_duration_grammar() {
        for (interval, expected) in [
            ("1h30m", Duration::from_secs(5_400)),
            ("0.25h", Duration::from_secs(900)),
            ("900000ms", Duration::from_secs(900)),
        ] {
            let config = parse(&format!("[app]\nupdate_check_interval = \"{interval}\""))
                .expect("config parses")
                .expect("app section exists");
            assert_eq!(
                config.update_check_interval, expected,
                "interval {interval}"
            );
        }
    }

    #[test]
    fn should_fail_closed_for_invalid_or_unknown_values() {
        let home = CompozyHome::from_root(PathBuf::from("/tmp/compozy"));
        let invalid = [
            "[app\nupdate_check = true",
            "[app]\nunknown = true",
            "[app]\nupdate_check_interval = \"never\"",
        ];
        for content in invalid {
            let path = tempfile::NamedTempFile::new().expect("fixture file opens");
            fs::write(path.path(), content).expect("fixture writes");
            let loaded = load(path.path(), &home);
            assert!(!loaded.consumer_enabled);
            assert_eq!(
                loaded.diagnostic.expect("diagnostic exists").code,
                ShellErrorCode::ConfigInvalid
            );
        }
    }

    #[test]
    fn should_reject_intervals_outside_shared_bounds() {
        for interval in ["5m", "200h"] {
            let result = parse(&format!(
                "[app]\nupdate_check = true\nupdate_check_interval = \"{interval}\""
            ));
            assert!(result.is_err());
        }
        for interval in ["15m", "168h"] {
            assert!(
                parse(&format!(
                    "[app]\nupdate_check = true\nupdate_check_interval = \"{interval}\""
                ))
                .is_ok()
            );
        }
    }
}
