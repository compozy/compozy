fn main() {
    if std::env::var("PROFILE").is_ok_and(|profile| profile != "release") {
        const DEVELOPMENT_CONFIG: &str =
            r#"{"plugins":{"deep-link":{"desktop":{"schemes":["compozyos","compozyos-dev"]}}}}"#;
        // SAFETY: Cargo executes this build script as a single-threaded process before Tauri reads
        // its configuration, so no other thread can observe a partially changed environment.
        unsafe {
            std::env::set_var("TAURI_CONFIG", DEVELOPMENT_CONFIG);
        }
        println!("cargo:rustc-env=TAURI_CONFIG={DEVELOPMENT_CONFIG}");
    }
    tauri_build::build();
}
