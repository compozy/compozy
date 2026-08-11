use serde_json::Value;

const DISTRIBUTION_ORIGIN: &str = "https://releases.compozy.com";

pub fn validate_release_build(
    channel: Option<&str>,
    tauri_config: Option<&str>,
    private_key: Option<&str>,
    private_key_password: Option<&str>,
) -> Result<String, String> {
    let channel = required("COMPOZY_RELEASE_CHANNEL", channel)?;
    if !matches!(channel, "beta" | "legacy") {
        return Err(format!(
            "COMPOZY_RELEASE_CHANNEL must be beta or legacy, got {channel:?}"
        ));
    }

    let tauri_config = required("TAURI_CONFIG", tauri_config)?;
    let config: Value = serde_json::from_str(tauri_config)
        .map_err(|error| format!("TAURI_CONFIG must be valid JSON: {error}"))?;
    required_json_string(&config, &["version"], "release version")?;

    let endpoints = config
        .pointer("/plugins/updater/endpoints")
        .and_then(Value::as_array)
        .ok_or_else(|| "TAURI_CONFIG updater feed is missing".to_owned())?;
    if endpoints.len() != 1 {
        return Err("TAURI_CONFIG updater feed must contain exactly one endpoint".to_owned());
    }
    let endpoint = endpoints[0]
        .as_str()
        .map(str::trim)
        .filter(|endpoint| !endpoint.is_empty())
        .ok_or_else(|| "TAURI_CONFIG updater feed endpoint is missing".to_owned())?;
    let expected_endpoint = format!("{DISTRIBUTION_ORIGIN}/desktop/{channel}/latest.json");
    if endpoint != expected_endpoint {
        return Err(format!(
            "TAURI_CONFIG updater feed must be {expected_endpoint}, got {endpoint}"
        ));
    }
    required_json_string(
        &config,
        &["plugins", "updater", "pubkey"],
        "updater public key",
    )?;

    required("TAURI_SIGNING_PRIVATE_KEY", private_key)?;
    required("TAURI_SIGNING_PRIVATE_KEY_PASSWORD", private_key_password)?;
    Ok(channel.to_owned())
}

fn required<'a>(name: &str, value: Option<&'a str>) -> Result<&'a str, String> {
    value
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| format!("{name} is required for a desktop release build"))
}

fn required_json_string(config: &Value, path: &[&str], label: &str) -> Result<(), String> {
    let value = path
        .iter()
        .try_fold(config, |value, key| value.get(*key))
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty());
    if value.is_some() {
        return Ok(());
    }
    Err(format!("TAURI_CONFIG {label} is missing"))
}
