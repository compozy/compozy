# Released v2 decoder fixture

Source: `v0.3.0-beta.24` (`3327649151d9f79d2f5fc2dea437f20e54d53722`).

The Go files are byte-for-byte copies of `internal/marketplace/` at that tag. They retain the strict
decoder and semantic rules, including rejection of an input query parameter already present in the
launch URL. Do not modernize these files alongside the production v3 grammar. Remove this fixture
when root-family publication retires in v0.6.0.

The unchanged decoder uses the repository config environment-name and secure HTTP validators.
Its catalog structs, defaults, launch rules, binding collisions and version checks are frozen here.

Original SHA-256 digests:

- `types.go`: `ac8a3161cb6fd53088561e760cf7b4c525b6e5b849a5e1c06de575401520b47e`
- `source.go`: `d658845d8e6519a61a6e651a43b167fc58647defc32cde26e0e5f01bc0fd00c0`
- `errors.go`: `19d1afe775ffedc99227897a725b25c147541916f93f0a0d2b7ff7a8f811f5b6`
- `document_validation.go`: `842bffa2dc0a9fd9461ea52dca12af004b8b8fe3477efe19c41b18104e845a4d`
- `entry_common.go`: `d053d726f8ab385ca9ecabca42bd8395c0554d1f39d732d5b687ad6575e8ec08`
- `entry_extension.go`: `f4dd3965ddc1452e3b50f9909167cc304a9afc5ec4f61266529ef952a4677516`
- `entry_mcp.go`: `a224cc2349e185c11b6e4a11c346f4b752514aac8466b1022a94293c6cd251cd`
- `entry_mcp_input.go`: `9aedfd80eb2f437ea7b74ff6b405664b8e4eaea6da1fc24e1af282b9346dd071`
- `entry_mcp_launch.go`: `29f2e774ac979e20267300373371b61fd850e53b4d0abe4858eabcc33ee5ff86`
- `entry_skill.go`: `2901aea0919f1f0c1b8d6e2e9fd0076c49ff5bee9c18a95d14ca7f04a7fefdd8`
