## Pokemon (Attachments Router) Example

This example demonstrates the unified `attachments` model with a router that branches to image, audio, or video analysis tasks.

Run from repo root:

```
cd examples/pokemon
```

Example inputs:

- Image: `kind=image ref=media/sample.png`
- Audio: `kind=audio ref=media/sample.wav`
- Video: `kind=video ref=media/sample.mp4`

Notes:

- Tests use only local fixtures; the example can reference a remote URL.
- Media files under `media/` are small test assets added to this example.
