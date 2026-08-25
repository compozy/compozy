package core

import (
	"context"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network/participation"
)

// CallsService is the daemon-owned public facade shared by HTTP and UDS.
type CallsService interface {
	Create(context.Context, callspkg.CreateInput) (callspkg.CallRecord, error)
	CreateBatch(context.Context, []callspkg.CreateInput) ([]callspkg.BatchOutcome, error)
	Return(context.Context, callspkg.ReturnInput) (callspkg.Settlement, error)
	List(context.Context, callspkg.CallListQuery) (callspkg.CallPage, error)
	GetRead(context.Context, callspkg.CallReadQuery, string) (callspkg.CallRecord, error)
	Result(context.Context, callspkg.CallReadQuery, string) (callspkg.ResultPayload, error)
	Await(context.Context, callspkg.AwaitInput) (callspkg.AwaitOutcome, error)
	Cancel(context.Context, string, string, callspkg.Actor) (callspkg.CallRecord, error)
	SendMessage(context.Context, callspkg.SendMessageInput) (callspkg.MessageRecord, error)
	Publish(context.Context, callspkg.PublishInput) (callspkg.PublishReceipt, error)
	Message(context.Context, callspkg.CallScope, string) (callspkg.MessageRecord, error)
	ListMessages(context.Context, callspkg.MessageListQuery) (callspkg.MessagePage, error)
	DrainSubtree(context.Context, string, callspkg.Actor, string) (callspkg.DrainReport, error)
	ResolveOperatorCaller(
		context.Context,
		callspkg.CallScope,
		callspkg.Actor,
	) (participation.OwnerRef, error)
}
