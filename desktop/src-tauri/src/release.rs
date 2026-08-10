use serde::Deserialize;
use tauri::Config;
use thiserror::Error;
use url::Url;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReleaseConfig {
    pub channel: String,
    pub app_endpoint: Url,
    pub runtime_manifest: Url,
    pub current_public_key: String,
    pub previous_public_key: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct UpdaterPluginConfig {
    endpoints: Vec<Url>,
    pubkey: String,
    #[serde(default)]
    previous_pubkey: Option<String>,
}

#[derive(Debug, Error)]
pub enum ReleaseConfigError {
    #[error("release updater configuration is missing")]
    MissingUpdater,
    #[error("release updater configuration is malformed")]
    MalformedUpdater(#[source] serde_json::Error),
    #[error("release updater endpoint is missing")]
    MissingEndpoint,
    #[error("release runtime endpoint cannot be derived")]
    InvalidRuntimeEndpoint,
    #[error("release updater public key is missing")]
    MissingPublicKey,
}

impl ReleaseConfig {
    pub fn compiled(config: &Config) -> Result<Option<Self>, ReleaseConfigError> {
        let Some(channel) = option_env!("COMPOZY_RELEASE_CHANNEL") else {
            return Ok(None);
        };
        let channel = channel.trim();
        if channel.is_empty() {
            return Ok(None);
        }
        let plugin = config
            .plugins
            .0
            .get("updater")
            .ok_or(ReleaseConfigError::MissingUpdater)?
            .clone();
        let updater: UpdaterPluginConfig =
            serde_json::from_value(plugin).map_err(ReleaseConfigError::MalformedUpdater)?;
        let app_endpoint = updater
            .endpoints
            .into_iter()
            .next()
            .ok_or(ReleaseConfigError::MissingEndpoint)?;
        let runtime_manifest = sibling_manifest(&app_endpoint, "runtime.json")
            .ok_or(ReleaseConfigError::InvalidRuntimeEndpoint)?;
        let current_public_key = updater.pubkey.trim().to_owned();
        if current_public_key.is_empty() {
            return Err(ReleaseConfigError::MissingPublicKey);
        }
        Ok(Some(Self {
            channel: channel.to_owned(),
            app_endpoint,
            runtime_manifest,
            current_public_key,
            previous_public_key: updater.previous_pubkey.filter(|key| !key.trim().is_empty()),
        }))
    }
}

fn sibling_manifest(endpoint: &Url, file_name: &str) -> Option<Url> {
    let mut url = endpoint.clone();
    let mut segments = url.path_segments()?.collect::<Vec<_>>();
    segments.pop()?;
    segments.push(file_name);
    url.set_path(&segments.join("/"));
    url.set_query(None);
    url.set_fragment(None);
    Some(url)
}
