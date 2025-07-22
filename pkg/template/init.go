package template

// Note: To avoid import cycles, template registration should be done
// by the main application or CLI command that uses the templates.
//
// Example usage in cli/cmd/init/init.go:
//
//   import (
//       "github.com/compozy/compozy/pkg/template"
//       "github.com/compozy/compozy/pkg/template/templates/basic"
//   )
//
//   func init() {
//       basic.Register()
//   }
