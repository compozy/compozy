use std::fs;
use std::io::{BufReader, ErrorKind, Read, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use serde_json::Value;
use socket2::{Domain, SockAddr, Socket, Type};
use thiserror::Error;

use crate::errors::sanitize_public_text;

pub const CONTROL_SCHEMA_VERSION: u8 = 1;
const MAX_REQUEST_BYTES: u64 = 64 * 1024;

#[derive(Debug, Deserialize)]
struct ControlRequest {
    schema_version: u8,
    id: u64,
    method: String,
    #[serde(default)]
    params: Value,
}

#[derive(Debug, Deserialize, Serialize)]
struct ControlResponse {
    schema_version: u8,
    id: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<ControlError>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ControlError {
    pub code: String,
    pub message: String,
}

impl ControlError {
    pub fn new(code: impl Into<String>, message: impl AsRef<str>) -> Self {
        Self {
            code: code.into(),
            message: sanitize_public_text(message.as_ref()),
        }
    }
}

pub trait ControlHandler: Send + Sync + 'static {
    fn handle(&self, method: &str, params: Value) -> Result<Value, ControlError>;
}

#[derive(Debug, Error)]
pub enum ControlServerError {
    #[error("another app control server is already listening")]
    AlreadyListening,
    #[error("app control socket could not be created")]
    Io(#[source] std::io::Error),
}

pub struct ControlServer {
    path: PathBuf,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl ControlServer {
    pub fn start(
        path: &Path,
        handler: Arc<dyn ControlHandler>,
    ) -> Result<Self, ControlServerError> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(ControlServerError::Io)?;
        }
        let address = SockAddr::unix(path).map_err(ControlServerError::Io)?;
        if socket_is_live(&address) {
            return Err(ControlServerError::AlreadyListening);
        }
        remove_socket(path)?;
        let listener =
            Socket::new(Domain::UNIX, Type::STREAM, None).map_err(ControlServerError::Io)?;
        listener.bind(&address).map_err(ControlServerError::Io)?;
        listener.listen(32).map_err(ControlServerError::Io)?;
        listener
            .set_nonblocking(true)
            .map_err(ControlServerError::Io)?;
        set_socket_permissions(path)?;
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = Arc::clone(&stop);
        let thread_path = path.to_path_buf();
        let thread = thread::spawn(move || {
            while !thread_stop.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((connection, _)) => {
                        let request_handler = Arc::clone(&handler);
                        thread::spawn(move || {
                            if let Err(error) =
                                serve_connection(&connection, request_handler.as_ref())
                            {
                                crate::logging::error(format!(
                                    "serve app control request: {error}"
                                ));
                            }
                        });
                    }
                    Err(error) if error.kind() == ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(20));
                    }
                    Err(error) => {
                        crate::logging::error(format!("accept app control connection: {error}"));
                        break;
                    }
                }
            }
            if let Err(error) = fs::remove_file(&thread_path)
                && error.kind() != ErrorKind::NotFound
            {
                crate::logging::error(format!(
                    "remove app control socket {}: {error}",
                    thread_path.display()
                ));
            }
        });
        Ok(Self {
            path: path.to_path_buf(),
            stop,
            thread: Some(thread),
        })
    }

    pub fn path(&self) -> &Path {
        &self.path
    }
}

impl Drop for ControlServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take()
            && let Err(error) = thread.join()
        {
            crate::logging::error(format!("join app control server: {error:?}"));
        }
    }
}

fn serve_connection(socket: &Socket, handler: &dyn ControlHandler) -> std::io::Result<()> {
    socket.set_read_timeout(Some(Duration::from_secs(2)))?;
    socket.set_write_timeout(Some(Duration::from_secs(2)))?;
    let mut limited = BufReader::new(socket).take(MAX_REQUEST_BYTES);
    let decoded = serde_json::from_reader::<_, ControlRequest>(&mut limited);
    let response = match decoded {
        Ok(request) if request.schema_version == CONTROL_SCHEMA_VERSION => {
            match handler.handle(request.method.trim(), request.params) {
                Ok(result) => success(request.id, result),
                Err(error) => failure(request.id, error),
            }
        }
        Ok(request) => failure(
            request.id,
            ControlError::new(
                "app_control_incompatible",
                "The app control protocol version is not supported.",
            ),
        ),
        Err(_) => failure(
            0,
            ControlError::new(
                "app_control_invalid_request",
                "The app control request is invalid.",
            ),
        ),
    };
    let mut writer = socket;
    serde_json::to_writer(&mut writer, &response).map_err(std::io::Error::other)?;
    writer.write_all(b"\n")?;
    writer.flush()
}

fn success(id: u64, result: Value) -> ControlResponse {
    ControlResponse {
        schema_version: CONTROL_SCHEMA_VERSION,
        id,
        result: Some(result),
        error: None,
    }
}

fn failure(id: u64, error: ControlError) -> ControlResponse {
    ControlResponse {
        schema_version: CONTROL_SCHEMA_VERSION,
        id,
        result: None,
        error: Some(error),
    }
}

fn socket_is_live(address: &SockAddr) -> bool {
    let Ok(socket) = Socket::new(Domain::UNIX, Type::STREAM, None) else {
        return false;
    };
    socket.connect(address).is_ok()
}

fn remove_socket(path: &Path) -> Result<(), ControlServerError> {
    match fs::remove_file(path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == ErrorKind::NotFound => Ok(()),
        Err(error) => Err(ControlServerError::Io(error)),
    }
}

#[cfg(unix)]
fn set_socket_permissions(path: &Path) -> Result<(), ControlServerError> {
    use std::os::unix::fs::PermissionsExt;

    fs::set_permissions(path, fs::Permissions::from_mode(0o600)).map_err(ControlServerError::Io)
}

