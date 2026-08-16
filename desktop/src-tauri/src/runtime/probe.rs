use std::io::Read;
use std::net::IpAddr;
use std::path::PathBuf;
use std::time::Duration;

use chrono::{DateTime, NaiveDate, Utc};
use semver::{Version, VersionReq};
use serde::Deserialize;
use url::Url;

use crate::errors::{ShellError, ShellErrorCode};

use super::PROCESS_START_TOLERANCE_SECONDS;
use super::discovery::DaemonRecord;

pub const MAX_KNOWN_STATUS_SCHEMA: &str = "2026-07-16";
pub const MINIMUM_RUNTIME: &str = ">=0.3.0-beta.8";
const MAX_IDENTITY_BODY_BYTES: usize = 8 * 1024;

#[derive(Clone, Debug, Deserialize, Eq, PartialEq)]
pub struct StatusPayload {
    pub schema_version: String,
    pub daemon: DaemonStatusPayload,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq)]
pub struct DaemonStatusPayload {
    pub pid: u32,
    pub started_at: DateTime<Utc>,
    pub http_host: String,
    pub http_port: u16,
    pub user_home_dir: PathBuf,
    #[serde(default)]
    pub version: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BoundDaemonIdentity {
    pub status: StatusPayload,
    pub record: DaemonRecord,
    pub origin: Url,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Identity {
    Compozy(Box<BoundDaemonIdentity>),
    Foreign,
    Unreachable,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Compatibility {
    Compatible {
        runtime: Version,
    },
    SkewOlder {
        runtime: Version,
        needed: VersionReq,
    },
    SkewNewer {
        runtime: Version,
        needed: VersionReq,
    },
}

#[derive(Debug)]
pub enum TransportResult {
    Ok(Vec<u8>),
    HttpFailure,
    Unreachable,
}

pub trait StatusTransport: Send + Sync {
    fn get_status(&self, origin: &Url, deadline: Duration) -> TransportResult;
}

pub trait IdentityProbe: Send + Sync {
    fn probe(&self, record: &DaemonRecord, expected_child_pid: Option<u32>) -> Identity;
}

pub struct HttpStatusTransport {
    client: Option<reqwest::blocking::Client>,
}

impl Default for HttpStatusTransport {
    fn default() -> Self {
        let client = reqwest::blocking::Client::builder()
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .map_err(|error| crate::logging::error(format!("build status HTTP client: {error}")))
            .ok();
        Self { client }
    }
}

impl StatusTransport for HttpStatusTransport {
    fn get_status(&self, origin: &Url, deadline: Duration) -> TransportResult {
        let Some(client) = &self.client else {
            return TransportResult::Unreachable;
        };
        let Ok(url) = origin.join("/api/status/identity") else {
            return TransportResult::Unreachable;
        };
        match client.get(url).timeout(deadline).send() {
            Ok(response) if response.status().is_success() => {
                let mut bytes = Vec::new();
                let mut limited = response.take((MAX_IDENTITY_BODY_BYTES + 1) as u64);
                match limited.read_to_end(&mut bytes) {
                    Ok(_) if bytes.len() <= MAX_IDENTITY_BODY_BYTES => TransportResult::Ok(bytes),
                    Ok(_) => TransportResult::HttpFailure,
                    Err(error) => {
                        crate::logging::error(format!("read runtime identity: {error}"));
                        TransportResult::Unreachable
                    }
                }
            }
            Ok(_) => TransportResult::HttpFailure,
            Err(error) => {
                crate::logging::error(format!("request runtime identity: {error}"));
                TransportResult::Unreachable
            }
        }
    }
}

pub struct BoundProbe<T> {
    transport: T,
    deadline: Duration,
}

impl<T> BoundProbe<T> {
    pub fn new(transport: T, deadline: Duration) -> Self {
        Self {
            transport,
            deadline,
        }
    }
}

impl<T: StatusTransport> IdentityProbe for BoundProbe<T> {
    fn probe(&self, record: &DaemonRecord, expected_child_pid: Option<u32>) -> Identity {
        let Ok(origin) = Url::parse(&format!("http://localhost:{}", record.port)) else {
            return Identity::Foreign;
        };
        let bytes = match self.transport.get_status(&origin, self.deadline) {
            TransportResult::Ok(bytes) => bytes,
            TransportResult::HttpFailure | TransportResult::Unreachable => {
                return Identity::Unreachable;
            }
        };
        let Ok(status) = serde_json::from_slice::<StatusPayload>(&bytes) else {
            return Identity::Foreign;
        };
        if !identity_agrees(&status, record, expected_child_pid) {
            return Identity::Foreign;
        }
        let Ok(origin) = Url::parse(&format!(
            "http://{}:{}",
            status.daemon.http_host, status.daemon.http_port
        )) else {
            return Identity::Foreign;
        };
        Identity::Compozy(Box::new(BoundDaemonIdentity {
            status,
            record: record.clone(),
            origin,
        }))
    }
}

pub fn handshake(identity: &BoundDaemonIdentity, minimum: &VersionReq) -> Compatibility {
    let runtime = identity
        .status
        .daemon
        .version
        .as_deref()
        .and_then(|raw| Version::parse(raw.trim_start_matches('v')).ok())
        .unwrap_or_else(|| Version::new(0, 0, 0));
    let schema_is_newer = schema_date(&identity.status.schema_version)
        .zip(schema_date(MAX_KNOWN_STATUS_SCHEMA))
        .is_none_or(|(actual, known)| actual > known);
    if schema_is_newer {
        return Compatibility::SkewNewer {
            runtime,
            needed: minimum.clone(),
        };
    }
    if !minimum.matches(&runtime) {
        return Compatibility::SkewOlder {
            runtime,
            needed: minimum.clone(),
        };
    }
    Compatibility::Compatible { runtime }
}

pub fn identity_error(identity: &Identity, log_path: PathBuf) -> Option<ShellError> {
    match identity {
        Identity::Compozy(_) => None,
        Identity::Foreign => Some(ShellError::from_code(
            ShellErrorCode::PortConflictForeign,
            log_path,
        )),
        Identity::Unreachable => Some(ShellError::from_code(
            ShellErrorCode::RuntimeUnhealthy,
            log_path,
        )),
    }
}

fn identity_agrees(
    status: &StatusPayload,
    record: &DaemonRecord,
    expected_child_pid: Option<u32>,
) -> bool {
    let daemon = &status.daemon;
    daemon.pid == record.pid
        && expected_child_pid.is_none_or(|pid| daemon.pid == pid)
        && daemon.http_port == record.port
        && is_loopback_host(&daemon.http_host)
        && (daemon.started_at.timestamp() - record.started_at.timestamp()).abs()
            <= PROCESS_START_TOLERANCE_SECONDS
}

fn is_loopback_host(host: &str) -> bool {
    host.eq_ignore_ascii_case("localhost")
        || host
            .trim_matches(['[', ']'])
            .parse::<IpAddr>()
            .is_ok_and(|address| address.is_loopback())
}

fn schema_date(value: &str) -> Option<NaiveDate> {
    NaiveDate::parse_from_str(value, "%Y-%m-%d").ok()
}

#[cfg(test)]
mod tests {
    use std::io::Write;
    use std::net::TcpListener;
    use std::sync::{Arc, Mutex};

    use super::*;

    #[derive(Clone)]
    struct StubTransport(TransportResultFixture);

    #[derive(Clone)]
    enum TransportResultFixture {
        Body(Vec<u8>),
        HttpFailure,
        Unreachable,
    }

    impl StatusTransport for StubTransport {
        fn get_status(&self, _origin: &Url, _deadline: Duration) -> TransportResult {
            match &self.0 {
                TransportResultFixture::Body(body) => TransportResult::Ok(body.clone()),
                TransportResultFixture::HttpFailure => TransportResult::HttpFailure,
                TransportResultFixture::Unreachable => TransportResult::Unreachable,
            }
        }
    }

    fn fixture_record() -> DaemonRecord {
        DaemonRecord {
            pid: 42,
            port: 2123,
            started_at: DateTime::parse_from_rfc3339("2026-08-10T03:00:00Z")
                .expect("fixture date parses")
                .with_timezone(&Utc),
        }
    }

    fn payload(overrides: serde_json::Value) -> Vec<u8> {
        let mut value = serde_json::json!({
            "schema_version": MAX_KNOWN_STATUS_SCHEMA,
            "daemon": {
                "pid": 42,
                "started_at": "2026-08-10T03:00:00Z",
                "http_host": "localhost",
                "http_port": 2123,
                "user_home_dir": "/tmp/compozy",
                "version": "0.3.0"
            }
        });
        merge(&mut value, overrides);
        serde_json::to_vec(&value).expect("fixture serializes")
    }

    fn merge(target: &mut serde_json::Value, source: serde_json::Value) {
        match (target, source) {
            (serde_json::Value::Object(target), serde_json::Value::Object(source)) => {
                for (key, value) in source {
                    merge(target.entry(key).or_insert(serde_json::Value::Null), value);
                }
            }
            (target, source) => *target = source,
        }
    }

    #[test]
    fn should_bind_real_status_shape_to_discovery_record() {
        let probe = BoundProbe::new(
            StubTransport(TransportResultFixture::Body(payload(serde_json::json!({})))),
            Duration::from_secs(1),
        );
        assert!(matches!(
            probe.probe(&fixture_record(), None),
            Identity::Compozy(_)
        ));
    }

    #[test]
    fn should_treat_user_home_dir_as_operator_metadata() {
        let probe = BoundProbe::new(
            StubTransport(TransportResultFixture::Body(payload(serde_json::json!({
                "daemon": {"user_home_dir": "/Users/operator"}
            })))),
            Duration::from_secs(1),
        );

        assert!(matches!(
            probe.probe(&fixture_record(), None),
            Identity::Compozy(_)
        ));
    }

    #[test]
    fn should_reject_foreign_shape_html_and_copied_payload_mismatches() {
        let cases = [
            b"<html>foreign</html>".to_vec(),
            payload(serde_json::json!({"daemon": {"pid": 99}})),
            payload(serde_json::json!({"daemon": {"started_at": "2026-08-09T03:00:00Z"}})),
            payload(serde_json::json!({"daemon": {"http_port": 9999}})),
        ];
        for body in cases {
            let probe = BoundProbe::new(
                StubTransport(TransportResultFixture::Body(body)),
                Duration::from_secs(1),
            );
            assert_eq!(probe.probe(&fixture_record(), None), Identity::Foreign);
        }
        assert_eq!(
            identity_error(&Identity::Foreign, PathBuf::from("/tmp/compozy.log"))
                .expect("foreign identity maps to an error")
                .code,
            ShellErrorCode::PortConflictForeign
        );
    }

    #[test]
    fn should_require_spawned_child_pid_when_provided() {
        let probe = BoundProbe::new(
            StubTransport(TransportResultFixture::Body(payload(serde_json::json!({})))),
            Duration::from_secs(1),
        );
        assert_eq!(probe.probe(&fixture_record(), Some(99)), Identity::Foreign);
    }

    #[test]
    fn should_classify_transport_failures_as_unreachable() {
        for fixture in [
            TransportResultFixture::HttpFailure,
            TransportResultFixture::Unreachable,
        ] {
            let probe = BoundProbe::new(StubTransport(fixture), Duration::from_millis(10));
            assert_eq!(probe.probe(&fixture_record(), None), Identity::Unreachable);
            assert_eq!(
                identity_error(&Identity::Unreachable, PathBuf::from("/tmp/compozy.log"))
                    .expect("unreachable identity maps to an error")
                    .code,
                ShellErrorCode::RuntimeUnhealthy
            );
        }
    }

    struct DeadlineTransport(Arc<Mutex<Option<Duration>>>);

    impl StatusTransport for DeadlineTransport {
        fn get_status(&self, _origin: &Url, deadline: Duration) -> TransportResult {
            *self
                .0
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner()) = Some(deadline);
            TransportResult::Unreachable
        }
    }

    #[test]
    fn should_apply_the_bounded_request_deadline() {
        let observed = Arc::new(Mutex::new(None));
        let deadline = Duration::from_millis(125);
        let probe = BoundProbe::new(DeadlineTransport(Arc::clone(&observed)), deadline);
        assert_eq!(probe.probe(&fixture_record(), None), Identity::Unreachable);
        assert_eq!(*observed.lock().expect("deadline lock"), Some(deadline));
    }

    #[test]
    fn should_request_the_bounded_runtime_identity_surface() {
        let listener = TcpListener::bind("127.0.0.1:0").expect("test listener binds");
        let address = listener.local_addr().expect("test listener has an address");
        let server = std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("identity request connects");
            let mut request = [0_u8; 1024];
            let length = stream.read(&mut request).expect("identity request reads");
            stream
                .write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}")
                .expect("identity response writes");
            String::from_utf8(request[..length].to_vec()).expect("identity request is UTF-8")
        });

