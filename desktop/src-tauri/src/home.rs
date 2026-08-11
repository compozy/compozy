use std::env;
use std::path::{Path, PathBuf};

use thiserror::Error;

pub const COMPOZY_HOME_ENV: &str = "COMPOZY_HOME";
pub const COMPOZY_DIR_NAME: &str = ".compozy";

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CompozyHome {
    pub root: PathBuf,
    pub config_file: PathBuf,
    pub daemon_info: PathBuf,
    pub app_record: PathBuf,
    pub logs_dir: PathBuf,
    pub runtime_log: PathBuf,
    pub app_log: PathBuf,
    pub app_binary: PathBuf,
    pub provenance: PathBuf,
    pub update_lock: PathBuf,
    pub update_journal: PathBuf,
    pub app_socket: PathBuf,
    pub app_update_intent: PathBuf,
    pub support_bundles_dir: PathBuf,
}

#[derive(Debug, Error)]
pub enum HomeError {
    #[error("resolve current directory: {0}")]
    CurrentDir(#[source] std::io::Error),
    #[error("resolve user home directory")]
    UserHomeUnavailable,
}

impl CompozyHome {
    pub fn resolve() -> Result<Self, HomeError> {
        let current_dir = env::current_dir().map_err(HomeError::CurrentDir)?;
        resolve_with(
            env::var(COMPOZY_HOME_ENV).ok().as_deref(),
            dirs::home_dir().as_deref(),
            &current_dir,
        )
    }

    pub fn from_root(root: PathBuf) -> Self {
        let executable = if cfg!(windows) {
            "compozy.exe"
        } else {
            "compozy"
        };
        Self {
            config_file: root.join("config.toml"),
            daemon_info: root.join("daemon.json"),
            app_record: root.join("app.json"),
            logs_dir: root.join("logs"),
            runtime_log: root.join("logs").join("compozy.log"),
            app_log: root.join("logs").join("desktop.log"),
            app_binary: root.join("bin").join(executable),
            provenance: root.join("bin").join(".desktop-provenance.json"),
            update_lock: root.join("update.lock"),
            update_journal: root.join("bin").join(".update-journal.json"),
            app_socket: root.join("app.sock"),
            app_update_intent: root.join(".app-update-intent.json"),
            support_bundles_dir: root.join("support-bundles"),
            root,
        }
    }
}

pub fn resolve_with(
    override_value: Option<&str>,
    user_home: Option<&Path>,
    current_dir: &Path,
) -> Result<CompozyHome, HomeError> {
    let root = if let Some(value) = override_value.filter(|value| !value.trim().is_empty()) {
        absolute_expanded(value, user_home, current_dir)?
    } else {
        user_home
            .ok_or(HomeError::UserHomeUnavailable)?
            .join(COMPOZY_DIR_NAME)
    };
    Ok(CompozyHome::from_root(root))
}

fn absolute_expanded(
    raw: &str,
    user_home: Option<&Path>,
    current_dir: &Path,
) -> Result<PathBuf, HomeError> {
    let clean = raw.trim();
    let expanded = if clean == "~" {
        user_home
            .ok_or(HomeError::UserHomeUnavailable)?
            .to_path_buf()
    } else if let Some(rest) = clean.strip_prefix("~/") {
        user_home.ok_or(HomeError::UserHomeUnavailable)?.join(rest)
    } else {
        PathBuf::from(clean)
    };
    Ok(if expanded.is_absolute() {
        expanded
    } else {
        current_dir.join(expanded)
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_resolve_explicit_absolute_home() {
        let home = resolve_with(
            Some("/tmp/compozy-test"),
            Some(Path::new("/home/operator")),
            Path::new("/work"),
        )
        .expect("home resolves");
        assert_eq!(home.root, PathBuf::from("/tmp/compozy-test"));
    }

    #[test]
    fn should_resolve_default_home_like_go_runtime() {
        let home = resolve_with(None, Some(Path::new("/home/operator")), Path::new("/work"))
            .expect("home resolves");
        assert_eq!(home.root, PathBuf::from("/home/operator/.compozy"));
    }

    #[test]
    fn should_match_go_absolute_and_tilde_semantics() {
        let cases = [
            (Some("relative/home"), PathBuf::from("/work/relative/home")),
            (Some(" ~/custom "), PathBuf::from("/home/operator/custom")),
            (Some("   "), PathBuf::from("/home/operator/.compozy")),
            (None, PathBuf::from("/home/operator/.compozy")),
        ];
        for (value, expected) in cases {
            let home = resolve_with(value, Some(Path::new("/home/operator")), Path::new("/work"))
                .expect("fixture home resolves");
            assert_eq!(home.root, expected);
        }
    }

    #[test]
    fn should_fail_when_user_home_is_required_but_unavailable() {
        let error = resolve_with(None, None, Path::new("/work")).expect_err("home should fail");
        assert!(matches!(error, HomeError::UserHomeUnavailable));
    }
}
