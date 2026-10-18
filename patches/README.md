# Dependency patches

`@hey-api/openapi-ts@0.99.0` ships an SSE template whose abort handler discards the
Promise returned by `reader.cancel()`. If aborting fetch has already errored the
reader, cancellation rejects and can terminate the Bun host through an unhandled
rejection. [This generated-client reproduction](https://github.com/anomalyco/opencode/issues/47963)
matches the real TypeScript SDK journey on Bun 1.4.0.

The Bun patch consumes that cleanup Promise in the template used by both generated
SDK jobs. Generated files are still produced and checked by `make codegen` and
`make codegen-check`. Transport and parsing errors retain their existing paths.

Remove this patch and its `patchedDependencies` entry when the pinned generator's
SSE template handles cancellation rejections itself. Keep the existing client
abort regression and rerun `TestDaemonE2EAppSDK` with the public built package to
verify that stopping an event reader leaves the host alive without replaying work.
