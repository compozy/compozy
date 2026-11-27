package cloudclient

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const computerReservationsPath = "/v1/computers/reservations"

type ComputerReservation struct {
	ID         string                 `json:"reservation_id"`
	ExpiresAt  time.Time              `json:"expires_at"`
	ComputerID remoteproto.ComputerID `json:"computer_id,omitempty"`
}

func (c *Client) ReserveComputer(
	ctx context.Context, creds Credentials, previous string,
) (ComputerReservation, error) {
	body := struct {
		Previous string `json:"previous_reservation_id,omitempty"`
	}{previous}
	var result ComputerReservation
	err := c.deviceJSON(ctx, creds, http.MethodPost, computerReservationsPath, body, &result)
	return result, err
}

func (c *Client) ReleaseComputerReservation(ctx context.Context, creds Credentials, id string) error {
	return c.deviceJSON(ctx, creds, http.MethodDelete, computerReservationsPath+"/"+url.PathEscape(id), nil, nil)
}

type ComputerLinkToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (c *Client) NewComputerLinkToken(
	ctx context.Context, creds Credentials, reservation string,
) (ComputerLinkToken, error) {
	body := struct {
		ReservationID string `json:"reservation_id"`
	}{reservation}
	var result ComputerLinkToken
	err := c.deviceJSON(ctx, creds, http.MethodPost, "/v1/computers/link-tokens", body, &result)
	return result, err
}
