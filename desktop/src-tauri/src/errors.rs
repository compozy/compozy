use std::path::PathBuf;
use std::sync::LazyLock;

use regex::Regex;
use serde::{Deserialize, Serialize};

const MAX_SAFE_MESSAGE_BYTES: usize = 512;
const REDACTED: &str = "[redacted]";

static CLAIM_VALUE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)compozy_claim_[A-Za-z0-9._~-]+").expect("claim token regex is valid")
});
static NAMED_SECRET: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(
        r#"(?i)(claim_token|authorization|api[_-]?key|access[_-]?token)\s*[:=]\s*[^\s,;\"']+"#,
    )
    .expect("named secret regex is valid")
});
static BEARER: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)bearer\s+[A-Za-z0-9._~+/-]+=*").expect("bearer regex is valid")
});
static VAULT_REF: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r#"(?i)vault:[^\s,;\"']+"#).expect("vault reference regex is valid")
});

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ShellErrorCode {
    PortConflictForeign,
    RuntimeStartFailed,
    ProvisionNetwork,
    ProvisionDiskSpace,
    ProvisionPermission,
    RuntimeUnhealthy,
    HandshakeFailed,
    LoadDeadlineExceeded,
    UpdateLockHeld,
    QuiesceFailed,
    MigrationRecoveryRequired,
    ConfigInvalid,
    NotOwned,
}

impl ShellErrorCode {
    pub const ALL: [Self; 13] = [
        Self::PortConflictForeign,
        Self::RuntimeStartFailed,
        Self::ProvisionNetwork,
        Self::ProvisionDiskSpace,
        Self::ProvisionPermission,
        Self::RuntimeUnhealthy,
        Self::HandshakeFailed,
        Self::LoadDeadlineExceeded,
        Self::UpdateLockHeld,
        Self::QuiesceFailed,
        Self::MigrationRecoveryRequired,
        Self::ConfigInvalid,
        Self::NotOwned,
    ];

    pub const fn default_message(self) -> &'static str {
        match self {
            Self::PortConflictForeign => "Another service is using the runtime address.",
            Self::RuntimeStartFailed => "The runtime did not start.",
            Self::ProvisionNetwork => "The runtime could not be downloaded.",
            Self::ProvisionDiskSpace => "There is not enough disk space for the runtime.",
            Self::ProvisionPermission => "The runtime could not be written to disk.",
            Self::RuntimeUnhealthy => "The runtime is not responding.",
            Self::HandshakeFailed => "The runtime identity could not be verified.",
            Self::LoadDeadlineExceeded => "The product did not finish loading.",
            Self::UpdateLockHeld => "Another runtime change is in progress.",
            Self::QuiesceFailed => "The runtime could not pause new work safely.",
            Self::MigrationRecoveryRequired => "The runtime needs update recovery.",
            Self::ConfigInvalid => "The app settings are invalid.",
            Self::NotOwned => "This runtime is managed outside the desktop app.",
        }
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ShellError {
    pub code: ShellErrorCode,
    pub safe_message: String,
    pub log_path: PathBuf,
}

impl ShellError {
    pub fn new(code: ShellErrorCode, message: impl AsRef<str>, log_path: PathBuf) -> Self {
        let safe_message = sanitize_public_text(message.as_ref());
        Self {
            code,
            safe_message: if safe_message.is_empty() {
                code.default_message().to_owned()
            } else {
                safe_message
            },
            log_path,
        }
    }

    pub fn from_code(code: ShellErrorCode, log_path: PathBuf) -> Self {
        Self::new(code, code.default_message(), log_path)
    }
}

pub fn sanitize_public_text(input: &str) -> String {
    let printable: String = input
        .chars()
        .map(|character| {
            if character.is_control() && character != '\n' && character != '\t' {
                ' '
            } else {
                character
            }
        })
        .collect();
    let redacted = CLAIM_VALUE.replace_all(&printable, REDACTED);
    let redacted = NAMED_SECRET.replace_all(&redacted, REDACTED);
    let redacted = BEARER.replace_all(&redacted, REDACTED);
    let redacted = VAULT_REF.replace_all(&redacted, REDACTED);
    truncate_utf8(redacted.trim(), MAX_SAFE_MESSAGE_BYTES)
}

fn truncate_utf8(value: &str, limit: usize) -> String {
    if value.len() <= limit {
        return value.to_owned();
    }
    let mut boundary = limit;
    while !value.is_char_boundary(boundary) {
        boundary -= 1;
    }
    value[..boundary].to_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_map_every_error_code_to_safe_public_error() {
        for code in ShellErrorCode::ALL {
            let error = ShellError::from_code(code, PathBuf::from("/tmp/compozy.log"));
            assert!(!error.safe_message.is_empty());
            assert!(error.safe_message.len() <= MAX_SAFE_MESSAGE_BYTES);
            assert!(!error.log_path.as_os_str().is_empty());
        }
    }

    #[test]
    fn should_redact_adversarial_secret_corpus_and_bound_output() {
        let corpus = [
            "compozy_claim_very-secret-value",
            "claim_token=raw-token",
            "Authorization:Bearer abc.def.ghi",
            "api_key = secret-key",
            "vault:providers/acme/token",
            "control\0characters",
        ]
        .join(" ");
        let sanitized = sanitize_public_text(&format!("{corpus} {}", "x".repeat(700)));

        assert!(!sanitized.contains("compozy_claim_"));
        assert!(!sanitized.to_lowercase().contains("claim_token"));
        assert!(!sanitized.to_lowercase().contains("bearer"));
        assert!(!sanitized.contains("vault:"));
        assert!(!sanitized.contains('\0'));
        assert!(sanitized.len() <= MAX_SAFE_MESSAGE_BYTES);
    }

    #[test]
    fn should_never_embed_http_response_body_in_public_error() {
        let body = "claim_token=compozy_claim_response-secret";
        let error = ShellError::new(
            ShellErrorCode::RuntimeUnhealthy,
            "The runtime returned an invalid response.",
            PathBuf::from("/tmp/compozy.log"),
        );

        assert!(!error.safe_message.contains(body));
        assert_eq!(
            error.safe_message,
            "The runtime returned an invalid response."
        );
    }
}
