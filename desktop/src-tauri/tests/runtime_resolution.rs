use std::collections::HashMap;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use chrono::{DateTime, Utc};
use compozyos_desktop::errors::ShellErrorCode;
use compozyos_desktop::home::CompozyHome;
use compozyos_desktop::runtime::discovery::{DaemonRecord, ProcessTable};
use compozyos_desktop::runtime::probe::{
    BoundProbe, Compatibility, HttpStatusTransport, handshake,
};
use compozyos_desktop::runtime::resolver::{
    AttachFirstResolver, BinarySource, InstallLocator, Resolution, RuntimeResolver,
};
use semver::VersionReq;

#[derive(Clone)]
enum StubResponse {
    Status { version: String, schema: String },
    Foreign,
    Unhealthy,
}

struct StubServer {
    address: SocketAddr,
    response: Arc<Mutex<StubResponse>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl StubServer {
    fn start(home: PathBuf, pid: u32, started_at: DateTime<Utc>, response: StubResponse) -> Self {
        let listener = TcpListener::bind(("127.0.0.1", 0)).expect("stub listener binds");
        let address = listener.local_addr().expect("stub address resolves");
        let response = Arc::new(Mutex::new(response));
        let worker_response = Arc::clone(&response);
        let stop = Arc::new(AtomicBool::new(false));
        let worker_stop = Arc::clone(&stop);
        let worker = std::thread::spawn(move || {
            for connection in listener.incoming() {
                let mut connection = match connection {
                    Ok(connection) => connection,
                    Err(error) => {
                        eprintln!("accept stub connection: {error}");
                        continue;
                    }
                };
                if worker_stop.load(Ordering::Acquire) {
                    break;
                }
                let mut request = [0_u8; 2048];
                if let Err(error) = connection.read(&mut request) {
                    eprintln!("read stub request: {error}");
                    continue;
                }
                let response = worker_response
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner())
                    .clone();
                let (status, content_type, body) = match response {
                    StubResponse::Status { version, schema } => (
                        "200 OK",
                        "application/json",
                        serde_json::json!({
                            "schema_version": schema,
                            "daemon": {
                                "pid": pid,
                                "started_at": started_at,
                                "http_host": "localhost",
                                "http_port": address.port(),
                                "user_home_dir": home,
                                "version": version
                            }
                        })
                        .to_string(),
                    ),
                    StubResponse::Foreign => {
                        ("200 OK", "text/html", "<html>not compozy</html>".to_owned())
                    }
                    StubResponse::Unhealthy => (
                        "500 Internal Server Error",
                        "application/json",
                        "{\"error\":\"unavailable\"}".to_owned(),
                    ),
                };
                let message = format!(
                    "HTTP/1.1 {status}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                if let Err(error) = connection.write_all(message.as_bytes()) {
                    eprintln!("write stub response: {error}");
                }
            }
        });
        Self {
            address,
            response,
            stop,
            worker: Some(worker),
        }
    }

    fn set_response(&self, response: StubResponse) {
        *self
            .response
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner()) = response;
    }
}

impl Drop for StubServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Err(error) = TcpStream::connect(self.address) {
            eprintln!("wake stub server for shutdown: {error}");
        }
        if let Some(worker) = self.worker.take()
            && let Err(error) = worker.join()
        {
            eprintln!("join stub server: {error:?}");
        }
    }
}

struct FakeProcesses {
    starts: HashMap<u32, DateTime<Utc>>,
}

impl ProcessTable for FakeProcesses {
    fn start_time(&self, pid: u32) -> Option<DateTime<Utc>> {
        self.starts.get(&pid).copied()
    }

    fn executable(&self, pid: u32) -> Option<PathBuf> {
        self.starts
            .contains_key(&pid)
            .then(|| PathBuf::from("/operator/compozy"))
    }

    fn is_descendant(&self, descendant: u32, ancestor: u32) -> bool {
        descendant == ancestor
    }
}

#[derive(Default)]
struct CountingInstalls {
    calls: AtomicUsize,
    operator: Option<PathBuf>,
}

impl InstallLocator for CountingInstalls {
    fn app_owned(&self, _home: &CompozyHome) -> Option<PathBuf> {
        self.calls.fetch_add(1, Ordering::SeqCst);
        None
    }

    fn operator_install(&self, _home: &CompozyHome) -> Option<PathBuf> {
        self.calls.fetch_add(1, Ordering::SeqCst);
        self.operator.clone()
    }

