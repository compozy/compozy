package common

import "github.com/google/uuid"

type CorrID string

func (c CorrID) String() string {
	return string(c)
}

func NewCorrID() CorrID {
	return CorrID(uuid.New().String())
}

type ExecID string

func (e ExecID) String() string {
	return string(e)
}

func NewExecID() ExecID {
	return ExecID(uuid.New().String())
}

type EventID string

func (e EventID) String() string {
	return string(e)
}

func NewEventID() EventID {
	return EventID(uuid.New().String())
}

type RequestID string

func (r RequestID) String() string {
	return string(r)
}

func NewRequestID() RequestID {
	return RequestID(uuid.New().String())
}

type CompID string

func (c CompID) String() string {
	return string(c)
}
