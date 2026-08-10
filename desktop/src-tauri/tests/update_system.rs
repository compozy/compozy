// Suite: desktop update system
// Invariant: only signed, hash-matching runtime or app artifacts can become installable updates.
// Boundary IN: feed HTTP, archive extraction, filesystem install, provenance, and Tauri updater.
// Boundary OUT: daemon process lifecycle and platform installer execution.

use std::collections::{BTreeMap, HashMap};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;

use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64;
use compozyos_desktop::home::CompozyHome;
use compozyos_desktop::runtime::artifacts::{ArtifactError, current_target};
use compozyos_desktop::runtime::provenance::{self, ProvenanceRecord};
use compozyos_desktop::runtime::provision::{
    ProvisionError, ProvisionOutcome, ProvisionRequest, ResumeChoice, provision,
};
use compozyos_desktop::state::ProvisionStage;
use compozyos_desktop::update::app_update::{
    AppUpdateEvent, AppUpdater, AppUpdaterError, TauriAppUpdateBackend,
};
use minisign::{KeyPair, sign};
use serde_json::json;
use sha2::{Digest, Sha256};
use url::Url;

struct FixtureServer {
    origin: Url,
    manifest_url: Url,
    stop: Arc<AtomicBool>,
    hits: Arc<Mutex<Vec<String>>>,
    thread: Option<JoinHandle<()>>,
}

impl FixtureServer {
    fn start(build_routes: impl FnOnce(&Url) -> HashMap<String, Vec<u8>>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("fixture server binds");
        let address = listener.local_addr().expect("fixture address reads");
        let origin = Url::parse(&format!("http://{address}/")).expect("fixture origin parses");
        let manifest_url = origin.join("runtime.json").expect("manifest URL joins");
        let routes = build_routes(&origin);
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = Arc::clone(&stop);
        let hits = Arc::new(Mutex::new(Vec::new()));
        let thread_hits = Arc::clone(&hits);
        let thread = std::thread::spawn(move || {
            while !thread_stop.load(Ordering::Acquire) {
                let (stream, _) = listener.accept().expect("fixture request accepts");
                if thread_stop.load(Ordering::Acquire) {
                    break;
                }
                serve_route(stream, &routes, &thread_hits);
            }
        });
        Self {
            origin,
            manifest_url,
            stop,
            hits,
            thread: Some(thread),
        }
    }

    fn hits(&self) -> Vec<String> {
        self.hits
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .clone()
    }
}

impl Drop for FixtureServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Err(error) = TcpStream::connect(
            self.origin
                .socket_addrs(|| None)
                .expect("fixture address resolves")[0],
        ) {
            panic!("wake fixture server: {error}");
        }
        if let Some(thread) = self.thread.take() {
            thread.join().expect("fixture server joins");
        }
    }
}

fn serve_route(
    mut stream: TcpStream,
    routes: &HashMap<String, Vec<u8>>,
    hits: &Mutex<Vec<String>>,
) {
    let mut request = [0_u8; 4096];
    let read = stream.read(&mut request).expect("fixture request reads");
    let request = String::from_utf8_lossy(&request[..read]);
    let path = request
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .expect("request path exists")
        .to_owned();
    hits.lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner())
        .push(path.clone());
    let (status, body) = routes
        .get(&path)
        .map_or(("404 Not Found", Vec::new()), |body| {
            ("200 OK", body.clone())
        });
    let response = format!(
        "HTTP/1.1 {status}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    );
    stream
        .write_all(response.as_bytes())
        .expect("fixture headers write");
    stream.write_all(&body).expect("fixture body writes");
}

