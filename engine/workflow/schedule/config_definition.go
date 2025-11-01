package schedule

import projectschedule "github.com/compozy/compozy/engine/project/schedule"

// Config aliases the project-level schedule configuration so callers can continue importing
// definitions from the workflow schedule package without creating dependency cycles.
type Config = projectschedule.Config

// RetryPolicy aliases the project-level retry configuration for scheduled workflows.
type RetryPolicy = projectschedule.RetryPolicy
