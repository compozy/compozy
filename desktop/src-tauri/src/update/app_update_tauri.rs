use std::fs::{self, File};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use base64::Engine;
use base64::engine::general_purpose::STANDARD;
use chrono::Utc;
use minisign_verify::{PublicKey, Signature};
use tauri::{AppHandle, Runtime};
use tauri_plugin_updater::{Error as TauriUpdaterError, Update, UpdaterExt};
use url::Url;

use crate::durability::sync_directory;

use super::app_update::{AppUpdateBackend, AppUpdateIntent, AppUpdaterError, PendingAppUpdate};

pub struct TauriAppUpdateBackend<R: Runtime> {
    app: AppHandle<R>,
    endpoint: Url,
    intent_path: PathBuf,
    payload_path: PathBuf,
}

impl<R: Runtime> TauriAppUpdateBackend<R> {
    pub fn new(app: AppHandle<R>, endpoint: Url, intent_path: PathBuf) -> Self {
        let payload_path = intent_path.with_file_name(".app-update-payload");
        Self {
            app,
            endpoint,
            intent_path,
            payload_path,
        }
    }
}

impl<R: Runtime> AppUpdateBackend for TauriAppUpdateBackend<R> {
    fn check_and_download(
        &self,
        public_key: &str,
    ) -> Result<Option<Box<dyn PendingAppUpdate>>, AppUpdaterError> {
        let intent_slot = Arc::new(Mutex::new(None::<AppUpdateIntent>));
        let hook_slot = Arc::clone(&intent_slot);
        let hook_path = self.intent_path.clone();
        let hook_app = self.app.clone();
        let updater = self
            .app
            .updater_builder()
            .endpoints(vec![self.endpoint.clone()])
            .map_err(map_tauri_error)?
            .pubkey(public_key)
            .timeout(Duration::from_secs(15))
            .on_before_exit(move || {
                if let Some(intent) = hook_slot
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner())
                    .clone()
                    && let Err(error) = write_intent(&hook_path, &intent)
                {
                    crate::logging::error(format!("persist app update intent: {error:?}"));
                }
                hook_app.cleanup_before_exit();
            })
            .build()
            .map_err(map_tauri_error)?;
        let update = tauri::async_runtime::block_on(updater.check()).map_err(map_tauri_error)?;
        let Some(update) = update else {
            remove_payload_if_present(&self.payload_path)?;
            return Ok(None);
        };
        let intent = AppUpdateIntent {
            current_version: update.current_version.clone(),
            target_version: update.version.clone(),
            consented: true,
            started_at: Utc::now(),
        };
        *intent_slot
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = Some(intent.clone());
        let bytes = read_verified_payload(&self.payload_path, &update.signature, public_key)?
            .map_or_else(
                || {
                    let bytes = tauri::async_runtime::block_on(update.download(|_, _| {}, || {}))
                        .map_err(map_tauri_error)?;
                    write_payload(&self.payload_path, &bytes)?;
                    Ok(bytes)
                },
                Ok,
            )?;
        Ok(Some(Box::new(TauriPendingUpdate {
            app: self.app.clone(),
            update,
            bytes,
            intent,
            intent_path: self.intent_path.clone(),
            payload_path: self.payload_path.clone(),
        })))
    }
}

struct TauriPendingUpdate<R: Runtime> {
    app: AppHandle<R>,
    update: Update,
    bytes: Vec<u8>,
    intent: AppUpdateIntent,
    intent_path: PathBuf,
    payload_path: PathBuf,
}

impl<R: Runtime> PendingAppUpdate for TauriPendingUpdate<R> {
    fn version(&self) -> &str {
        &self.update.version
    }

    fn install(self: Box<Self>) -> Result<(), AppUpdaterError> {
        write_intent(&self.intent_path, &self.intent)?;
        #[cfg(target_os = "macos")]
        install_macos_with_backup(&self.update, &self.bytes)?;
        #[cfg(not(target_os = "macos"))]
        self.update
            .install(&self.bytes)
            .map_err(|_| AppUpdaterError::Install)?;
        if let Err(error) = remove_payload_if_present(&self.payload_path) {
            crate::logging::error(format!("remove installed app update payload: {error:?}"));
        }
        self.app.restart();
    }
}

