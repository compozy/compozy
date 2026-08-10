use url::Url;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum NavigationDecision {
    Allow,
    OpenExternal,
}

pub fn classify(target: &Url, daemon_origin: &Url) -> NavigationDecision {
    if same_origin(target, daemon_origin) || bundled_origin(target) {
        NavigationDecision::Allow
    } else {
        NavigationDecision::OpenExternal
    }
}

pub fn same_origin(left: &Url, right: &Url) -> bool {
    left.scheme() == right.scheme()
        && left.host_str() == right.host_str()
        && left.port_or_known_default() == right.port_or_known_default()
}

fn bundled_origin(url: &Url) -> bool {
    url.scheme() == "tauri"
        || ((url.scheme() == "http" || url.scheme() == "https")
            && url.host_str() == Some("tauri.localhost"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn daemon() -> Url {
        Url::parse("http://localhost:2123").expect("fixture URL is valid")
    }

    #[test]
    fn should_allow_only_daemon_and_bundled_origins() {
        for input in [
            "http://localhost:2123/sessions/abc",
            "tauri://localhost/boot.html",
            "http://tauri.localhost/boot.html",
        ] {
            let target = Url::parse(input).expect("fixture URL is valid");
            assert_eq!(classify(&target, &daemon()), NavigationDecision::Allow);
        }
    }

    #[test]
    fn should_handoff_every_other_origin() {
        for input in [
            "http://localhost:9999/sessions/abc",
            "https://compozy.com/docs",
            "http://127.0.0.1:2123/",
        ] {
            let target = Url::parse(input).expect("fixture URL is valid");
            assert_eq!(
                classify(&target, &daemon()),
                NavigationDecision::OpenExternal,
                "input: {input}"
            );
        }
    }
}
