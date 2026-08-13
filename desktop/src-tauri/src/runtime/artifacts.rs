use std::collections::BTreeMap;
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::Path;
use std::time::Duration;

use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64;
use minisign_verify::{PublicKey, Signature};
use reqwest::blocking::Client;
use semver::Version;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use thiserror::Error;
use url::Url;

const MANIFEST_SCHEMA_VERSION: u8 = 1;
const MAX_MANIFEST_BYTES: usize = 1024 * 1024;
const MAX_SIGNATURE_BYTES: u64 = 64 * 1024;
const MAX_ARCHIVE_BYTES: u64 = 512 * 1024 * 1024;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct RuntimeManifest {
    pub schema_version: u8,
    pub version: Version,
    pub platforms: BTreeMap<String, PlatformArtifact>,
    pub schema_heads: BTreeMap<String, u64>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct PlatformArtifact {
    pub url: Url,
    pub sha256: String,
    pub size: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedArtifact {
    pub version: Version,
    pub artifact: PlatformArtifact,
    pub schema_heads: BTreeMap<String, u64>,
}

#[derive(Debug, Error)]
pub enum ArtifactError {
    #[error("runtime manifest signature is missing or invalid")]
    Signature,
    #[error("runtime manifest is not canonical JSON")]
    NonCanonical,
    #[error("runtime manifest is malformed")]
    Malformed,
    #[error("runtime manifest schema version is unsupported")]
    SchemaVersion,
    #[error("runtime manifest does not carry schema heads")]
    SchemaHeads,
    #[error("runtime manifest has no artifact for {0}")]
    UnsupportedPlatform(String),
    #[error("runtime artifact metadata is invalid")]
    InvalidArtifact,
    #[error("runtime artifact download failed")]
    Network,
    #[error("runtime artifact exceeds the declared or supported size")]
    TooLarge,
    #[error("runtime artifact digest does not match the signed manifest")]
    DigestMismatch,
    #[error("runtime artifact could not be written")]
    Io(#[source] std::io::Error),
}

pub fn verify_manifest(
    bytes: &[u8],
    minisig: &str,
    public_key: &str,
    target: &str,
    installed: Option<&Version>,
) -> Result<Option<VerifiedArtifact>, ArtifactError> {
    verify_manifest_with_keys(bytes, minisig, &[public_key], target, installed)
}

pub fn verify_manifest_with_keys(
    bytes: &[u8],
    minisig: &str,
    public_keys: &[&str],
    target: &str,
    installed: Option<&Version>,
) -> Result<Option<VerifiedArtifact>, ArtifactError> {
    if bytes.is_empty() || bytes.len() > MAX_MANIFEST_BYTES || minisig.trim().is_empty() {
        return Err(ArtifactError::Signature);
    }
    let signature = Signature::decode(minisig)
        .map_err(|error| diagnostic("parse runtime signature", error, ArtifactError::Signature))?;
    verify_signature_with_keys(bytes, &signature, public_keys)?;

    let value: Value = serde_json::from_slice(bytes)
        .map_err(|error| diagnostic("decode runtime manifest", error, ArtifactError::Malformed))?;
    let canonical = canonical_json(&value).map_err(|error| {
        diagnostic(
            "canonicalize runtime manifest",
            error,
            ArtifactError::Malformed,
        )
    })?;
    if canonical != bytes {
        return Err(ArtifactError::NonCanonical);
    }
    let manifest: RuntimeManifest = serde_json::from_value(value)
        .map_err(|error| diagnostic("parse runtime manifest", error, ArtifactError::Malformed))?;
    validate_manifest(&manifest)?;
    if installed.is_some_and(|current| manifest.version <= *current) {
        return Ok(None);
    }
    let artifact = manifest
        .platforms
        .get(target)
        .cloned()
        .ok_or_else(|| ArtifactError::UnsupportedPlatform(target.to_owned()))?;
    validate_artifact(&artifact)?;
    Ok(Some(VerifiedArtifact {
        version: manifest.version,
        artifact,
        schema_heads: manifest.schema_heads,
    }))
}

fn verify_signature_with_keys(
    bytes: &[u8],
    signature: &Signature,
    public_keys: &[&str],
) -> Result<(), ArtifactError> {
    let mut failures = Vec::with_capacity(public_keys.len());
    for public_key in public_keys {
        let result = BASE64
            .decode(public_key.trim())
            .map_err(|error| format!("decode key: {error}"))
            .and_then(|key_file| {
                String::from_utf8(key_file).map_err(|error| format!("read key: {error}"))
            })
            .and_then(|key_file| {
                PublicKey::decode(&key_file).map_err(|error| format!("parse key: {error}"))
            })
            .and_then(|key| {
                key.verify(bytes, signature, false)
                    .map_err(|error| format!("verify signature: {error}"))
            });
        match result {
            Ok(()) => return Ok(()),
            Err(error) => failures.push(error),
        }
    }
    crate::logging::error(format!(
        "verify runtime manifest signature with {} configured key(s): {}",
        public_keys.len(),
        failures.join("; ")
    ));
    Err(ArtifactError::Signature)
}

pub fn fetch_signed_manifest(
    client: &Client,
    manifest_url: &Url,
) -> Result<(Vec<u8>, String), ArtifactError> {
    let signature_url = Url::parse(&format!("{}.minisig", manifest_url.as_str()))
        .map_err(|_| ArtifactError::Malformed)?;
    let bytes = fetch_bounded(client, manifest_url, MAX_MANIFEST_BYTES as u64)?;
    let signature = fetch_bounded(client, &signature_url, MAX_SIGNATURE_BYTES)?;
    let signature = String::from_utf8(signature).map_err(|error| {
        diagnostic(
            "decode runtime signature response",
            error,
            ArtifactError::Signature,
        )
    })?;
    Ok((bytes, signature))
}

fn diagnostic<E: std::fmt::Display>(
    context: &str,
    error: E,
    public: ArtifactError,
) -> ArtifactError {
    crate::logging::error(format!("{context}: {error}"));
    public
}

pub fn download_verified(
    client: &Client,
    artifact: &PlatformArtifact,
    destination: &Path,
    progress: &mut dyn FnMut(u8),
) -> Result<(), ArtifactError> {
    validate_artifact(artifact)?;
    if let Some(parent) = destination.parent() {
        fs::create_dir_all(parent).map_err(ArtifactError::Io)?;
    }
    let result = download_verified_inner(client, artifact, destination, progress);
    if result.is_err() {
        match fs::remove_file(destination) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => crate::logging::error(format!(
                "remove incomplete runtime artifact {}: {error}",
                destination.display()
            )),
        }
    }
    result
}

fn download_verified_inner(
    client: &Client,
    artifact: &PlatformArtifact,
    destination: &Path,
    progress: &mut dyn FnMut(u8),
) -> Result<(), ArtifactError> {
    let mut response = client
        .get(artifact.url.clone())
        .send()
        .map_err(|_| ArtifactError::Network)?;
    if !response.status().is_success() {
        return Err(ArtifactError::Network);
    }
    if response
        .content_length()
        .is_some_and(|length| length > artifact.size || length > MAX_ARCHIVE_BYTES)
    {
        return Err(ArtifactError::TooLarge);
    }
    let mut file = File::create(destination).map_err(ArtifactError::Io)?;
    let mut hasher = Sha256::new();
    let mut written = 0_u64;
    let mut last_progress = None;
    let mut buffer = [0_u8; 64 * 1024];
    loop {
        let read = response
            .read(&mut buffer)
            .map_err(|_| ArtifactError::Network)?;
        if read == 0 {
            break;
        }
        written = written
            .checked_add(read as u64)
            .ok_or(ArtifactError::TooLarge)?;
        if written > artifact.size || written > MAX_ARCHIVE_BYTES {
            return Err(ArtifactError::TooLarge);
        }
        let percentage = written.saturating_mul(100) / artifact.size;
        let percentage = percentage.min(100) as u8;
        if last_progress != Some(percentage) {
            progress(percentage);
            last_progress = Some(percentage);
        }
        hasher.update(&buffer[..read]);
        file.write_all(&buffer[..read]).map_err(ArtifactError::Io)?;
    }
    file.sync_all().map_err(ArtifactError::Io)?;
    if written != artifact.size {
        return Err(ArtifactError::DigestMismatch);
    }
    let digest = format!("{:x}", hasher.finalize());
    if !digest.eq_ignore_ascii_case(&artifact.sha256) {
        return Err(ArtifactError::DigestMismatch);
    }
    Ok(())
}

fn fetch_bounded(client: &Client, url: &Url, limit: u64) -> Result<Vec<u8>, ArtifactError> {
    let response = client
        .get(url.clone())
        .send()
        .map_err(|_| ArtifactError::Network)?;
    if !response.status().is_success() {
        return Err(ArtifactError::Network);
    }
    if response
        .content_length()
        .is_some_and(|length| length > limit)
    {
        return Err(ArtifactError::TooLarge);
    }
    let mut bytes = Vec::new();
    response
        .take(limit + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| ArtifactError::Network)?;
    if bytes.len() as u64 > limit {
        return Err(ArtifactError::TooLarge);
    }
    Ok(bytes)
}

fn validate_manifest(manifest: &RuntimeManifest) -> Result<(), ArtifactError> {
    if manifest.schema_version != MANIFEST_SCHEMA_VERSION {
        return Err(ArtifactError::SchemaVersion);
    }
    if manifest.schema_heads.is_empty() {
        return Err(ArtifactError::SchemaHeads);
    }
    Ok(())
}

fn validate_artifact(artifact: &PlatformArtifact) -> Result<(), ArtifactError> {
    let secure = artifact.url.scheme() == "https";
    let local_fixture = artifact.url.scheme() == "http"
        && matches!(
            artifact.url.host_str(),
            Some("localhost" | "127.0.0.1" | "[::1]")
        );
    let digest_valid =
        artifact.sha256.len() == 64 && artifact.sha256.bytes().all(|byte| byte.is_ascii_hexdigit());
    if (!secure && !local_fixture)
        || !digest_valid
        || artifact.size == 0
        || artifact.size > MAX_ARCHIVE_BYTES
    {
        return Err(ArtifactError::InvalidArtifact);
    }
    Ok(())
}

fn canonical_json(value: &Value) -> Result<Vec<u8>, serde_json::Error> {
    let canonical = canonical_value(value);
    serde_json::to_vec(&canonical)
}

fn canonical_value(value: &Value) -> Value {
    match value {
        Value::Array(values) => Value::Array(values.iter().map(canonical_value).collect()),
        Value::Object(values) => {
            let sorted: BTreeMap<_, _> = values
                .iter()
                .map(|(key, value)| (key.clone(), canonical_value(value)))
                .collect();
            Value::Object(sorted.into_iter().collect())
        }
        _ => value.clone(),
    }
}

pub fn http_client(timeout: Duration) -> Result<Client, ArtifactError> {
    Client::builder()
        .timeout(timeout)
        .build()
        .map_err(|_| ArtifactError::Network)
}

pub fn current_target() -> Result<&'static str, ArtifactError> {
    match (std::env::consts::OS, std::env::consts::ARCH) {
        ("macos", "aarch64") => Ok("darwin-aarch64"),
        ("macos", "x86_64") => Ok("darwin-x86_64"),
        ("linux", "x86_64") => Ok("linux-x86_64"),
        ("windows", "x86_64") => Ok("windows-x86_64"),
        _ => Err(ArtifactError::UnsupportedPlatform(format!(
            "{}-{}",
            std::env::consts::OS,
            std::env::consts::ARCH
        ))),
    }
}

#[cfg(test)]
mod tests {
    use std::io::Cursor;

    use minisign::{KeyPair, sign};
    use serde_json::json;

    use super::*;

    struct SignedManifest {
        bytes: Vec<u8>,
        signature: String,
        public_key: String,
    }

    fn sign_manifest_bytes(bytes: Vec<u8>) -> SignedManifest {
        let pair = KeyPair::generate_unencrypted_keypair().expect("test key generates");
        let signature = sign(
            Some(&pair.pk),
            &pair.sk,
            Cursor::new(&bytes),
            Some("timestamp:0"),
            Some("test signature"),
        )
        .expect("manifest signs")
        .to_string();
        SignedManifest {
            bytes,
            signature,
            public_key: BASE64.encode(pair.pk.to_box().expect("public key boxes").to_string()),
        }
    }

    fn manifest_value(version: &str, schema_heads: Value) -> Value {
        json!({
            "schema_version": 1,
            "version": version,
            "platforms": {
                "darwin-aarch64": {
                    "url": "http://127.0.0.1:43119/runtime.tar.gz",
                    "sha256": "a".repeat(64),
                    "size": 3
                }
            },
            "schema_heads": schema_heads
        })
    }

    fn signed_manifest(version: &str, schema_heads: Value) -> SignedManifest {
        let value = manifest_value(version, schema_heads);
        let bytes = canonical_json(&value).expect("manifest canonicalizes");
        sign_manifest_bytes(bytes)
    }

    #[test]
    fn should_verify_canonical_manifest_before_selecting_platform_artifact() {
        let signed = signed_manifest("0.4.0", json!({"global": 12}));
        let verified = verify_manifest(
            &signed.bytes,
            &signed.signature,
            &signed.public_key,
            "darwin-aarch64",
            Some(&Version::parse("0.3.0").expect("version parses")),
        )
        .expect("signed manifest verifies")
        .expect("newer artifact is selected");

        assert_eq!(
            verified.version,
            Version::parse("0.4.0").expect("version parses")
        );
        assert_eq!(verified.artifact.size, 3);
    }

    #[test]
    fn should_refuse_missing_invalid_or_unpinned_manifest_signatures() {
        let signed = signed_manifest("0.4.0", json!({"global": 12}));
        let other = KeyPair::generate_unencrypted_keypair().expect("other key generates");
        for signature in ["", "invalid signature"] {
            assert!(matches!(
                verify_manifest(
                    &signed.bytes,
                    signature,
                    &signed.public_key,
                    "darwin-aarch64",
                    None,
                ),
                Err(ArtifactError::Signature)
            ));
        }
        assert!(matches!(
            verify_manifest(
                &signed.bytes,
                &signed.signature,
                &BASE64.encode(
                    other
                        .pk
                        .to_box()
                        .expect("other public key boxes")
                        .to_string(),
                ),
                "darwin-aarch64",
                None,
            ),
            Err(ArtifactError::Signature)
        ));
    }

    #[test]
    fn should_accept_previous_manifest_key_during_signing_key_rotation() {
        let signed = signed_manifest("0.4.0", json!({"global": 12}));
        let current = KeyPair::generate_unencrypted_keypair().expect("current key generates");
        let current_public_key = BASE64.encode(
            current
                .pk
                .to_box()
                .expect("current public key boxes")
                .to_string(),
        );

        let verified = verify_manifest_with_keys(
            &signed.bytes,
            &signed.signature,
            &[current_public_key.as_str(), signed.public_key.as_str()],
            "darwin-aarch64",
            None,
        )
        .expect("previous signing key remains valid during rotation")
        .expect("artifact is selected");

        assert_eq!(
            verified.version,
            Version::parse("0.4.0").expect("version parses")
        );
    }

    #[test]
    fn should_refuse_signed_manifest_when_json_is_not_canonical() {
        let value = manifest_value("0.4.0", json!({"global": 12}));
        let bytes = serde_json::to_vec(&RuntimeManifest {
            schema_version: 1,
            version: Version::parse("0.4.0").expect("version parses"),
            platforms: serde_json::from_value(value["platforms"].clone()).expect("platforms parse"),
            schema_heads: BTreeMap::from([("global".to_owned(), 12)]),
        })
        .expect("noncanonical manifest serializes");
        let signed = sign_manifest_bytes(bytes);

        assert!(matches!(
            verify_manifest(
                &signed.bytes,
                &signed.signature,
                &signed.public_key,
                "darwin-aarch64",
                None,
            ),
            Err(ArtifactError::NonCanonical)
        ));
    }

    #[test]
    fn should_refuse_manifest_without_migration_heads() {
        let signed = signed_manifest("0.4.0", json!({}));
        assert!(matches!(
            verify_manifest(
                &signed.bytes,
                &signed.signature,
                &signed.public_key,
                "darwin-aarch64",
                None,
            ),
            Err(ArtifactError::SchemaHeads)
        ));
    }

    #[test]
    fn should_ignore_equal_or_older_versions_and_refuse_missing_target() {
        let signed = signed_manifest("0.3.0", json!({"global": 12}));
        let current = Version::parse("0.3.0").expect("version parses");
        assert!(
            verify_manifest(
                &signed.bytes,
                &signed.signature,
                &signed.public_key,
                "darwin-aarch64",
                Some(&current),
            )
            .expect("manifest verifies")
            .is_none()
        );
        assert!(matches!(
            verify_manifest(
                &signed.bytes,
                &signed.signature,
                &signed.public_key,
                "windows-x86_64",
                None,
            ),
            Err(ArtifactError::UnsupportedPlatform(target)) if target == "windows-x86_64"
        ));
    }

    #[test]
    fn should_allow_ipv6_loopback_only_for_local_http_fixtures() {
        let artifact = PlatformArtifact {
            url: Url::parse("http://[::1]:43119/runtime.tar.gz").expect("IPv6 URL parses"),
            sha256: "a".repeat(64),
            size: 3,
        };

        assert!(validate_artifact(&artifact).is_ok());
    }
}