#[cfg(not(unix))]
fn set_socket_permissions(_path: &Path) -> Result<(), ControlServerError> {
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::io::{Read, Write};
    use std::net::Shutdown;
    use std::sync::{Mutex, mpsc};

    use serde_json::json;

    use super::*;

    struct EchoHandler;

    impl ControlHandler for EchoHandler {
        fn handle(&self, method: &str, params: Value) -> Result<Value, ControlError> {
            match method {
                "navigate" | "update.check" | "update.apply" | "retry" | "diagnose" => {
                    Ok(json!({"method": method, "params": params}))
                }
                _ => Err(ControlError::new(
                    "app_control_unknown_method",
                    "The app control method is not supported.",
                )),
            }
        }
    }

    struct ConcurrentHandler {
        slow_entered: mpsc::Sender<()>,
        slow_release: Mutex<mpsc::Receiver<()>>,
    }

    impl ControlHandler for ConcurrentHandler {
        fn handle(&self, method: &str, _params: Value) -> Result<Value, ControlError> {
            if method == "update.check" {
                self.slow_entered.send(()).expect("slow request signals");
                self.slow_release
                    .lock()
                    .unwrap_or_else(|poisoned| poisoned.into_inner())
                    .recv()
                    .expect("slow request releases");
            }
            Ok(json!({"method": method}))
        }
    }

    fn request(path: &Path, schema_version: u8, method: &str, params: Value) -> ControlResponse {
        let socket = Socket::new(Domain::UNIX, Type::STREAM, None).expect("client socket creates");
        socket
            .connect(&SockAddr::unix(path).expect("socket address creates"))
            .expect("client connects");
        let payload = json!({
            "schema_version": schema_version,
            "id": 41,
            "method": method,
            "params": params,
        });
        serde_json::to_writer(&socket, &payload).expect("request serializes");
        (&socket).write_all(b"\n").expect("request terminates");
        socket
            .shutdown(Shutdown::Write)
            .expect("request half closes");
        let mut bytes = Vec::new();
        (&socket).read_to_end(&mut bytes).expect("response reads");
        serde_json::from_slice(&bytes).expect("response decodes")
    }

    #[test]
    fn should_round_trip_every_versioned_control_primitive() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("app.sock");
        let _server =
            ControlServer::start(&path, Arc::new(EchoHandler)).expect("control server starts");
        for (method, params) in [
            ("navigate", json!({"path": "/sessions"})),
            ("update.check", json!({})),
            ("update.apply", json!({"target": "app"})),
            ("update.apply", json!({"target": "runtime"})),
            ("retry", json!({})),
            ("diagnose", json!({})),
        ] {
            let response = request(&path, CONTROL_SCHEMA_VERSION, method, params.clone());
            assert_eq!(response.schema_version, CONTROL_SCHEMA_VERSION);
            assert_eq!(response.id, 41);
            assert_eq!(
                response.result,
                Some(json!({"method": method, "params": params}))
            );
            assert!(response.error.is_none());
        }
    }

    #[test]
    fn should_return_deterministic_errors_for_unknown_method_and_protocol() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("app.sock");
        let _server =
            ControlServer::start(&path, Arc::new(EchoHandler)).expect("control server starts");

        let unknown = request(&path, CONTROL_SCHEMA_VERSION, "unknown", json!({}));
        assert_eq!(
            unknown.error.expect("unknown method fails").code,
            "app_control_unknown_method"
        );
        let incompatible = request(&path, CONTROL_SCHEMA_VERSION + 1, "retry", json!({}));
        assert_eq!(
            incompatible.error.expect("unknown protocol fails").code,
            "app_control_incompatible"
        );
    }

    #[test]
    fn should_serve_independent_connections_while_an_update_check_is_slow() {
        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("app.sock");
        let (entered_tx, entered_rx) = mpsc::channel();
        let (release_tx, release_rx) = mpsc::channel();
        let _server = ControlServer::start(
            &path,
            Arc::new(ConcurrentHandler {
                slow_entered: entered_tx,
                slow_release: Mutex::new(release_rx),
            }),
        )
        .expect("control server starts");

        let slow_path = path.clone();
        let slow = std::thread::spawn(move || {
            request(
                &slow_path,
                CONTROL_SCHEMA_VERSION,
                "update.check",
                json!({}),
            )
        });
        entered_rx
            .recv_timeout(Duration::from_secs(5))
            .expect("slow request enters handler");

        let fast_path = path.clone();
        let (fast_tx, fast_rx) = mpsc::channel();
        let fast = std::thread::spawn(move || {
            let response = request(&fast_path, CONTROL_SCHEMA_VERSION, "diagnose", json!({}));
            fast_tx.send(response).expect("fast response sends");
        });
        let fast_response = fast_rx.recv_timeout(Duration::from_secs(1));
        release_tx.send(()).expect("slow request releases");
        let slow_response = slow.join().expect("slow request joins");
        fast.join().expect("fast request joins");

        let fast_response = fast_response.expect("diagnose responds before update check completes");
        assert_eq!(fast_response.result, Some(json!({"method": "diagnose"})));
        assert_eq!(
            slow_response.result,
            Some(json!({"method": "update.check"}))
        );
    }

    #[cfg(unix)]
    #[test]
    fn should_create_control_socket_with_owner_only_permissions() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().expect("temp directory opens");
        let path = directory.path().join("app.sock");
        let _server =
            ControlServer::start(&path, Arc::new(EchoHandler)).expect("control server starts");
        let mode = fs::metadata(&path)
            .expect("socket metadata reads")
            .permissions()
            .mode()
            & 0o777;
        assert_eq!(mode, 0o600);
    }
}
