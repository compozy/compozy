package admission

import "errors"

const LocalExecutionDisabledCode = "local_execution_disabled"

var ErrLocalExecutionDisabled = errors.New("Running agents on this computer is turned off.")
