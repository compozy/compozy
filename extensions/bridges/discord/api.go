package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/bridgesdk"
)

type discordAPI interface {
	GetBotUser(context.Context) (*discordBotIdentity, error)
	PostMessage(context.Context, discordPostMessageRequest) (*discordPostedMessage, error)
	UpdateMessage(context.Context, discordUpdateMessageRequest) error
	DeleteMessage(context.Context, discordDeleteMessageRequest) error
}

type discordProgressAPI interface {
	SetTyping(context.Context, discordTypingRequest) error
	AddReaction(context.Context, discordReactionRequest) error
}

type discordBotIdentity struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}

type discordPostedMessage struct {
	ID string `json:"id,omitempty"`
}

type discordPostMessageRequest struct {
	ChannelID string `json:"-"`
	Content   string `json:"content"`
}

type discordUpdateMessageRequest struct {
	ChannelID string `json:"-"`
	MessageID string `json:"-"`
	Content   string `json:"content"`
}

type discordDeleteMessageRequest struct {
	ChannelID string `json:"-"`
	MessageID string `json:"-"`
}

type discordTypingRequest struct {
	ChannelID string `json:"-"`
	Active    bool   `json:"-"`
}

type discordReactionRequest struct {
	ChannelID string `json:"-"`
	MessageID string `json:"-"`
	Reaction  string `json:"-"`
}

type discordBotClient struct {
	baseURL               string
	botToken              string
	httpClient            *http.Client
	reportResponseCleanup func(error)
}

var _ discordProgressAPI = (*discordBotClient)(nil)

func (c *discordBotClient) SetTyping(ctx context.Context, req discordTypingRequest) error {
	if !req.Active {
		return nil
	}
	return c.callJSON(
		ctx,
		http.MethodPost,
		"/channels/"+strings.TrimSpace(req.ChannelID)+"/typing",
		nil,
		nil,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	)
}

func (c *discordBotClient) AddReaction(ctx context.Context, req discordReactionRequest) error {
	return c.callJSON(
		ctx,
		http.MethodPut,
		"/channels/"+strings.TrimSpace(req.ChannelID)+
			"/messages/"+strings.TrimSpace(req.MessageID)+
			"/reactions/"+url.PathEscape(strings.TrimSpace(req.Reaction))+"/@me",
		nil,
		nil,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	)
}
