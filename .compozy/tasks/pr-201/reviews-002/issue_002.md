---
provider: coderabbit
pr: "201"
round: 2
round_created_at: 2026-06-15T19:35:34.422493Z
status: resolved
file: internal/core/run/ui/view_test.go
line: 1614
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jr-C6,comment:PRRC_kwDORy7nkc7LnPei
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Find and examine the test file and helper function
fd -type f -name "view_test.go" internal/core/run/ui/
```

Repository: compozy/compozy

Length of output: 230

---

🏁 Script executed:

```shell
# Read the specific function and surrounding context
cat -n internal/core/run/ui/view_test.go | sed -n '1600,1650p'
```

Repository: compozy/compozy

Length of output: 1744

---

🏁 Script executed:

```shell
# Search for test cases related to hasForcedBackgroundSGR or similar
rg -n "hasForcedBackgroundSGR|sgrParamsHaveForcedBackground" internal/core/run/ui/view_test.go -A 3 -B 1
```

Repository: compozy/compozy

Length of output: 861

---

🏁 Script executed:

```shell
# Check if there are any existing test cases for the SGR background detection
rg -n "TestHasForcedBackgroundSGR" internal/core/run/ui/view_test.go -A 20
```

Repository: compozy/compozy

Length of output: 980

---

🏁 Script executed:

```shell
# Check how the escape sequences are extracted and processed
cat -n internal/core/run/ui/view_test.go | sed -n '1585,1602p'
```

Repository: compozy/compozy

Length of output: 615

---

🏁 Script executed:

```shell
# Verify the ANSI SGR specification understanding by checking if there are other test patterns
rg -n "\\\\x1b\\\[" internal/core/run/ui/view_test.go | head -20
```

Repository: compozy/compozy

Length of output: 136

---

🏁 Script executed:

```shell
# Search for escape sequence patterns in test cases (simpler approach)
rg -n "38;5|48;5|48;2" internal/core/run/ui/view_test.go
```

Repository: compozy/compozy

Length of output: 226

---

🏁 Script executed:

```shell
# Look at the full test to understand expected behavior
cat -n internal/core/run/ui/view_test.go | sed -n '1542,1583p'
```

Repository: compozy/compozy

Length of output: 1789

---

🏁 Script executed:

```shell
# Verify the fix logic by examining if there are any edge cases we need to consider
# Check if there are patterns like just "48" at the end of a sequence
rg -n "\\[48m" internal/core/run/ui/view_test.go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check the full context around where this function is used to ensure nothing else depends on the current behavior
rg -n "sgrParamsHaveForcedBackground" internal/core/run/ui/
```

Repository: compozy/compozy

Length of output: 259

---

🏁 Script executed:

```shell
python3 << 'PY'
import re

def sgrParamsHaveForcedBackground_current(params):
    """Current implementation - has the bug"""
    if params == "":
        return False
    for raw in params.split(";"):
        try:
            value = int(raw)
            if value == 7 or value == 48 or (40 <= value <= 47) or (100 <= value <= 107):
                return True
        except ValueError:
            continue
    return False

def sgrParamsHaveForcedBackground_fixed(params):
    """Proposed fix"""
    if params == "":
        return False
    parts = params.split(";")
    for i in range(len(parts)):
        raw = parts[i]
        try:
            value = int(raw)
            if value == 7 or (40 <= value <= 47) or (100 <= value <= 107):
                return True
            if value == 48:
                # Extended background forms: 48;5;<idx> or 48;2;<r>;<g>;<b>
                if i + 1 < len(parts):
                    try:
                        mode = int(parts[i + 1])
                        if mode == 5 or mode == 2:
                            return True
                    except ValueError:
                        pass
        except ValueError:
            continue
    return False

# Test cases from the test file
test_cases = [
    ("38;2;255;255;255", False),  # foreground truecolor
    ("48;2;1;2;3", True),          # background truecolor
    ("48;5;12", True),             # background 256-color
    ("44", True),                  # standard 16 color background
    ("104", True),                 # bright 16 color background
    ("7", True),                   # reverse video
    # The problematic case from review comment
    ("38;5;48", False),            # foreground 256-color with index 48
    # Edge cases
    ("48", False),                 # bare 48 without mode specifier
]

