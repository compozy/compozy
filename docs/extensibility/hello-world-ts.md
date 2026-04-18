# Hello World in TypeScript

This example creates a minimal lifecycle observer in TypeScript.

## 1. Create the project

```bash
mkdir hello-ts
cd hello-ts
npm init -y
npm install @compozy/extension-sdk
npm install --save-dev typescript @types/node
```

## 2. Add `tsconfig.json`

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "rootDir": ".",
    "outDir": "dist",
    "types": ["node"]
  },
  "include": ["src/**/*.ts"]
}
```

## 3. Add `src/index.ts`

```ts
import { Extension } from "@compozy/extension-sdk";

const extension = new Extension("hello-ts", "0.1.0").onRunPostShutdown(
  async (_context, payload) => {
    process.stderr.write(`run ${payload.run_id} finished with ${payload.summary.status}\n`);
  }
);

extension.start().catch(error => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
```

## 4. Add `extension.toml`

```toml
[extension]
name = "hello-ts"
version = "0.1.0"
description = "Hello-world TypeScript extension"
min_compozy_version = "0.1.10"

[subprocess]
command = "node"
args = ["dist/src/index.js"]

[security]
capabilities = ["run.mutate"]

[[hooks]]
event = "run.post_shutdown"
```

## 5. Build, install, and enable

```bash
npx tsc -p tsconfig.json
compozy ext install --yes .
compozy ext enable hello-ts
```

## 6. Run Compozy

```bash
compozy exec "hello from the TypeScript extension"
```

You should see the final run status on stderr when the run shuts down.

## Faster path

The recommended path is to start from the scaffolded template instead of writing files manually:

```bash
npx @compozy/create-extension hello-ts --template lifecycle-observer
```
