## Go Package Recommendations for Release Automation (Keeping git-cliff and GoReleaser)

Since you want to keep **git-cliff** and **GoReleaser** as the main tools, here are the best Go packages to complement them for building your release automation system:

## Core Infrastructure

### **spf13/cobra** + **spf13/viper** [1][2]

The industry standard CLI framework and configuration management combo.

```go
import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)
```

**Perfect for:**

- Building subcommands like `release check-changes`, `release calculate-version`
- Managing configuration files and environment variables
- Providing consistent CLI experience

### **go-git/go-git** [3][4]

Pure Go Git operations without external dependencies.

```go
import "github.com/go-git/go-git/v5"
```

**Use cases:**

- Tag existence checking
- Branch creation and management
- Commit parsing and history analysis
- Remote operations (push/pull)

### **Masterminds/semver** [5][6]

Semantic versioning operations.

```go
import "github.com/Masterminds/semver/v3"
```

**Use cases:**

- Version parsing and validation
- Version comparison and bumping
- Constraint checking

## External Tool Integration

### **os/exec** (Standard Library) [7]

For running git-cliff and goreleaser commands.

```go
import "os/exec"
```

**Example integration:**

```go
// Run git-cliff to generate changelog
cmd := exec.Command("git-cliff", "--unreleased", "--tag", version)
output, err := cmd.Output()

// Run goreleaser
cmd := exec.Command("goreleaser", "release", "--clean")
err := cmd.Run()
```

## JSON Package Manipulation

### **encoding/json** (Standard Library) [8] + **goccy/go-json** [9]

For package.json manipulation with better performance.

```go
import (
    "encoding/json"
    // or for better performance:
    json "github.com/goccy/go-json"
)
```

**Use cases:**

- Reading and updating package.json files
- Version synchronization across multiple packages
- NPM package configuration

## Template Processing

### **text/template** (Standard Library) [10]

For PR body and template generation.

```go
import "text/template"
```

**Use cases:**

- Generate PR body from templates
- Create release notes
- Template configuration files

## HTTP and GitHub Integration

### **google/go-github** [11] + **go-resty/resty** [12]

GitHub API client and HTTP library.

```go
import (
    "github.com/google/go-github/v74/github"
    "github.com/go-resty/resty/v2"
)
```

**Use cases:**

- Create pull requests and releases
- Manage repository branches
- Upload release assets
- NPM registry operations

## File System Operations

### **spf13/afero** [13]

Filesystem abstraction for better testing.

```go
import "github.com/spf13/afero"
```

**Benefits:**

- Mock filesystem for testing
- Atomic file operations
- Cross-platform compatibility

## Package Publishing

### NPM Publishing via HTTP

Since you need NPM publishing, use **go-resty/resty** for HTTP operations:

```go
// Check if package version exists
resp, err := client.R().
    SetHeader("Accept", "application/json").
    Get("https://registry.npmjs.org/" + packageName + "/" + version)

// Publish package (using npm CLI via os/exec)
cmd := exec.Command("npm", "publish", "--access", "public")
cmd.Dir = packageDir
err := cmd.Run()
```

## Logging and Error Handling

### **uber-go/zap** [14]

High-performance structured logging.

```go
import "go.uber.org/zap"
```

### **pkg/errors** or Standard Library

For error wrapping and context.

This approach gives you the best of both worlds: the powerful functionality of git-cliff and GoReleaser with the reliability, performance, and maintainability of Go.

