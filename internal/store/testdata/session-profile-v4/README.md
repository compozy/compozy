# Previous-release creation witness

`profile.json.golden` and the three identity digests in `meta.json.golden` were generated with
`SessionCreationProfile.CanonicalJSON`, `Ref`, `PolicySpecDigest`, and `CreationDigest`
from tag `v0.3.0-beta.21` (`93a5c04c880c893a4a975accb469b9276e702b0b`),
`internal/store/session_creation_profile.go`. The fixture is synthetic and contains
no operator data. Metadata wraps that exact profile and identity in a stopped
session. Do not regenerate the historical hashes with the current implementation.

The `.golden` extension preserves the exact serialized fixture bytes through formatting.
Original SHA-256 values:

- `profile.json.golden`: `3f6ef5fd5e0608405cdffc56cde94737936edc27b9541f2455b73f35d4aaef54`
- `meta.json.golden`: `884591ce6b9d957c45bbfd4b105cc3e3a0569b0a2bc7439810b60813fd1c683a`
