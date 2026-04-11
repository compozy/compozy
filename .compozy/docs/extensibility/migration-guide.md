# Migration Guide from Early Prototypes

This guide is for pre-release extensions that were built before the public SDKs and starter templates shipped.

## What changed in v1

The supported authoring model is now:

- manifest-first with `extension.toml` or `extension.json`
- JSON-RPC 2.0 over stdin/stdout
- official SDKs for Go and TypeScript
- typed Host API clients instead of shelling out or writing internal files directly
- local enablement with `compozy ext enable`

## Migration checklist

1. Move subprocess metadata into `extension.toml`.
2. Declare capabilities under `[security]`.
3. Replace ad hoc JSON parsing with the official SDK `Extension` type.
4. Replace direct filesystem writes for tasks, artifacts, and memory with Host API calls.
5. Replace recursive `compozy` shellouts with `host.runs.start`.
6. Add initialize, hook, and shutdown tests with the official harness.

## Old pattern -> supported pattern

| Earlier prototype pattern                                | Supported v1 pattern                                                                  |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| Hand-written stdio loop                                  | `new Extension(...).start()` in TS or `extension.New(...).Start(...)` in Go           |
| Custom initialize request parsing                        | Let the SDK negotiate `protocol_version`, capability grants, and lifecycle flags      |
| Writing `.compozy/tasks/...` files directly              | Use `host.tasks.create` or `host.memory.write`                                        |
| Shelling out to `compozy exec` from inside the extension | Use `host.runs.start` so recursion protection and parent-run propagation stay correct |
| Implicit trust by repository presence                    | Install and enable explicitly with `compozy ext install` and `compozy ext enable`     |
| Template-specific internal fixtures                      | Start from `@compozy/create-extension` and keep local tests on the official harness   |

## Notes for declarative extensions

If you built early provider or skill prototypes:

- move provider overlays under `[[providers.ide]]`, `[[providers.review]]`, or `[[providers.model]]`
- move skill packs under `[resources] skills = ["skills/*"]`
- declare `providers.register` or `skills.ship` explicitly in the manifest

## Notes for executable extensions

If you already have working hook logic, the shortest migration path is:

1. keep the business logic
2. wrap it with the official SDK handler registration
3. move side effects behind the Host API
4. add one real subprocess smoke test

## Compatibility expectation

Protocol version `1` is the supported wire contract for the public SDKs in this release line. If your prototype used different method names or initialize direction assumptions, update it to match the published protocol and SDK helpers before shipping it to other users.