fn runtime_archive(binary: &[u8]) -> Vec<u8> {
    #[cfg(not(windows))]
    {
        use flate2::Compression;
        use flate2::write::GzEncoder;

        let encoder = GzEncoder::new(Vec::new(), Compression::default());
        let mut archive = tar::Builder::new(encoder);
        let mut header = tar::Header::new_gnu();
        header.set_size(binary.len() as u64);
        header.set_mode(0o755);
        header.set_cksum();
        archive
            .append_data(&mut header, "compozy", binary)
            .expect("runtime archive appends");
        archive
            .into_inner()
            .expect("tar finalizes")
            .finish()
            .expect("gzip finalizes")
    }
    #[cfg(windows)]
    {
        use std::io::Cursor;
        use zip::write::SimpleFileOptions;

        let mut writer = zip::ZipWriter::new(Cursor::new(Vec::new()));
        writer
            .start_file("compozy.exe", SimpleFileOptions::default())
            .expect("runtime archive entry starts");
        writer.write_all(binary).expect("runtime archive writes");
        writer.finish().expect("zip finalizes").into_inner()
    }
}

fn fixture(
    archive: &[u8],
    advertised_sha: Option<String>,
    serve_archive: bool,
) -> (FixtureServer, String) {
    let pair = KeyPair::generate_unencrypted_keypair().expect("test key generates");
    let public_key = BASE64.encode(
        pair.pk
            .to_box()
            .expect("runtime public key boxes")
            .to_string(),
    );
    let target = current_target()
        .expect("test target is supported")
        .to_owned();
    let sha256 = advertised_sha.unwrap_or_else(|| format!("{:x}", Sha256::digest(archive)));
    let archive = archive.to_vec();
    let server = FixtureServer::start(move |origin| {
        let artifact = BTreeMap::from([
            ("sha256".to_owned(), json!(sha256)),
            ("size".to_owned(), json!(archive.len())),
            (
                "url".to_owned(),
                json!(origin.join("runtime-archive").expect("archive URL joins")),
            ),
        ]);
        let manifest = BTreeMap::from([
            (
                "platforms".to_owned(),
                json!(BTreeMap::from([(target, artifact)])),
            ),
            ("schema_heads".to_owned(), json!({"global": 12})),
            ("schema_version".to_owned(), json!(1)),
            ("version".to_owned(), json!("0.4.0")),
        ]);
        let bytes = serde_json::to_vec(&manifest).expect("manifest serializes");
        let signature = sign(
            Some(&pair.pk),
            &pair.sk,
            std::io::Cursor::new(&bytes),
            Some("timestamp:0"),
            Some("fixture signature"),
        )
        .expect("manifest signs")
        .to_string();
        let mut routes = HashMap::from([
            ("/runtime.json".to_owned(), bytes),
            ("/runtime.json.minisig".to_owned(), signature.into_bytes()),
        ]);
        if serve_archive {
            routes.insert("/runtime-archive".to_owned(), archive);
        }
        routes
    });
    (server, public_key)
}

fn app_feed(artifact: &[u8], signed_bytes: &[u8]) -> (FixtureServer, String) {
    let pair = KeyPair::generate_unencrypted_keypair().expect("app test key generates");
    let public_key = BASE64.encode(pair.pk.to_box().expect("app public key boxes").to_string());
    let signature = BASE64.encode(
        sign(
            Some(&pair.pk),
            &pair.sk,
            std::io::Cursor::new(signed_bytes),
            Some("timestamp:0"),
            Some("app fixture signature"),
        )
        .expect("app artifact signs")
        .to_string(),
    );
    let artifact = artifact.to_vec();
    let server = FixtureServer::start(move |origin| {
        let feed = serde_json::to_vec(&json!({
            "version": "0.2.0",
            "url": origin.join("app-update").expect("app update URL joins"),
            "signature": signature,
        }))
        .expect("app feed serializes");
        HashMap::from([
            ("/feed.json".to_owned(), feed),
            ("/app-update".to_owned(), artifact),
        ])
    });
    (server, public_key)
}

