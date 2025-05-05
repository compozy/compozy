module github.com/compozy/compozy

go 1.22

require (
	github.com/sirupsen/logrus v1.9.3 // Logging (like tracing)
	github.com/spf13/cobra v1.9.1 // CLI parsing (like clap)
	github.com/stretchr/testify v1.10.0 // Testing (like pretty_assertions)
	github.com/xeipuuv/gojsonschema v1.2.0 // JSON schema validation
	gopkg.in/yaml.v3 v3.0.1 // YAML parsing (like serde_yaml)
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect; Indirect (cobra)
	github.com/spf13/pflag v1.0.6 // indirect; Indirect (cobra)
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect; Indirect (testify)
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
)
