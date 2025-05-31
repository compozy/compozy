# Ref Package Specification

## Overview

The **Ref** package provides declarative helpers that let workflow authors reuse, combine, and conditionally render configuration fragments while keeping every file valid YAML.

Core directives:

| Directive      | Purpose                                                                                              |
| -------------- | ---------------------------------------------------------------------------------------------------- |
| **`$use`**     | Transform a referenced component into a concrete **component configuration** (agent, tool, or task). |
| **`$ref`**     | Inject any JSON/YAML value directly.                                                                 |
| **`$merge`**   | Merge multiple objects **or** arrays with optional strategies.                                       |
| **`$include`** | Inline the root value of an external file.                                                           |
| **`$when`**    | Conditionally render a node based on an expression.                                                  |

All directives expand during a single **evaluation pass** so they compose predictably.

---

## Syntax Reference

### `$use` – Component shortcut

```yaml
$use: <component=agent|tool|task>(<scope=local|global>::<gjson_path>)
```

*Returns* a fully expanded component configuration.

---

### `$ref` – Raw value reference

```yaml
$ref: <scope=local|global>::<gjson_path>
```

*Returns* the value found at the GJSON‑style path.

---

<!-- Implement Later -->

### `$merge` – Declarative merge helper

The node **under** `$merge` must be either:

1. A **sequence** (shorthand) – implicitly `{ strategy: default, sources: <sequence> }`.
2. A **mapping** containing a `sources` key plus optional merge options.

```yaml
a:                               # object merge (shorthand)
  $merge:
    - { host: localhost, port: 80 }
    - { port: 8080, proto: https }

b:                               # array merge (shorthand)
  $merge:
    - [auth, logging]
    - [metrics, tracing]

c:                               # explicit with options
  $merge:
    strategy: deep               # objects: deep | shallow
    key_conflict: last           # objects: last | first | error
    sources:
      - $ref: local::defaults
      - { timeout: 30 }
```

#### Merge options

| Option         | Applies to | Values (default **bold**)   | Meaning                                        |
| -------------- | ---------- | --------------------------- | ---------------------------------------------- |
| `strategy`     | objects    | **deep**, shallow           | Recurse into nested maps or replace whole keys |
| `strategy`     | arrays     | **concat**, prepend, unique | How to combine arrays                          |
| `key_conflict` | objects    | **last**, first, error      | Winner on duplicate keys                       |

> **Rule:** All items in `sources` must be maps (→ object merge) **or** sequences (→ array merge). Mixed lists are invalid.

---

### `$include` – Inline external file

```yaml
$include: <path>
```

*Reads* the referenced file (JSON, YAML, or plain text) **once** and inserts its root value. Paths are resolved relative to the current file unless absolute.

```yaml
secrets:
  $include: ./common/secrets.yaml
```

---

### `$when` – Conditional inclusion

```yaml
$when:
  if: <expression>
  then: <yaml‑node>
  else: <yaml‑node>   # optional
```

*Evaluates* the boolean `if` expression; renders `then` when truthy, otherwise `else` (if provided). Expression syntax supports:

* environment variables: `$env::VAR`
* refs: `$ref: ...` within the expression
* basic operators: `==`, `!=`, `in`, `!`, `&&`, `||`

```yaml
logging:
  $when:
    if: $env::CI == true
    then: { level: verbose }
    else: { level: info }
```

---

## Semantics

* **Order‑independent** – Directives can nest arbitrarily; evaluator must resolve until no directive nodes remain.
* `$merge` operates left‑to‑right per `strategy`.
* `$include` is a *pure include* (no further IO once loaded). Circular includes are a validation error.
* `$when` nodes disappear after evaluation, leaving only the selected branch.

---

## Validation Rules

1. `$merge` must be sequence or mapping with `sources`.
2. Mixed source types in `$merge.sources` are invalid.
3. `$include` path must exist and be readable; file must parse as JSON or YAML unless treated as scalar string.
4. `$when` requires `if` and `then` keys; `else` is optional.
5. Unknown keys in directive mappings fail validation for forward‑compatibility.

---

## Examples

### 1. Deployment config merge

```yaml
deploy:
  $merge:
    - $ref: local::defaults.deploy
    - $ref: global::envs.prod.deploy
    - { retries: 5 }
```

### 2. Building a unique tag list

```yaml
steps:
  tags:
    $merge:
      strategy: unique
      sources:
        - $ref: local::base_tags
        - [build, docker]
```

### 3. Including secrets from a file

```yaml
secrets:
  $include: ./common/secrets.yaml
```

### 4. Environment‑specific logging

```yaml
logging:
  $when:
    if: $env::CI == true
    then: { level: verbose }
    else: { level: info }
```

---

## Additional (Future) Directives

* **`$env`** – Inject the value of an environment variable.
* **`$set`** – Override a deep‑nested key via path.

These maintain the same principles—valid YAML, minimal ceremony, declarative power—and may be adopted later.