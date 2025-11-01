package runtime

//go:generate go run ../internal/codegen/cmd/optionsgen/main.go -engine ../../engine/runtime/config.go -struct Config -output options_generated.go -package runtime
