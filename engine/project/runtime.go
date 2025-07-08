package project

// RuntimeConfig defines the JavaScript runtime configuration for tool execution.
//
// **Tools** extend Compozy's capabilities by providing custom functions that agents can call.
// This configuration controls:
//   - Which JavaScript runtime executes the tools
//   - Security permissions and sandboxing
//   - Tool discovery and loading
//
// **Security Note**: These settings directly impact the security posture of tool execution.
// Always follow the principle of least privilege when configuring permissions.
//
// Example runtime configurations:
//
//	# Minimal permissions (recommended for most use cases)
//	runtime:
//	  type: bun
//	  entrypoint: ./tools.ts
//	  permissions:
//	    - --allow-read
//	    - --allow-net
//	    - --allow-env
type RuntimeConfig struct {
	// Type specifies the JavaScript runtime to use for tool execution.
	//
	// Valid values:
	//   - **"bun"** (default): High-performance runtime with built-in TypeScript support
	//   - ~**"node"**: Traditional Node.js runtime for broader compatibility~ (not supported yet)
	Type string `json:"type,omitempty" yaml:"type,omitempty" mapstructure:"type"`

	// Entrypoint specifies the path to the JavaScript/TypeScript file that exports all available tools.
	//
	// This file serves as the single entry point for tool discovery and execution.
	// Path specifications:
	//   - **Relative to project root**: `"./tools.ts"`, `"./src/tools/index.ts"`
	//   - **Required extensions**: `.ts` (TypeScript) or `.js` (JavaScript)
	//   - **Default**: `"./tools.ts"` (if not specified)
	//
	// Example entrypoint structure:
	//
	// ```ts
	//	// tools.ts
	//	export async function fetchWeatherData(params: { city: string }) {
	//	  // Tool implementation
	//	}
	//
	//	export async function analyzeData(params: { data: any[] }) {
	//	  // Tool implementation
	//	}
	//```
	//
	// **Security**: Must be a trusted file as it has access to all tool implementations
	Entrypoint string `json:"entrypoint" yaml:"entrypoint" mapstructure:"entrypoint"`

	// Permissions defines the security permissions granted to the runtime during tool execution.
	//
	// These permissions control what system resources tools can access, implementing
	// defense-in-depth security through capability-based access control.
	//
	// **For Bun runtime** - Comprehensive permission flags:
	//   - `--allow-read`: Read file system access (default)
	//   - `--allow-read=/specific/path`: Scoped read access
	//   - `--allow-write`: Write file system access
	//   - `--allow-write=/specific/path`: Scoped write access
	//   - `--allow-net`: Network access (HTTP/HTTPS requests)
	//   - `--allow-net=api.example.com`: Scoped network access
	//   - `--allow-env`: Environment variable access
	//   - `--allow-env=API_KEY,DATABASE_URL`: Scoped env access
	//   - `--allow-sys`: System information access
	//   - `--allow-ffi`: Foreign Function Interface access
	//   - `--allow-run`: Subprocess execution (use with extreme caution)
	//
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty" mapstructure:"permissions"`
}
