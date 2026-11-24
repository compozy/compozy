package admission

import "errors"

const LocalExecutionDisabledCode = "local_execution_disabled"
const LocalExecutionDisabledMessage = "Running agents on this computer is turned off."

var ErrLocalExecutionDisabled = errors.New("admission: local execution disabled")