fn mock_app_updater(
    endpoint: Url,
    public_key: String,
    intent_path: std::path::PathBuf,
) -> (tauri::App<tauri::test::MockRuntime>, AppUpdater) {
    let mut context = tauri::test::mock_context(tauri::test::noop_assets());
    context.config_mut().plugins.0.insert(
        "updater".to_owned(),
        json!({
            "dangerousInsecureTransportProtocol": true,
            "pubkey": public_key.clone(),
        }),
    );
    let app = tauri::test::mock_builder()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .build(context)
        .expect("mock Tauri app builds");
    let backend = Arc::new(TauriAppUpdateBackend::new(
        app.handle().clone(),
        endpoint,
        intent_path,
    ));
    let updater = AppUpdater::new(backend, public_key, None);
    (app, updater)
}

fn request<'a>(
    home: &'a CompozyHome,
    server: &'a FixtureServer,
    key: &'a str,
) -> ProvisionRequest<'a> {
    ProvisionRequest {
        home,
        manifest_url: &server.manifest_url,
        public_key: key,
        target: current_target().expect("test target is supported"),
        installed: None,
        app_version: "0.3.0",
        channel: "beta",
        resume: ResumeChoice::Continue,
    }
}

#[test]
fn should_provision_signed_runtime_and_write_matching_provenance() {
    let archive = runtime_archive(b"fixture runtime");
    let (server, key) = fixture(&archive, None, true);
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let mut stages = Vec::new();
    let outcome = provision(&request(&home, &server, &key), &|| false, &mut |stage| {
        stages.push(stage)
    })
    .expect("runtime provisions");

    assert_eq!(
        outcome,
        ProvisionOutcome::Installed {
            version: semver::Version::parse("0.4.0").expect("version parses")
        }
    );
    assert_eq!(
        std::fs::read(&home.app_binary).expect("binary reads"),
        b"fixture runtime"
    );
    assert_eq!(
        stages,
        [
            ProvisionStage::Download { pct: 0 },
            ProvisionStage::Verify,
            ProvisionStage::Install,
            ProvisionStage::Start,
        ]
    );
    let marker: ProvenanceRecord =
        serde_json::from_slice(&std::fs::read(&home.provenance).expect("marker reads"))
            .expect("marker decodes");
    assert_eq!(marker.channel, "beta");
    assert!(provenance::owns_executable(&home, &home.app_binary));
}

#[test]
fn should_refuse_tampered_archive_without_touching_binary_or_marker() {
    let archive = runtime_archive(b"tampered runtime");
    let (server, key) = fixture(&archive, Some("0".repeat(64)), true);
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let result = provision(&request(&home, &server, &key), &|| false, &mut |_stage| {});

    assert!(matches!(
        result,
        Err(ProvisionError::Artifact(ArtifactError::DigestMismatch))
    ));
    assert!(!home.app_binary.exists());
    assert!(!home.provenance.exists());
    assert!(!home.root.join("bin/.runtime-download").exists());
}

#[test]
fn should_resume_only_from_verified_download_and_overwrite_orphaned_stage() {
    let archive = runtime_archive(b"verified resumed runtime");
    let (server, key) = fixture(&archive, None, true);
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let bin = home.app_binary.parent().expect("binary has parent");
    std::fs::create_dir_all(bin).expect("bin creates");
    std::fs::write(bin.join(".runtime-download"), &archive).expect("verified download writes");
    std::fs::write(bin.join(".runtime-staged"), b"orphaned stage").expect("orphaned stage writes");

    provision(&request(&home, &server, &key), &|| false, &mut |_stage| {})
        .expect("verified download resumes");
    assert_eq!(
        std::fs::read(&home.app_binary).expect("binary reads"),
        b"verified resumed runtime"
    );
    assert!(!server.hits().contains(&"/runtime-archive".to_owned()));
}

#[test]
fn should_abort_install_when_external_runtime_appears_after_staging() {
    let archive = runtime_archive(b"fixture runtime");
    let (server, key) = fixture(&archive, None, true);
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());
    let probes = AtomicUsize::new(0);
    let outcome = provision(
        &request(&home, &server, &key),
        &|| probes.fetch_add(1, Ordering::AcqRel) > 0,
        &mut |_stage| {},
    )
    .expect("external runtime aborts install");
    assert_eq!(outcome, ProvisionOutcome::AttachExternally);
    assert!(!home.app_binary.exists());
    assert!(!home.provenance.exists());
}

