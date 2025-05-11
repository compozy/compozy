package core

import "fmt"

type ErrorResponse struct {
	Location string `json:"location"`
	Code     string `json:"code"`
	Msg      string `json:"name"`
	Err      error  `json:"error"`
}

func (e ErrorResponse) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s - %s: %s (%s)", e.Location, e.Code, e.Msg, e.Err.Error())
	}
	return fmt.Sprintf("%s - %s: %s", e.Location, e.Code, e.Msg)
}

func NewWorkflowError(id string, code string, msg string, err error) ErrorResponse {
	return ErrorResponse{
		Location: fmt.Sprintf("workflow:%s", id),
		Code:     code,
		Msg:      msg,
		Err:      err,
	}
}

func NewTaskError(id string, code string, msg string, err error) ErrorResponse {
	return ErrorResponse{
		Location: fmt.Sprintf("task:%s", id),
		Code:     code,
		Msg:      msg,
		Err:      err,
	}
}

func NewAgentError(id string, code string, msg string, err error) ErrorResponse {
	return ErrorResponse{
		Location: fmt.Sprintf("agent:%s", id),
		Code:     code,
		Msg:      msg,
		Err:      err,
	}
}

func NewToolError(id string, code string, msg string, err error) ErrorResponse {
	return ErrorResponse{
		Location: fmt.Sprintf("tool:%s", id),
		Code:     code,
		Msg:      msg,
		Err:      err,
	}
}
