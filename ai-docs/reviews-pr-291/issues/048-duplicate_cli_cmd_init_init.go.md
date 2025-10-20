# Duplicate comments for `cli/cmd/init/init.go`

## Duplicate from Comment 4

**File:** `cli/cmd/init/init.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
<summary>cli/cmd/init/init.go (3)</summary><blockquote>

`74-87`: **Add missing --install-bun flag binding.**

The `Options.InstallBun` field is never bound to a CLI flag in `applyInitFlags`. This means JSON/non-interactive users cannot trigger Bun installation via the command line, making the feature only accessible through the TUI form.




Apply this diff to add the flag binding:

```diff
 func applyInitFlags(command *cobra.Command, opts *Options) {
   command.Flags().StringVarP(&opts.Name, "name", "n", "", "Project name")
   command.Flags().StringVarP(&opts.Description, "description", "d", "", "Project description")
   command.Flags().StringVarP(&opts.Version, "version", "v", "0.1.0", "Project version")
   command.Flags().StringVarP(&opts.Template, "template", "t", "basic", "Project template")
   command.Flags().StringVar(&opts.Author, "author", "", "Author name")
   command.Flags().StringVar(&opts.AuthorURL, "author-url", "", "Author URL")
   command.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Force interactive mode")
   command.Flags().BoolVar(&opts.DockerSetup, "docker", false, "Include Docker Compose setup")
+  command.Flags().BoolVar(&opts.InstallBun, "install-bun", false, "Install Bun runtime if missing")
 }
```

---

`89-104`: **Remove non-existent "format" flag check.**

The init command does not define a "format" flag in `applyInitFlags`. The check `!cobraCmd.Flags().Changed("format")` always returns `true` for non-existent flags, causing unintended implicit Interactive mode activation when `opts.Name == ""`.




Remove the non-existent flag check:

```diff
 func prepareInitOptions(cobraCmd *cobra.Command, opts *Options, args []string) error {
   if len(args) > 0 {
     opts.Path = args[0]
   } else {
     opts.Path = "."
   }
   absPath, err := filepath.Abs(opts.Path)
   if err != nil {
     return fmt.Errorf("failed to resolve path: %w", err)
   }
   opts.Path = absPath
-  if opts.Name == "" && !cobraCmd.Flags().Changed("format") {
+  if opts.Name == "" {
     opts.Interactive = true
   }
   return nil
 }
```

---

`185-193`: **Strengthen Bun installation safety and consent.**

The current implementation can execute the curl|bash installer without explicit user consent in TUI mode (if `opts.InstallBun` is set via the form). Consider:
- In non-interactive/JSON mode: The installation should only proceed if `--install-bun` flag is explicitly set (requires fixing the missing flag binding).
- In TUI mode: The form already captures consent, which is good.
- Platform safety: The installer assumes bash and won't work on Windows without WSL.
- No integrity verification: Consider warning users or providing manual installation guidance.




Consider adding platform detection and clearer error messaging:

```diff
 func installBunIfNeeded(ctx context.Context, opts *Options) error {
   if !opts.InstallBun || runtime.IsBunAvailable() {
     return nil
   }
+  // Check if bash is available (required by installer)
+  if _, err := exec.LookPath("bash"); err != nil {
+    return fmt.Errorf("bash is required for Bun installation. Please install Bun manually from https://bun.sh")
+  }
   if err := installBun(ctx); err != nil {
     return fmt.Errorf("failed to install Bun: %w", err)
   }
   return nil
 }
```

</blockquote></details>