fn read_verified_payload(
    path: &Path,
    release_signature: &str,
    public_key: &str,
) -> Result<Option<Vec<u8>>, AppUpdaterError> {
    let bytes = match fs::read(path) {
        Ok(bytes) => bytes,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(AppUpdaterError::Intent(error)),
    };
    if verify_signature(&bytes, release_signature, public_key) {
        return Ok(Some(bytes));
    }
    remove_payload_if_present(path)?;
    Ok(None)
}

fn verify_signature(bytes: &[u8], release_signature: &str, public_key: &str) -> bool {
    let decoded_key = STANDARD
        .decode(public_key)
        .ok()
        .and_then(|value| String::from_utf8(value).ok());
    let decoded_signature = STANDARD
        .decode(release_signature)
        .ok()
        .and_then(|value| String::from_utf8(value).ok());
    decoded_key
        .and_then(|value| PublicKey::decode(&value).ok())
        .zip(decoded_signature.and_then(|value| Signature::decode(&value).ok()))
        .is_some_and(|(key, signature)| key.verify(bytes, &signature, true).is_ok())
}

fn write_payload(path: &Path, bytes: &[u8]) -> Result<(), AppUpdaterError> {
    let parent = path
        .parent()
        .ok_or_else(|| intent_error("payload has no parent"))?;
    fs::create_dir_all(parent).map_err(AppUpdaterError::Intent)?;
    let temporary = parent.join(".app-update-payload.tmp");
    let mut file = File::create(&temporary).map_err(AppUpdaterError::Intent)?;
    file.write_all(bytes).map_err(AppUpdaterError::Intent)?;
    file.sync_all().map_err(AppUpdaterError::Intent)?;
    fs::rename(&temporary, path).map_err(AppUpdaterError::Intent)?;
    sync_directory(parent).map_err(AppUpdaterError::Intent)
}

pub(super) fn write_intent(path: &Path, intent: &AppUpdateIntent) -> Result<(), AppUpdaterError> {
    let parent = path
        .parent()
        .ok_or_else(|| intent_error("intent has no parent"))?;
    fs::create_dir_all(parent).map_err(AppUpdaterError::Intent)?;
    let temporary = parent.join(".app-update-intent.json.tmp");
    let mut file = File::create(&temporary).map_err(AppUpdaterError::Intent)?;
    serde_json::to_writer(&mut file, intent).map_err(|error| {
        AppUpdaterError::Intent(std::io::Error::new(std::io::ErrorKind::InvalidData, error))
    })?;
    file.write_all(b"\n").map_err(AppUpdaterError::Intent)?;
    file.sync_all().map_err(AppUpdaterError::Intent)?;
    fs::rename(&temporary, path).map_err(AppUpdaterError::Intent)?;
    sync_directory(parent).map_err(AppUpdaterError::Intent)
}

pub(super) fn remove_intent(path: &Path) -> Result<(), AppUpdaterError> {
    remove_file_if_present(path)
}

fn remove_payload_if_present(path: &Path) -> Result<(), AppUpdaterError> {
    remove_file_if_present(path)
}

fn remove_file_if_present(path: &Path) -> Result<(), AppUpdaterError> {
    match fs::remove_file(path) {
        Ok(()) => {
            if let Some(parent) = path.parent() {
                sync_directory(parent).map_err(AppUpdaterError::Intent)?;
            }
            Ok(())
        }
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(AppUpdaterError::Intent(error)),
    }
}

fn intent_error(message: &'static str) -> AppUpdaterError {
    AppUpdaterError::Intent(std::io::Error::new(
        std::io::ErrorKind::InvalidInput,
        message,
    ))
}

fn map_tauri_error(error: TauriUpdaterError) -> AppUpdaterError {
    match error {
        TauriUpdaterError::ReleaseNotFound => AppUpdaterError::FeedNotFound,
        TauriUpdaterError::Serialization(_)
        | TauriUpdaterError::TargetNotFound(_)
        | TauriUpdaterError::TargetsNotFound(_) => AppUpdaterError::FeedInvalid,
        TauriUpdaterError::Minisign(_)
        | TauriUpdaterError::Base64(_)
        | TauriUpdaterError::SignatureUtf8(_) => AppUpdaterError::InvalidSignature,
        TauriUpdaterError::Reqwest(error) if error.is_decode() => AppUpdaterError::FeedInvalid,
        TauriUpdaterError::Reqwest(_) | TauriUpdaterError::Network(_) => AppUpdaterError::Network,
        _ => AppUpdaterError::Install,
    }
}

