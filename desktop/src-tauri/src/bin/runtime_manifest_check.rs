use std::env;
use std::ffi::OsString;
use std::fs;
use std::path::PathBuf;

use compozyos_desktop::runtime::artifacts::verify_manifest;
use semver::Version;

fn main() {
    if let Err(error) = run(env::args_os(), env::var("TAURI_SIGNING_PUBLIC_KEY")) {
        eprintln!("desktop release: runtime manifest verification failed: {error}");
        std::process::exit(1);
    }
}

fn run(
    mut args: impl Iterator<Item = OsString>,
    public_key: Result<String, env::VarError>,
) -> Result<(), String> {
    let program = args
        .next()
        .and_then(|value| value.into_string().ok())
        .unwrap_or_else(|| "compozy-runtime-manifest-check".to_owned());
    let manifest_path = required_path(&mut args, &program, "manifest")?;
    let signature_path = required_path(&mut args, &program, "signature")?;
    let expected_version = required_text(&mut args, &program, "expected version")?;
    let targets = args
        .map(|target| {
            target
                .into_string()
                .map_err(|_| "target must be valid UTF-8".to_owned())
        })
        .collect::<Result<Vec<_>, _>>()?;
    if targets.is_empty() {
        return Err(usage(&program));
    }

    let expected_version = Version::parse(&expected_version)
        .map_err(|error| format!("parse expected version: {error}"))?;
    let public_key = public_key.map_err(|_| "TAURI_SIGNING_PUBLIC_KEY is required".to_owned())?;
    let manifest = fs::read(&manifest_path)
        .map_err(|error| format!("read {}: {error}", manifest_path.display()))?;
    let signature = fs::read_to_string(&signature_path)
        .map_err(|error| format!("read {}: {error}", signature_path.display()))?;

    for target in &targets {
        let verified = verify_manifest(&manifest, &signature, &public_key, target, None)
            .map_err(|error| format!("verify {target}: {error}"))?
            .ok_or_else(|| {
                format!("verify {target}: manifest unexpectedly selected no artifact")
            })?;
        if verified.version != expected_version {
            return Err(format!(
                "verify {target}: manifest version {} does not match {expected_version}",
                verified.version
            ));
        }
    }

    println!(
        "desktop release: runtime manifest verified for {} target(s)",
        targets.len()
    );
    Ok(())
}

fn required_path(
    args: &mut impl Iterator<Item = OsString>,
    program: &str,
    name: &str,
) -> Result<PathBuf, String> {
    args.next()
        .map(PathBuf::from)
        .ok_or_else(|| format!("{name} path is required\n{}", usage(program)))
}

fn required_text(
    args: &mut impl Iterator<Item = OsString>,
    program: &str,
    name: &str,
) -> Result<String, String> {
    args.next()
        .ok_or_else(|| format!("{name} is required\n{}", usage(program)))?
        .into_string()
        .map_err(|_| format!("{name} must be valid UTF-8"))
}

fn usage(program: &str) -> String {
    format!(
        "usage: {program} <runtime.json> <runtime.json.minisig> <version> <target> [<target> ...]"
    )
}

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;
    use std::io::Cursor;

    use base64::Engine;
    use base64::engine::general_purpose::STANDARD as BASE64;
    use minisign::{KeyPair, sign};
    use serde_json::json;

    use super::*;

    #[test]
    fn should_verify_signed_canonical_manifest_for_requested_target() {
        let temp = tempfile::tempdir().expect("temporary directory creates");
        let manifest_path = temp.path().join("runtime.json");
        let signature_path = temp.path().join("runtime.json.minisig");
        let manifest = BTreeMap::from([
            (
                "platforms",
                json!({
                    "darwin-aarch64": {
                        "sha256": "a".repeat(64),
                        "size": 3,
                        "url": "http://127.0.0.1:43119/runtime.tar.gz"
                    }
                }),
            ),
            ("schema_heads", json!({"global": 12})),
            ("schema_version", json!(1)),
            ("version", json!("0.4.0")),
        ]);
        let bytes = serde_json::to_vec(&manifest).expect("manifest serializes");
        let pair = KeyPair::generate_unencrypted_keypair().expect("test key generates");
        let signature = sign(
            Some(&pair.pk),
            &pair.sk,
            Cursor::new(&bytes),
            Some("timestamp:0"),
            Some("test signature"),
        )
        .expect("manifest signs")
        .to_string();
        fs::write(&manifest_path, bytes).expect("manifest writes");
        fs::write(&signature_path, signature).expect("signature writes");
        let public_key = BASE64.encode(pair.pk.to_box().expect("public key boxes").to_string());
        let args = [
            OsString::from("checker"),
            manifest_path.into_os_string(),
            signature_path.into_os_string(),
            OsString::from("0.4.0"),
            OsString::from("darwin-aarch64"),
        ];

        run(args.into_iter(), Ok(public_key)).expect("candidate manifest verifies");
    }
}
