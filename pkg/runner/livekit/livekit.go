// Package livekit provides LiveKit room URL and agent token configuration from environment
// (aligned with Python pipecat/runner/livekit.py).
package livekit

import (
	"fmt"
	"os"
	"time"

	"github.com/livekit/protocol/auth"
)

// Config holds the result of configuring LiveKit (URL, agent token, room name).
type Config struct {
	URL      string
	Token    string
	RoomName string
}

// Configure returns LiveKit server URL, agent token, and room name from environment.
// Requires: LIVEKIT_URL, LIVEKIT_API_KEY, LIVEKIT_API_SECRET, LIVEKIT_ROOM_NAME.
func Configure() (*Config, error) {
	url := os.Getenv("LIVEKIT_URL")
	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")
	roomName := os.Getenv("LIVEKIT_ROOM_NAME")
	if roomName == "" {
		return nil, fmt.Errorf("LIVEKIT_ROOM_NAME is required (use -r/--room or set env)")
	}
	if url == "" {
		return nil, fmt.Errorf("LIVEKIT_URL is required (use -u/--url or set env)")
	}
	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("LIVEKIT_API_KEY and LIVEKIT_API_SECRET are required")
	}
	token, err := generateAgentToken(roomName, "Pipecat Agent", apiKey, apiSecret)
	if err != nil {
		return nil, err
	}
	return &Config{URL: url, Token: token, RoomName: roomName}, nil
}

// generateAgentToken creates a JWT for an agent to join the room (room_join + agent grant).
func generateAgentToken(roomName, participantName, apiKey, apiSecret string) (string, error) {
	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
		Agent:    true,
	}
	at.SetVideoGrant(grant).
		SetIdentity(participantName).
		SetName(participantName).
		SetValidFor(24 * time.Hour)
	return at.ToJWT()
}