#[cfg(target_os = "macos")]
fn install_macos_with_backup(update: &Update, bytes: &[u8]) -> Result<(), AppUpdaterError> {
    let executable = std::env::current_exe().map_err(|_| AppUpdaterError::Install)?;
    let bundle = executable
        .ancestors()
        .find(|path| path.extension().is_some_and(|extension| extension == "app"))
        .ok_or(AppUpdaterError::Install)?;
    with_macos_backup(bundle, || {
        update.install(bytes).map_err(|_| AppUpdaterError::Install)
    })
}

#[cfg(target_os = "macos")]
pub(super) fn with_macos_backup(
    bundle: &Path,
    install: impl FnOnce() -> Result<(), AppUpdaterError>,
) -> Result<(), AppUpdaterError> {
    let backup = bundle.with_extension("app.update-backup");
    copy_tree(bundle, &backup)?;
    if let Err(error) = install() {
        if bundle.exists() {
            fs::remove_dir_all(bundle).map_err(|_| AppUpdaterError::Install)?;
        }
        fs::rename(&backup, bundle).map_err(|_| AppUpdaterError::Install)?;
        return Err(error);
    }
    if let Err(error) = fs::remove_dir_all(&backup) {
        crate::logging::error(format!("remove installed app backup: {error}"));
    }
    Ok(())
}

#[cfg(target_os = "macos")]
fn copy_tree(source: &Path, destination: &Path) -> Result<(), AppUpdaterError> {
    if destination.exists() {
        fs::remove_dir_all(destination).map_err(|_| AppUpdaterError::Install)?;
    }
    fs::create_dir_all(destination).map_err(|_| AppUpdaterError::Install)?;
    for entry in fs::read_dir(source).map_err(|_| AppUpdaterError::Install)? {
        let entry = entry.map_err(|_| AppUpdaterError::Install)?;
        let target = destination.join(entry.file_name());
        let file_type = entry.file_type().map_err(|_| AppUpdaterError::Install)?;
        if file_type.is_symlink() {
            let link = fs::read_link(entry.path()).map_err(|_| AppUpdaterError::Install)?;
            std::os::unix::fs::symlink(link, target).map_err(|_| AppUpdaterError::Install)?;
        } else if file_type.is_dir() {
            copy_tree(&entry.path(), &target)?;
        } else {
            fs::copy(entry.path(), target).map_err(|_| AppUpdaterError::Install)?;
        }
    }
    fs::set_permissions(
        destination,
        fs::metadata(source)
            .map_err(|_| AppUpdaterError::Install)?
            .permissions(),
    )
    .map_err(|_| AppUpdaterError::Install)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::io::Cursor;

    use minisign::{KeyPair, sign};

    use super::*;

    #[test]
    fn should_reuse_only_a_persisted_payload_with_the_current_release_signature() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let payload = directory.path().join("payload");
        let bytes = b"signed app update";
        let pair = KeyPair::generate_unencrypted_keypair().expect("test key generates");
        let signature = sign(
            Some(&pair.pk),
            &pair.sk,
            Cursor::new(bytes),
            Some("timestamp:0"),
            Some("test signature"),
        )
        .expect("payload signs")
        .to_string();
        let public_key = STANDARD.encode(pair.pk.to_box().expect("public key boxes").to_string());
        let release_signature = STANDARD.encode(signature);
        write_payload(&payload, bytes).expect("payload persists");

        assert_eq!(
            read_verified_payload(&payload, &release_signature, &public_key)
                .expect("payload reads"),
            Some(bytes.to_vec())
        );

        fs::write(&payload, b"tampered update").expect("payload tampers");
        assert_eq!(
            read_verified_payload(&payload, &release_signature, &public_key)
                .expect("tampered payload is handled"),
            None
        );
        assert!(!payload.exists());
    }
}
