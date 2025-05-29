# `$ref` Reference Specification · v0.2 (2025-05-28)

> A compact, deterministic way to pull data from the same
> document, another file, or the global config
> and to blend that data into the current node.

---

## 1 · Two representation forms

| Form                           | YAML example                                                 | Typical use              |
| ------------------------------ | ------------------------------------------------------------ | ------------------------ |
| **Object** (verbose, explicit) | `yaml $ref: {type: property, path: schemas.#(id=="city_input")} ` | Generated files, clarity |
| **String** (shorthand)         | `yaml $ref: schemas.#(id=="city_input")`                          | Hand-written docs        |

Both map 1-to-1 to the same internal `Ref` struct (see §7).

---

## 2 · Object form (canonical)

```yaml
$ref:
  type: property | file | global                  # REQUIRED
  # === common ===
  path: <gjson-path>                              # OPTIONAL (root if omitted)
  mode: merge | replace | append                  # OPTIONAL (default merge)

  # === type=file ===
  file: <relative|absolute path or URL>

  # === type=global ===
  # no extra keys

  # === type=property ===
  # no extra keys
```

Validation error if a key appears that is not allowed for the chosen `type`.

---

## 3 · String form

```
<source> [ "::" <path> ] [ "!" <mode> ]
```

### 3.1 `source` → `type` mapping

| String prefix / value                  | Interpreted `type` | Examples                          |
| -------------------------------------- | ------------------ | --------------------------------- |
| `$global`                              | `global`           | `$global`                         |
| starts with `./`, `/`, `http`, `https` | `file`             | `./remote.yaml`, `https://…`      |
| *blank* or anything else               | `property`         | `tools.#(id=="save")`, *(blank)*       |

### 3.2  Examples

| What you need             | String                                                      |
| ------------------------- | ----------------------------------------------------------- |
| Same doc, schema element  | `schemas.#(id=="city_input")`                          |
| Same doc, but **replace** | `schemas.#(id=="city_input")!replace`                  |
| External file             | `./wf.yaml::tools.#(id=="weather")`                    |
| Global provider           | `$global::providers.#(id=="groq_llama")`               |

---

## 4 · GJSON Path syntax (works in both forms)

We use [GJSON path syntax](https://github.com/tidwall/gjson#path-syntax) which is simple and intuitive:

**Basic Access:**
* `schemas.0` - First schema element
* `schemas.#` - Number of schemas
* `providers.groq_llama.model` - Nested property access
* `name.last` - Dot notation for nested objects

**Array Queries:**
* `schemas.#(id=="city_input")` - Schema with id 'city_input'
* `friends.#(age>45)` - Friends older than 45
* `friends.#(last=="Murphy").first` - First name of friend with last name Murphy
* `friends.#(first%"D*").last` - Last name of friends whose first name starts with D

**Wildcards:**
* `children.*` - All children properties
* `friends.#.first` - All first names from friends array

**Escaping:**
* `fav\.movie` - Access property with dot in name

For complete syntax reference, see [GJSON Syntax Documentation](https://github.com/tidwall/gjson/blob/master/SYNTAX.md).

---

## 5 · Merge modes

| Mode (default **merge**) | Array result | Map/Object result                     |
| ------------------------ | ------------ | ------------------------------------- |
| `merge`                  | **replaced** | deep-merge (inline wins on conflicts) |
| `replace`                | replaced     | replaced                              |
| `append`                 | concatenated | **error**                             |

If `$ref` is the **only** key under its parent, the chosen mode is ignored; the parent *becomes* the resolved value.

---

## 6 · Resolution algorithm (normative)

```text
input: currentDoc, currentFilePath, $ref node
output: JSON/YAML value

1. Parse $ref
   • if object → already structured
   • if string → apply §3 mapping to internal Ref struct

2. Select source document
   switch ref.Type
     property   → doc = currentDoc
     file       → doc = load(ref.File, currentFilePath)
     global     → doc = load(projectRoot / "compozy.yaml")

3. Extract node: value = walkGJSONPath(doc, ref.Path)
   • If ref.Path blank → whole doc
   • walkGJSONPath fails → error "path not found (with line numbers)"

4. Merge:
   • if parent had inline keys besides $ref
       apply mergeMode(ref.Mode, value, inlineObject)
   • else
       result = value

5. Detect circular chain
   keep map[ file+line ]bool; depth-limit 20.

return result
```

*`walkGJSONPath`*
Uses GJSON library for robust path traversal with full GJSON spec support.

---

## 7 · Internal Go struct

```go
type Ref struct {
    Type    string // "property", "file", "global"
    // shared
    Path    string // GJSON path expression
    Mode    string // "", "merge", "replace", "append"

    // file
    File    string
}
```

Implement:

* `ParseRef(node yaml.Node) (Ref, error)`
  – handles both mapping and string.
* `Resolve(ref Ref, ctx *Context) (any, error)`
  – uses the algorithm in §6.

Suggested libs:

| Task                        | Package                    |
| --------------------------- | -------------------------- |
| YAML with line numbers      | `gopkg.in/yaml.v3`         |
| Deep merge                  | `github.com/imdario/mergo` |
| GJSON path traversal        | `github.com/tidwall/gjson` |

---

## 8 · Error handling rules

* **Unknown type** → *fatal*: `unknown ref type "..."`.
* **Field not allowed for type** → *fatal* with key name.
* **Path lookup failure** → *fatal* with most specific
  segment reached and file\:line.
* **Merge mode append on map** → *fatal*: `"append" only valid on arrays`.
* **Circular reference** → *fatal*: show chain of `$ref`s.

All errors should include the file path and **line/column** of the `$ref`.

---

## 9 · Test matrix (minimum)

| Case                       | Expect                          |
| -------------------------- | ------------------------------- |
| property root (`""`)       | whole doc returned              |
| property GJSON query miss  | error                           |
| file URL fetch             | success, file cached            |
| merge vs. replace          | inline wins / loses accordingly |
| append                     | array grows                     |
| circular (A→B→A)           | error depth ≤20                 |

---

## 10 · Example document (excerpt)

```yaml
config:
  inherit_params: true
  input:
    $ref: schemas.#(id=="city_input")        # shorthand

agents:
  - id: inline_agent
    config:
      $ref:
        type: global
        path: providers.#(id=="groq_llama")  # object form

tasks:
  - id: external_schema
    $ref: ./api.yaml::schemas.#(id=="req")!replace
```

---

## 11 · Versioning & compatibility

* This spec is **v0.2**.
* Parsers **MUST** reject `type` values they do not recognise.
* New `type`s may be added in minor versions; they must not reuse existing prefixes in the string form.

---

### End of spec

Copy–paste above into a Markdown file ­– you now have an authoritative reference for coding, testing, and future contributors. 