Sources
[1] spf13/cobra: A Commander for modern Go CLI interactions - GitHub https://github.com/spf13/cobra
[2] viper package - github.com/spf13/viper - Go Packages https://pkg.go.dev/github.com/spf13/viper
[3] A highly extensible Git implementation in pure Go. - GitHub https://github.com/go-git/go-git
[4] git - Go Packages https://pkg.go.dev/github.com/go-git/go-git/v5
[5] Masterminds/semver: Work with Semantic Versions in Go - GitHub https://github.com/Masterminds/semver
[6] semver package - github.com/Masterminds/semver/v3 - Go Packages https://pkg.go.dev/github.com/Masterminds/semver/v3
[7] os/exec - Go Packages https://pkg.go.dev/os/exec
[8] encoding/json - Go Packages https://pkg.go.dev/encoding/json
[9] goccy/go-json: Fast JSON encoder/decoder compatible ... - GitHub https://github.com/goccy/go-json
[10] text/template - Go Packages - The Go Programming Language https://pkg.go.dev/text/template
[11] google/go-github: Go library for accessing the GitHub v3 API - GitHub https://github.com/google/go-github
[12] go-resty/resty: Simple HTTP, REST, and SSE client library for Go https://github.com/go-resty/resty
[13] spf13/afero: The Universal Filesystem Abstraction for Go - GitHub https://github.com/spf13/afero
[14] uber-go/zap: Blazing fast, structured, leveled logging in Go. - GitHub https://github.com/uber-go/zap
[15] file_contents.txt https://ppl-ai-file-upload.s3.amazonaws.com/web/direct-files/attachments/1584017/d8bf9b85-5b68-487f-82b7-8bfc2941731c/file_contents.txt
[16] Turning Git commits into changelog with Git-Cliff - Orhun Parmaksız https://www.youtube.com/watch?v=RWh8qbiLRts
[17] GoReleaser & GoTest CI - Go-Blueprint Docs https://docs.go-blueprint.dev/advanced-flag/goreleaser/
[18] git-cliff(1) - Arch Linux manual pages https://man.archlinux.org/man/git-cliff.1.en
[19] git-cliff: A highly customizable changelog generator ⛰️ https://git-cliff.org
[20] Continuous Integration with GoReleaser - Omri Bornstein https://applegamer22.github.io/posts/go/goreleaser/
[21] Golang: Executing a command with its arguments - Stack Overflow https://stackoverflow.com/questions/38375145/golang-executing-a-command-with-its-arguments
[22] orhun/git-cliff-action: GitHub action to generate a changelog based ... https://github.com/orhun/git-cliff-action
[23] How to Publish Your Golang Binaries with Goreleaser - Kosli https://www.kosli.com/blog/how-to-publish-your-golang-binaries-with-goreleaser/
[24] Examples | git-cliff https://git-cliff.org/docs/usage/examples/
[25] GitHub Integration | git-cliff https://git-cliff.org/docs/integration/github/
[26] Quick Start - GoReleaser https://goreleaser.com/quick-start/
[27] octocat: Github action to run git-cliff with a custom cliff.toml https://github.com/tj-actions/git-cliff
[28] cliff package - github.com/orsinium-labs/cliff - Go Packages https://pkg.go.dev/github.com/orsinium-labs/cliff
[29] GoReleaser - GoReleaser https://goreleaser.com
[30] Can't get go exec command to execute git config command properly https://stackoverflow.com/questions/34030318/cant-get-go-exec-command-to-execute-git-config-command-properly
[31] orhun/git-cliff: A highly customizable Changelog Generator ... - GitHub https://github.com/orhun/git-cliff
[32] Building Go modules - GoReleaser https://goreleaser.com/cookbooks/build-go-modules/
[33] Blog - git-cliff https://git-cliff.org/blog
[34] git-cliff - crates.io: Rust Package Registry https://crates.io/crates/git-cliff/1.0.0
[35] GitHub - go-semantic-release/hooks-goreleaser https://github.com/go-semantic-release/hooks-goreleaser
[36] golang-npm https://www.npmjs.com/package/golang-npm
[37] JSON manipulation in Go - Nikhil Akki's blog https://nikhilakki.in/json-manipulation-in-go
[38] Mastering Go Templates: A Guide with Practical Examples https://www.codingexplorations.com/blog/mastering-go-templates-a-guide-with-practical-examples
[39] Working with the npm registry - GitHub Docs https://docs.github.com/packages/working-with-a-github-packages-registry/working-with-the-npm-registry
[40] pjovanovic05/gojq: Easier manipulation of JSON data in Golang https://github.com/pjovanovic05/gojq
[41] How To Use Templates in Go - DigitalOcean https://www.digitalocean.com/community/tutorials/how-to-use-templates-in-go
[42] sanathkr/go-npm: Distribute and install Go binaries via NPM - GitHub https://github.com/sanathkr/go-npm
[43] How To Use JSON in Go - DigitalOcean https://www.digitalocean.com/community/tutorials/how-to-use-json-in-go
[44] Publish YOUR OWN public NPM package ! (in 5 simple steps) https://www.youtube.com/watch?v=cUfXaK2ybks
[45] Editing Json in Go without Unmarshalling into Structs First https://stackoverflow.com/questions/51293840/editing-json-in-go-without-unmarshalling-into-structs-first
[46] Go templates | GoLand Documentation - JetBrains https://www.jetbrains.com/help/go/integration-with-go-templates.html
[47] How to Create and Publish an npm Package: A Complete Guide https://www.aubergine.co/insights/mastering-npm-how-to-create-publish-and-manage-your-own-package
[48] Using Functions Inside Go Templates - Calhoun.io https://www.calhoun.io/using-functions-inside-go-templates/
[49] How to Create and Publish an NPM Package – a Step-by-Step Guide https://www.freecodecamp.org/news/how-to-create-and-publish-your-first-npm-package/
[50] Go template libraries: A performance comparison - LogRocket Blog https://blog.logrocket.com/golang-template-libraries-performance-comparison/
[51] Publishing a Go binary to NPM - Hong Shick Pak https://hspak.dev/post/publish-npm/
[52] encoding/json/v2 - Go Packages https://pkg.go.dev/encoding/json/v2