print("Testing current implementation vs proposed fix:\n")
print(f"{'Parameters':20} {'Expected':10} {'Current':10} {'Status':5}  {'Fixed':10} {'Status':5}")
print("-" * 70)
for params, expected in test_cases:
    current_result = sgrParamsHaveForcedBackground_current(params)
    fixed_result = sgrParamsHaveForcedBackground_fixed(params)
    current_ok = "✓" if current_result == expected else "✗"
    fixed_ok = "✓" if fixed_result == expected else "✗"
    print(f"{params:20} {str(expected):10} {str(current_result):10} {current_ok:5}  {str(fixed_result):10} {fixed_ok:5}")
PY
```

Repository: compozy/compozy

Length of output: 784

---



</details>

**Fix false positives in foreground ANSI sequence detection.**

The function incorrectly treats any isolated `48` token as a background indicator, causing false positives for foreground extended-color sequences like `38;5;48` (foreground color index 48). Additionally, a bare `48` without a mode specifier (`5` or `2`) is not a valid complete SGR sequence and should not be flagged as background.

<details>
<summary>Suggested fix</summary>

```diff
 func sgrParamsHaveForcedBackground(params string) bool {
 	if params == "" {
 		return false
 	}
-	for _, raw := range strings.Split(params, ";") {
+	parts := strings.Split(params, ";")
+	for i := 0; i < len(parts); i++ {
+		raw := parts[i]
 		value, err := strconv.Atoi(raw)
 		if err != nil {
 			continue
 		}
-		if value == 7 || value == 48 || (value >= 40 && value <= 47) || (value >= 100 && value <= 107) {
+		if value == 7 || (value >= 40 && value <= 47) || (value >= 100 && value <= 107) {
 			return true
 		}
+		if value == 48 {
+			// Extended background forms: 48;5;<idx> or 48;2;<r>;<g>;<b>
+			if i+1 < len(parts) {
+				if mode, err := strconv.Atoi(parts[i+1]); err == nil && (mode == 5 || mode == 2) {
+					return true
+				}
+			}
+		}
 	}
 	return false
 }
```
</details>

Add a test case in `TestHasForcedBackgroundSGR` for `"\x1b[38;5;48mtext\x1b[0m"` expecting `false`.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func sgrParamsHaveForcedBackground(params string) bool {
	if params == "" {
		return false
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		raw := parts[i]
		value, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		if value == 7 || (value >= 40 && value <= 47) || (value >= 100 && value <= 107) {
			return true
		}
		if value == 48 {
			// Extended background forms: 48;5;<idx> or 48;2;<r>;<g>;<b>
			if i+1 < len(parts) {
				if mode, err := strconv.Atoi(parts[i+1]); err == nil && (mode == 5 || mode == 2) {
					return true
				}
			}
		}
	}
	return false
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/ui/view_test.go` around lines 1604 - 1614, The
sgrParamsHaveForcedBackground function incorrectly flags extended-color
foreground sequences as having a background because it checks for isolated
tokens matching background values without considering context. The issue is that
in a sequence like "38;5;48", the value 48 is a color index for foreground
(indicated by the leading 38), not a background color selector. Fix the function
by tracking when you encounter a 38 (foreground) or 48 (background) token
followed by 5 or 2, then properly skip over or interpret the subsequent color
parameter tokens based on that context. Return true only when you encounter 48
with a mode (5 or 2), or basic background color codes (40-47, 100-107) that
stand alone. Additionally, add a test case in TestHasForcedBackgroundSGR that
verifies the string "\x1b[38;5;48mtext\x1b[0m" returns false.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:4216525b5329ee6d6400b669 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `sgrParamsHaveForcedBackground` scans every SGR parameter independently and treats any token equal to `48` as a background selector. In a foreground extended-color sequence such as `38;5;48`, the trailing `48` is the foreground color index, not a background control code.
- Fix approach: Parse extended color selectors contextually. `48;5;<idx>` and `48;2;<r>;<g>;<b>` indicate backgrounds, while `38;5;<idx>` and `38;2;<r>;<g>;<b>` should skip their color payloads. Add a focused regression for `38;5;48` returning false while preserving existing background detections.
- Verification: Focused UI/daemon tests passed, touched-package tests passed, and full `rtk make verify` passed after the production lint follow-up.
