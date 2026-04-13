---
status: resolved
file: internal/core/extension/host_helpers.go
line: 383
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPF,comment:PRRC_kwDORy7nkc621Va6
---

# Issue 026: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`In Go's encoding/json package, is Decoder.More a correct way to reject trailing top-level JSON after Decode, or should a second Decode expecting io.EOF be used instead?`

💡 Result:

Neither Decoder.More alone nor a second Decode expecting io.EOF is the correct or standard way to reject trailing top-level JSON after a successful Decode in Go's encoding/json package. Both approaches have issues. Decoder.More reports whether there is another element in the *current* array or object being parsed (pkg.go.dev/encoding/json#Decoder.More). For a top-level value (e.g., single object), after Decode completes successfully, the token state is not inside an array/object, so More returns false even if trailing data exists. It does not detect arbitrary trailing top-level JSON. A second Decode after the first succeeds will return io.EOF if at end-of-input (no trailing data), but if trailing JSON exists (e.g., another object), it succeeds in decoding it instead of erroring. To "expect io.EOF", you'd do: err := dec.Decode(&v2); if err != nil { if err != io.EOF { /* error */ } } else { /* trailing data! */ } This works but requires a second value v2 and assumes the trailing data decodes to v2's type. The idiomatic way to strictly validate a single top-level JSON value (rejecting trailing data) with Decoder is the streaming pattern from official docs: Decode the value, then if it's top-level array/object use while dec.More { dec.Token } followed by dec.Token for closing delim, but for arbitrary single value, after Decode, call dec.Token and expect io.EOF (not a delim or value). However, standard practice for rejecting trailing data is json.Unmarshal, which requires exactly one complete top-level value and errors on trailing garbage via checkValid (go.dev/src/encoding/json/decode.go). For io.Reader, read all with io.ReadAll(r) then Unmarshal, or use experimental json/v2 UnmarshalRead which consumes to EOF. Decoder is for streams of values; for single strict value, prefer Unmarshal or drain/check remaining with dec.Buffered after Decode then validate no non-whitespace.

Citations:

- 1: https://pkg.go.dev/encoding/json
- 2: https://pkg.go.dev/encoding/json/
- 3: https://pkg.go.dev/encoding/json@go1.25.6
- 4: https://go.dev/src/encoding/json/stream.go
- 5: https://pkg.go.dev/encoding/json@go1.25.5
- 6: https://ahmet.im/blog/golang-json-decoder-pitfalls/index.html

---



</details>

**`decoder.More()` cannot detect trailing top-level JSON and must be replaced.**

After a successful top-level `Decode`, `More()` only checks whether there is another element in the *current* array or object; it returns false even when arbitrary trailing data follows. This allows malformed requests with appended JSON to pass validation. Use a second `Decode` and check for `io.EOF` instead.

<details>
<summary>🩹 Proposed fix</summary>

```diff
 import (
 	"bytes"
 	"context"
 	"encoding/json"
 	"fmt"
+	"io"
 	"os"
 	"path/filepath"
 	"strings"
 	"sync/atomic"
 	"time"
 )
@@
-	if decoder.More() {
+	var extra json.RawMessage
+	if err := decoder.Decode(&extra); err != io.EOF {
 		return zero, subprocess.NewInvalidParams(map[string]any{
 			"method": method,
 			"error":  "unexpected trailing data",
 		})
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/host_helpers.go` around lines 363 - 383, The
decoder.More() check in decodeHostParams does not detect arbitrary trailing
top-level JSON; replace that check by attempting a second decoder.Decode(&extra
json.RawMessage) and treat io.EOF as the clean end-of-input, otherwise return
subprocess.NewInvalidParams with method and the decode error (or a custom
"unexpected trailing data" message); ensure you declare the extra variable
(e.g., var extra json.RawMessage) and import io so you can compare the error to
io.EOF while keeping the existing error payload structure used in
decodeHostParams.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:ce4005c2-f225-49f9-bac5-6ac8c129da42 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: `decoder.More()` only reports whether another element exists inside the current array/object; it does not reject arbitrary trailing top-level JSON after a successful decode. That means malformed Host API payloads can currently slip through as valid. The fix is to perform a second decode and require `io.EOF`.
