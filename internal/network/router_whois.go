package network

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type generatedWhoisResponse struct {
	senderSessionID string
	prepared        SendResult
}

func snapshotWhoisResponders(
	envelope Envelope,
	locals []LocalPeer,
	selected map[string]bool,
) ([]LocalPeer, error) {
	if envelope.Kind != KindWhois {
		return nil, nil
	}
	body, err := envelope.DecodeBody()
	if err != nil {
		return nil, err
	}
	whois, ok := body.(WhoisBody)
	if !ok {
		return nil, fmt.Errorf("network: unexpected whois body type %T", body)
	}
	if whois.Type != WhoisTypeRequest {
		return nil, nil
	}

	responders := make([]LocalPeer, 0, len(selected))
	for _, peer := range locals {
		if isEnvelopeSender(peer, envelope) || !selected[peer.SessionID] {
			continue
		}
		if !envelope.IsDirected() && !matchesWhoisQuery(peer.PeerCard, whois.Query) {
			continue
		}
		responders = append(responders, cloneLocalPeer(peer))
	}
	return responders, nil
}

func matchesWhoisQuery(card PeerCard, query string) bool {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return true
	}
	if card.PeerID == trimmedQuery {
		return true
	}
	if card.DisplayName != nil && strings.TrimSpace(*card.DisplayName) == trimmedQuery {
		return true
	}
	return containsString(card.Capabilities, trimmedQuery) ||
		containsString(card.ProfilesSupported, trimmedQuery) ||
		containsString(card.ArtifactsSupported, trimmedQuery) ||
		containsString(card.TrustModesSupported, trimmedQuery)
}

func (r *Router) buildWhoisResponses(route RouteResult) ([]generatedWhoisResponse, error) {
	if len(route.whoisPeers) == 0 {
		return nil, nil
	}
	discovery := parseWhoisCapabilityDiscoveryRequest(route.Envelope.Ext)
	responses := make([]generatedWhoisResponse, 0, len(route.whoisPeers))
	for _, responder := range route.whoisPeers {
		envelope, err := r.buildWhoisResponseEnvelope(route.Envelope, responder, discovery)
		if err != nil {
			return nil, err
		}
		responses = append(responses, generatedWhoisResponse{
			senderSessionID: responder.SessionID,
			prepared:        SendResult{ID: envelope.ID, Envelope: envelope},
		})
	}
	return responses, nil
}

func (r *Router) buildWhoisResponseEnvelope(
	request Envelope,
	responder LocalPeer,
	discovery whoisCapabilityDiscoveryRequest,
) (Envelope, error) {
	responseCard := clonePeerCard(responder.PeerCard)
	if len(responder.CapabilityCatalog) != 0 {
		responseCatalog := responder.CapabilityCatalog
		if discovery.includeCapabilityCatalog {
			responseCatalog = selectWhoisCapabilityCatalog(
				responder.CapabilityCatalog,
				discovery.capabilityIDs,
			)
		}
		if err := applyCapabilityBriefProjection(&responseCard, responseCatalog); err != nil {
			return Envelope{}, err
		}
	}
	body, err := json.Marshal(WhoisBody{Type: WhoisTypeResponse, PeerCard: &responseCard})
	if err != nil {
		return Envelope{}, fmt.Errorf("network: marshal whois response body: %w", err)
	}
	ext, err := buildWhoisCapabilityCatalogResponseExt(discovery, responder.CapabilityCatalog)
	if err != nil {
		return Envelope{}, err
	}
	to := request.From
	replyTo := request.ID
	response := Envelope{
		Protocol: ProtocolV0, ID: whoisResponseID(request, responder),
		WorkspaceID: request.WorkspaceID, Kind: KindWhois, Channel: request.Channel,
		From: responder.PeerID, To: &to, ReplyTo: &replyTo, TS: request.TS,
		Body: body, Ext: ext,
	}
	normalized, err := NormalizeEnvelope(response, ValidateOptions{
		Now: r.now().UTC(), MaxReplayAge: r.maxReplayAge,
	})
	if err != nil {
		return Envelope{}, err
	}
	return normalized, nil
}

func whoisResponseID(request Envelope, responder LocalPeer) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"network-whois-response-v1",
		request.WorkspaceID,
		request.Channel,
		request.ID,
		responder.SessionID,
	}, "\x00")))
	return "msg-" + hex.EncodeToString(digest[:16])
}
