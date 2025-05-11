package core

import "fmt"

type ErrorResponse struct {
	Code     string  `json:"code"`
	Msg      string  `json:"name"`
	Err      error   `json:"error"`
	Metadata StateID `json:"metadata"`
}

func (e ErrorResponse) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s - %s: %s (%s)", e.Metadata, e.Code, e.Msg, e.Err.Error())
	}
	return fmt.Sprintf("%s - %s: %s", e.Metadata, e.Code, e.Msg)
}

func NewError(id *StateID, code string, msg string, err error) ErrorResponse {
	return ErrorResponse{
		Code:     code,
		Msg:      msg,
		Err:      err,
		Metadata: *id,
	}
}
