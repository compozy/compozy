# Rotate and retire cloud signing keys

Rotate the Ed25519 signing key separately in staging and production every 90 days. `COMPOZY_CLOUD_SIGNING_KEYS` is a JSON object with `kid`, `seed` and `overlap`. The seed is a base64url Ed25519 seed. Each overlap item is a public JWK.

1. In a restricted operator shell, use `umask 077` and write `compozy-cloud keys generate <new-kid>` to a private file. Use `compozy-cloud keys jwk` with that configuration to export the new public JWK.
2. Deploy the current signer with the new public JWK in `overlap`. Verify the JWKS contains both IDs. Then deploy the new signer with the old public key in `overlap`. Both replicas can now verify either key during rollout.
3. After both replicas are on the new signer, run `compozy-cloud keys publish` against the environment's direct endpoint. Confirm a connected computer receives the complete set. Keep the overlap for at least the maximum lifetime of the previously signed credentials and leases plus clock skew.
4. Run `compozy-cloud keys retire <old-kid>` with the new signing configuration and the same direct endpoint. The retirement commits to Postgres, removes the old ID from both JWKS URLs and publishes a `jwks` frame. Connected computers close sessions minted with that key. Reconnecting computers receive the same retirements. Remove the old public JWK from the configured overlap and discard old private material securely.

## Suspected key compromise

If the affected key is still active anywhere, take public traffic into maintenance and switch both replicas to a fresh signer. Publish the replacement in advance when available. Retire the compromised ID as soon as the replacement can sign, using the command in step 4. An instance configured with the retired signer fails readiness. Verify the old ID is absent from the public `keys` list and a formerly accepted grant and remote session signed by it are refused. Verify a fresh grant before reopening ingress.

Record the environment, key IDs, retirement time and verification in the incident. Use the current safe secrets during a rollback.
