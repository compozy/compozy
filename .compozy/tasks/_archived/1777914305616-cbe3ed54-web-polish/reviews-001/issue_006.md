---
status: resolved
file: packages/ui/src/components/skeleton.tsx
line: 29
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc59RNfo,comment:PRRC_kwDORy7nkc662oQW
---

# Issue 006: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Normalize `lines` once before rendering and last-line styling.**

Line 29 compares against raw `lines`, while Line 28 uses `Math.max(1, lines)`. With `lines <= 0`, one row renders but never gets the intended last-line width class.


<details>
<summary>💡 Proposed fix</summary>

```diff
 export function SkeletonText({
   className,
   lines = 3,
   ...props
 }: HTMLAttributes<HTMLDivElement> & { lines?: number }): ReactElement {
+  const normalizedLines = Number.isFinite(lines) ? Math.max(1, Math.floor(lines)) : 1;
   return (
     <div className={cn("space-y-2", className)} aria-hidden="true" {...props}>
-      {Array.from({ length: Math.max(1, lines) }).map((_, index) => (
-        <Skeleton className={cn("h-3", index === lines - 1 ? "w-2/3" : "w-full")} key={index} />
+      {Array.from({ length: normalizedLines }).map((_, index) => (
+        <Skeleton
+          className={cn("h-3", index === normalizedLines - 1 ? "w-2/3" : "w-full")}
+          key={index}
+        />
       ))}
     </div>
   );
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
export function SkeletonText({
  className,
  lines = 3,
  ...props
}: HTMLAttributes<HTMLDivElement> & { lines?: number }): ReactElement {
  const normalizedLines = Number.isFinite(lines) ? Math.max(1, Math.floor(lines)) : 1;
  return (
    <div className={cn("space-y-2", className)} aria-hidden="true" {...props}>
      {Array.from({ length: normalizedLines }).map((_, index) => (
        <Skeleton
          className={cn("h-3", index === normalizedLines - 1 ? "w-2/3" : "w-full")}
          key={index}
        />
      ))}
    </div>
  );
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@packages/ui/src/components/skeleton.tsx` around lines 28 - 29, Normalize the
lines count before rendering in the Skeleton component: compute a local variable
(e.g., normalizedLines = Math.max(1, lines)) and use that for Array.from length
and for the last-line check (compare index === normalizedLines - 1) instead of
comparing against the raw lines prop so the last-line width class is applied
when lines <= 0.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:e5619032-55b8-42c6-8eab-a9afbd490671 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `SkeletonText` normalizes the line count for rendering length, but still compares the last-row width against the raw `lines` prop.
  - With `lines <= 0`, the component renders one row and never applies the intended `w-2/3` last-line class.
  - Fix: normalize once and reuse that value for both iteration and last-line detection, then add a regression test.