        let origin = Url::parse(&format!("http://{address}")).expect("test origin parses");
        let transport = HttpStatusTransport::default();
        assert!(matches!(
            transport.get_status(&origin, Duration::from_secs(1)),
            TransportResult::Ok(body) if body == b"{}"
        ));
        let request = server.join().expect("identity server joins");
        assert!(
            request.starts_with("GET /api/status/identity HTTP/1.1\r\n"),
            "request line = {request:?}"
        );
    }

    #[test]
    fn should_apply_minimum_version_and_date_schema_handshake() {
        let minimum = VersionReq::parse(MINIMUM_RUNTIME).expect("requirement parses");
        let cases = [
            ("0.3.0-beta.8", MAX_KNOWN_STATUS_SCHEMA, "compatible"),
            ("0.3.0-beta.7", MAX_KNOWN_STATUS_SCHEMA, "older"),
            ("0.3.0", "2099-01-01", "newer"),
        ];
        for (version, schema, expected) in cases {
            let probe = BoundProbe::new(
                StubTransport(TransportResultFixture::Body(payload(serde_json::json!({
                    "schema_version": schema,
                    "daemon": {"version": version}
                })))),
                Duration::from_secs(1),
            );
            let Identity::Compozy(identity) = probe.probe(&fixture_record(), None) else {
                panic!("fixture should bind");
            };
            let actual = match handshake(&identity, &minimum) {
                Compatibility::Compatible { .. } => "compatible",
                Compatibility::SkewOlder { .. } => "older",
                Compatibility::SkewNewer { .. } => "newer",
            };
            assert_eq!(actual, expected);
        }

        let probe = BoundProbe::new(
            StubTransport(TransportResultFixture::Body(payload(serde_json::json!({
                "daemon": {"version": null}
            })))),
            Duration::from_secs(1),
        );
        let Identity::Compozy(identity) = probe.probe(&fixture_record(), None) else {
            panic!("versionless fixture should still bind");
        };
        assert!(matches!(
            handshake(&identity, &minimum),
            Compatibility::SkewOlder { .. }
        ));
    }
}
