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
        r#"(?i)(api[_-]?key|authorization|(?:[a-z0-9]+[_-])*token|(?:[a-z0-9]+[_-])*secret|oauth[_-]?code|authorization[_-]?code|code(?:[_-]?(?:verifier|challenge))?|pkce[_-]?verifier|secret[_-]?(?:binding|ref)|password|passphrase|credential|private[_-]?key)\s*[:=]\s*[^\s,;\"']+"#,
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
    truncate_utf8(&redact_public_text(input), MAX_SAFE_MESSAGE_BYTES)
}

pub fn redact_public_text(input: &str) -> String {
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
    let redacted = BEARER.replace_all(&redacted, REDACTED);
    let redacted = NAMED_SECRET.replace_all(&redacted, REDACTED);
    let redacted = VAULT_REF.replace_all(&redacted, REDACTED);
    redacted.trim().to_owned()
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
            "refresh_token=refresh-secret",
            "client_secret: client-secret",
            "oauth_code=oauth-secret",
            "authorization_code=authorization-secret",
            "code=oauth-code-secret",
            "code_verifier=pkce-secret",
            "code_challenge=pkce-challenge",
            "pkce_verifier=verifier-secret",
            "mcp_token=mcp-secret",
            "private_key=private-secret",
            "password=password-secret",
            "passphrase=phrase-secret",
            "vault:providers/acme/token",
            "control\0characters",
        ]
        .join(" ");
        let sanitized = sanitize_public_text(&format!("{corpus} {}", "x".repeat(700)));

        assert!(!sanitized.contains("compozy_claim_"));
        assert!(!sanitized.to_lowercase().contains("claim_token"));
        for key in [
            "refresh_token",
            "client_secret",
            "oauth_code",
            "authorization_code",
            "code=",
            "code_verifier",
            "code_challenge",
            "pkce_verifier",
            "mcp_token",
            "private_key",
            "password",
            "passphrase",
        ] {
            assert!(!sanitized.to_lowercase().contains(key), "key: {key}");
        }
        assert!(!sanitized.to_lowercase().contains("bearer"));
        assert!(!sanitized.contains("vault:"));
        assert!(!sanitized.contains('\0'));
        assert!(sanitized.len() <= MAX_SAFE_MESSAGE_BYTES);
    }

    #[test]
    fn should_preserve_long_redacted_logs_beyond_the_public_message_limit() {
        let input = format!("{} refresh_token=refresh-secret", "x".repeat(700));
        let redacted = redact_public_text(&input);

        assert!(redacted.len() > MAX_SAFE_MESSAGE_BYTES);
        assert!(!redacted.contains("refresh-secret"));
        assert!(sanitize_public_text(&input).len() <= MAX_SAFE_MESSAGE_BYTES);
    }

    #[test]
    fn should_redact_secrets_from_untrusted_response_detail() {
        let body = "claim_token=compozy_claim_response-secret";
        let error = ShellError::new(
            ShellErrorCode::RuntimeUnhealthy,
            format!("The runtime returned an invalid response: {body}"),
            PathBuf::from("/tmp/compozy.log"),
        );

        assert!(!error.safe_message.contains(body));
        assert!(!error.safe_message.contains("compozy_claim_"));
        assert!(
            error
                .safe_message
                .starts_with("The runtime returned an invalid response:")
        );
    }
}
