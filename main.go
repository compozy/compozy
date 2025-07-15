//	@title			Compozy API
//	@version		1.0
//	@description	Compozy is a workflow orchestration engine for AI agents, tasks, and tools
//	@termsOfService	https://github.com/compozy/compozy

//	@contact.name	Compozy Support
//	@contact.url	https://github.com/compozy/compozy
//	@contact.email	support@compozy.dev

//	@license.name	MIT
//	@license.url	https://github.com/compozy/compozy/blob/main/LICENSE

//	@BasePath	/api/v0

//	@tag.name			workflows
//	@tag.description	Workflow management operations

//	@tag.name			tasks
//	@tag.description	Task management operations

//	@tag.name			agents
//	@tag.description	Agent management operations

//	@tag.name			tools
//	@tag.description	Tool management operations

//	@tag.name			schedules
//	@tag.description	Schedule management operations

//	@tag.name			memory
//	@tag.description	Memory management operations

//	@tag.name			auth
//	@tag.description	Authentication and API key management operations

//	@tag.name			users
//	@tag.description	User management operations (admin only)

//	@tag.name			Operations
//	@tag.description	Operational endpoints for monitoring and health

package main

import (
	"os"

	"github.com/compozy/compozy/cli"
	_ "github.com/compozy/compozy/engine/auth/router"              // Import for swagger docs
	_ "github.com/compozy/compozy/engine/memory/router"            // Import for swagger docs
	_ "github.com/compozy/compozy/engine/workflow/schedule/router" // Import for swagger docs
)

func main() {
	cmd := cli.RootCmd()
	if err := cmd.Execute(); err != nil {
		// Exit with error code 1 if command execution fails
		os.Exit(1)
	}
}