    fn owns_executable(&self, _home: &CompozyHome, _executable: &Path) -> bool {
        false
    }
}

fn runtime_fixture(
    response: StubResponse,
) -> (tempfile::TempDir, CompozyHome, StubServer, FakeProcesses) {
    let directory = tempfile::tempdir().expect("temp home creates");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let started_at = Utc::now();
    let pid = 42;
    let server = StubServer::start(home.root.clone(), pid, started_at, response);
    let record = DaemonRecord {
        pid,
        port: server.address.port(),
        started_at,
    };
    std::fs::write(
        &home.daemon_info,
        serde_json::to_vec(&record).expect("daemon record serializes"),
    )
    .expect("daemon record writes");
    let processes = FakeProcesses {
        starts: HashMap::from([(pid, started_at)]),
    };
    (directory, home, server, processes)
}

fn resolve(
    home: &CompozyHome,
    processes: &FakeProcesses,
    installs: &CountingInstalls,
) -> Resolution {
    let probe = BoundProbe::new(HttpStatusTransport::default(), Duration::from_secs(1));
    AttachFirstResolver {
        processes,
        probe: &probe,
        installs,
    }
    .resolve(home)
}

#[test]
fn should_attach_to_bound_daemon_without_consulting_install_resolvers() {
    let (_directory, home, _server, processes) = runtime_fixture(StubResponse::Status {
        version: "0.3.0".to_owned(),
        schema: "2026-07-16".to_owned(),
    });
    let installs = CountingInstalls::default();
    let Resolution::Attached {
        identity,
        owned: false,
    } = resolve(&home, &processes, &installs)
    else {
        panic!("healthy bound daemon should attach");
    };
    assert_eq!(identity.origin.port(), Some(identity.record.port));
    assert_eq!(installs.calls.load(Ordering::SeqCst), 0);
}

#[test]
fn should_refuse_foreign_port_and_report_version_skew() {
    let (_directory, home, server, processes) = runtime_fixture(StubResponse::Foreign);
    let installs = CountingInstalls::default();
    let Resolution::Failed { error } = resolve(&home, &processes, &installs) else {
        panic!("foreign responder should fail");
    };
    assert_eq!(error.code, ShellErrorCode::PortConflictForeign);

    let minimum = VersionReq::parse(">=0.3.0").expect("requirement parses");
    for (version, schema, newer) in [
        ("0.2.9", "2026-07-16", false),
        ("0.3.0", "2099-01-01", true),
    ] {
        server.set_response(StubResponse::Status {
            version: version.to_owned(),
            schema: schema.to_owned(),
        });
        let Resolution::Attached { identity, .. } = resolve(&home, &processes, &installs) else {
            panic!("bound skewed daemon should attach before handshake");
        };
        let compatibility = handshake(&identity, &minimum);
        if newer {
            assert!(matches!(compatibility, Compatibility::SkewNewer { .. }));
        } else {
            assert!(matches!(compatibility, Compatibility::SkewOlder { .. }));
        }
    }
}

#[test]
fn should_defer_a_live_unhealthy_record_and_ignore_stale_or_other_home_records() {
    let (_directory, home, server, processes) = runtime_fixture(StubResponse::Unhealthy);
    let installs = CountingInstalls::default();
    assert!(matches!(
        resolve(&home, &processes, &installs),
        Resolution::Awaiting(record) if record.pid == 42
    ));
    server.set_response(StubResponse::Status {
        version: "0.3.0".to_owned(),
        schema: "2026-07-16".to_owned(),
    });
    assert!(matches!(
        resolve(&home, &processes, &installs),
        Resolution::Attached { .. }
    ));

    let stale_processes = FakeProcesses {
        starts: HashMap::new(),
    };
    let operator = PathBuf::from("/operator/compozy");
    let installs = CountingInstalls {
        calls: AtomicUsize::new(0),
        operator: Some(operator.clone()),
    };
    assert!(matches!(
        resolve(&home, &stale_processes, &installs),
        Resolution::NeedsStart { binary, source: BinarySource::OperatorPath } if binary == operator
    ));

    let other_directory = tempfile::tempdir().expect("other home creates");
    let other_home = CompozyHome::from_root(other_directory.path().to_path_buf());
    assert!(matches!(
        resolve(&other_home, &processes, &CountingInstalls::default()),
        Resolution::NeedsProvision
    ));
}
