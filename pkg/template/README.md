# Template Package

The template package provides an extensible, interface-based system for project template generation in Compozy.

## Overview

This package implements a registry-based template system that allows easy addition of new project templates without modifying existing code.

## Architecture

### Core Components

1. **Template Interface** (`types.go`)
   - Defines the contract for all templates
   - Methods for metadata, files, directories, and configuration

2. **Registry** (`registry.go`)
   - Global registry for template management
   - Thread-safe registration and retrieval

3. **Generator** (`generator.go`)
   - Handles file creation and template processing
   - Manages directory structure and permissions

4. **Service** (`service.go`)
   - Public API for template operations
   - Singleton pattern for global access

## Usage

### Getting the Service

```go
templateSvc := template.GetService()
```

### Listing Available Templates

```go
templates := templateSvc.List()
for _, tmpl := range templates {
    fmt.Printf("Template: %s - %s\n", tmpl.Name, tmpl.Description)
}
```

### Generating a Project

```go
opts := &template.GenerateOptions{
    Path:        "./my-project",
    Name:        "My Project",
    Description: "A sample project",
    Version:     "0.1.0",
    Author:      "John Doe",
    DockerSetup: true,
}

err := templateSvc.Generate("basic", opts)
if err != nil {
    log.Fatal(err)
}
```

## Creating a New Template

To add a new template, implement the `Template` interface:

```go
package mytemplate

import (
    _ "embed"
    "github.com/compozy/compozy/pkg/template"
)

//go:embed files/config.yaml.tmpl
var configTemplate string

//go:embed files/main.go.tmpl
var mainTemplate string

type MyTemplate struct{}

func init() {
    // Self-register the template
    template.Register("mytemplate", &MyTemplate{})
}

func (t *MyTemplate) GetMetadata() template.Metadata {
    return template.Metadata{
        Name:        "mytemplate",
        Description: "My custom template",
        Author:      "Template Author",
        Version:     "1.0.0",
    }
}

func (t *MyTemplate) GetFiles() []template.File {
    return []template.File{
        {
            Name:    "config.yaml",
            Content: configTemplate,
        },
        {
            Name:        "main.go",
            Content:     mainTemplate,
            Permissions: 0755,
        },
    }
}

func (t *MyTemplate) GetDirectories() []string {
    return []string{"src", "tests", "docs"}
}

func (t *MyTemplate) GetProjectConfig(opts *template.GenerateOptions) any {
    return struct {
        Name        string
        Description string
        Version     string
    }{
        Name:        opts.Name,
        Description: opts.Description,
        Version:     opts.Version,
    }
}
```

## Template Functions

Templates support all Sprig functions plus:

- `jsEscape`: JavaScript string escaping
- `yamlEscape`: YAML string escaping
- `htmlEscape`: HTML string escaping

## File Permissions

Set file permissions using the `Permissions` field in `template.File`:

```go
{
    Name:        "script.sh",
    Content:     scriptContent,
    Permissions: 0755, // Executable
}
```

## Special Handling

### env.example Files

If `env.example` already exists in the target directory, the generator will automatically create `env-compozy.example` instead to avoid overwriting existing environment configuration.