#[test]
fn should_retry_transient_runtime_download_without_partial_installation() {
    let archive = runtime_archive(b"runtime after retry");
    let (offline_server, offline_key) = fixture(&archive, None, false);
    let directory = tempfile::tempdir().expect("temp directory opens");
    let home = CompozyHome::from_root(directory.path().to_path_buf());

    assert!(matches!(
        provision(
            &request(&home, &offline_server, &offline_key),
            &|| false,
            &mut |_stage| {},
        ),
        Err(ProvisionError::Artifact(ArtifactError::Network))
    ));
    assert!(!home.app_binary.exists());
    assert!(!home.provenance.exists());
    assert!(!home.root.join("bin/.runtime-download").exists());

    let (recovered_server, recovered_key) = fixture(&archive, None, true);
    assert!(matches!(
        provision(
            &request(&home, &recovered_server, &recovered_key),
            &|| false,
            &mut |_stage| {},
        ),
        Ok(ProvisionOutcome::Installed { .. })
    ));
    assert_eq!(
        std::fs::read(&home.app_binary).expect("retried binary reads"),
        b"runtime after retry"
    );
}

#[test]
fn should_drive_real_tauri_feed_and_verified_download_to_ready() {
    let artifact = b"signed app update";
    let (server, key) = app_feed(artifact, artifact);
    let directory = tempfile::tempdir().expect("temp directory opens");
    let (_app, updater) = mock_app_updater(
        server.origin.join("feed.json").expect("feed URL joins"),
        key,
        directory.path().join("app-update-intent.json"),
    );

    let event = updater.check(true);
    assert!(
        matches!(
            &event,
            AppUpdateEvent::Ready { version, .. } if version == "0.2.0"
        ),
        "update event = {event:?}, hits = {:?}, want ready",
        server.hits()
    );
    assert_eq!(server.hits(), ["/feed.json", "/app-update"]);
}

#[test]
fn should_refuse_tampered_tauri_artifact_without_pending_update() {
    let (server, key) = app_feed(b"tampered app update", b"signed app update");
    let directory = tempfile::tempdir().expect("temp directory opens");
    let (_app, updater) = mock_app_updater(
        server.origin.join("feed.json").expect("feed URL joins"),
        key,
        directory.path().join("app-update-intent.json"),
    );

    let event = updater.check(true);
    assert!(
        matches!(
            &event,
            AppUpdateEvent::Failed { message, .. }
                if message == &AppUpdaterError::InvalidSignature.to_string()
        ),
        "update event = {event:?}, hits = {:?}, want invalid signature",
        server.hits()
    );
    assert!(matches!(
        updater.apply(true),
        Err(AppUpdaterError::NotReady)
    ));
}

#[test]
fn should_keep_malformed_and_missing_tauri_feeds_distinct_from_up_to_date() {
    for (routes, expected) in [
        (
            HashMap::from([("/feed.json".to_owned(), b"not json".to_vec())]),
            AppUpdaterError::FeedInvalid.to_string(),
        ),
        (HashMap::new(), AppUpdaterError::FeedNotFound.to_string()),
    ] {
        let server = FixtureServer::start(move |_origin| routes);
        let directory = tempfile::tempdir().expect("temp directory opens");
        let pair = KeyPair::generate_unencrypted_keypair().expect("app test key generates");
        let (_app, updater) = mock_app_updater(
            server.origin.join("feed.json").expect("feed URL joins"),
            BASE64.encode(pair.pk.to_box().expect("app public key boxes").to_string()),
            directory.path().join("app-update-intent.json"),
        );

        match updater.check(true) {
            AppUpdateEvent::Failed { message, .. } => assert_eq!(message, expected),
            event => panic!("feed failure produced {event:?}, want failed"),
        }
    }
}
