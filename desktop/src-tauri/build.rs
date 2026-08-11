mod release_build_contract;

fn main() {
    println!("cargo:rerun-if-env-changed=COMPOZY_RELEASE_CHANNEL");
    println!("cargo:rerun-if-env-changed=TAURI_CONFIG");
    println!("cargo:rerun-if-env-changed=TAURI_SIGNING_PRIVATE_KEY");
    println!("cargo:rerun-if-env-changed=TAURI_SIGNING_PRIVATE_KEY_PASSWORD");

    if std::env::var("PROFILE").is_ok_and(|profile| profile != "release") {
        const DEVELOPMENT_CONFIG: &str =
            r#"{"plugins":{"deep-link":{"desktop":{"schemes":["compozyos","compozyos-dev"]}}}}"#;
        // SAFETY: Cargo executes this build script as a single-threaded process before Tauri reads
        // its configuration, so no other thread can observe a partially changed environment.
        unsafe {
            std::env::set_var("TAURI_CONFIG", DEVELOPMENT_CONFIG);
        }
        println!("cargo:rustc-env=TAURI_CONFIG={DEVELOPMENT_CONFIG}");
    } else {
        let channel = release_build_contract::validate_release_build(
            std::env::var("COMPOZY_RELEASE_CHANNEL").ok().as_deref(),
            std::env::var("TAURI_CONFIG").ok().as_deref(),
            std::env::var("TAURI_SIGNING_PRIVATE_KEY").ok().as_deref(),
            std::env::var("TAURI_SIGNING_PRIVATE_KEY_PASSWORD")
                .ok()
                .as_deref(),
        )
        .unwrap_or_else(|error| panic!("desktop release build configuration is invalid: {error}"));
        println!("cargo:rustc-env=COMPOZY_RELEASE_CHANNEL={channel}");
    }
    tauri_build::try_build(
        tauri_build::Attributes::new()
            .app_manifest(tauri_build::AppManifest::new().commands(&["shell_control"])),
    )
    .unwrap_or_else(|error| panic!("build desktop application: {error}"));
}
