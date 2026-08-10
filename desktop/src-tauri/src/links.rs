use std::sync::Mutex;

use percent_encoding::percent_decode_str;
use serde::{Deserialize, Serialize};
use url::Url;

const RELEASE_SCHEME: &str = "compozyos";
const DEVELOPMENT_SCHEME: &str = "compozyos-dev";

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum LinkTarget {
    ProductPath(String),
    DefaultView,
}

#[derive(Debug, Default)]
pub struct LinkQueue {
    pending: Mutex<Option<String>>,
}

impl LinkQueue {
    pub fn push(&self, link: &str) -> LinkTarget {
        let target = parse(link);
        let mut pending = self
            .pending
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        *pending = match &target {
            LinkTarget::ProductPath(path) => Some(path.clone()),
            LinkTarget::DefaultView => None,
        };
        target
    }

    pub fn take(&self) -> Option<String> {
        self.pending
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
            .take()
    }

    pub fn push_argv(&self, arguments: &[String]) -> Option<LinkTarget> {
        let link = arguments.iter().rev().find(|argument| {
            argument.starts_with("compozyos://") || argument.starts_with("compozyos-dev://")
        })?;
        Some(self.push(link))
    }
}

pub fn parse(raw: &str) -> LinkTarget {
    let Ok(url) = Url::parse(raw) else {
        return LinkTarget::DefaultView;
    };
    if url.scheme() != RELEASE_SCHEME && url.scheme() != DEVELOPMENT_SCHEME {
        return LinkTarget::DefaultView;
    }
    if url.host_str() != Some("open")
        || url.username() != ""
        || url.password().is_some()
        || url.fragment().is_some()
    {
        return LinkTarget::DefaultView;
    }
    let Some((_, raw_target)) = raw.split_once("://") else {
        return LinkTarget::DefaultView;
    };
    let raw_path_and_query = raw_target.split('#').next().unwrap_or(raw_target);
    let raw_path = raw_path_and_query
        .split('?')
        .next()
        .unwrap_or(raw_path_and_query);
    let Some(raw_path) = raw_path.strip_prefix("open") else {
        return LinkTarget::DefaultView;
    };
    let Ok(decoded_path) = percent_decode_str(raw_path).decode_utf8() else {
        return LinkTarget::DefaultView;
    };
    if !valid_product_path(&decoded_path) {
        return LinkTarget::DefaultView;
    }
    let mut path = decoded_path.into_owned();
    if let Some(query) = url.query() {
        path.push('?');
        path.push_str(query);
    }
    LinkTarget::ProductPath(path)
}

fn valid_product_path(path: &str) -> bool {
    if !path.starts_with('/')
        || path.starts_with("//")
        || path.contains("://")
        || path.contains('\\')
    {
        return false;
    }
    !path
        .split('/')
        .any(|segment| segment == "." || segment == ".." || segment.contains('\0'))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn should_parse_valid_product_path_with_query() {
        assert_eq!(
            parse("compozyos://open/sessions/abc?tab=logs"),
            LinkTarget::ProductPath("/sessions/abc?tab=logs".to_owned())
        );
    }

    #[test]
    fn should_reject_embedded_authority_scheme_and_traversal() {
        for input in [
            "compozyos://open/http://evil.com",
            "compozyos://open//evil.com/x",
            "compozyos://open/../etc",
            "compozyos://open/%2e%2e/etc",
            "compozyos://open/..\\..\\etc",
            "compozyos://open/%2e%2e%5cetc",
            "compozyos://open/sessions%5c..%5cadmin",
            "https://example.com/sessions/abc",
        ] {
            assert_eq!(parse(input), LinkTarget::DefaultView, "input: {input}");
        }
    }

    #[test]
    fn should_keep_only_the_latest_valid_link() {
        let queue = LinkQueue::default();
        queue.push("compozyos://open/sessions/first");
        queue.push("compozyos://open/sessions/last");
        assert_eq!(queue.take(), Some("/sessions/last".to_owned()));
        assert_eq!(queue.take(), None);
    }

    #[test]
    fn should_forward_only_the_last_link_argument() {
        let arguments = vec![
            "compozyos-desktop".to_owned(),
            "compozyos://open/tasks/first".to_owned(),
            "--ignored".to_owned(),
            "compozyos://open/tasks/last".to_owned(),
        ];
        let queue = LinkQueue::default();
        assert_eq!(
            queue.push_argv(&arguments),
            Some(LinkTarget::ProductPath("/tasks/last".to_owned()))
        );
        assert_eq!(queue.take(), Some("/tasks/last".to_owned()));
    }

    #[test]
    fn should_retain_a_link_until_product_consumes_it_once() {
        let queue = LinkQueue::default();
        queue.push("compozyos://open/sessions/queued");

        assert_eq!(queue.take(), Some("/sessions/queued".to_owned()));
        assert_eq!(queue.take(), None);
    }
}
