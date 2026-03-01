// Package daily provides Daily.co room and meeting token creation via the REST API
// (aligned with Python pipecat/runner/daily.py).
package daily

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

const defaultAPIURL = "https://api.daily.co/v1"

// Config holds the result of configuring a Daily room (room URL, token, optional SIP endpoint).
type Config struct {
	RoomURL    string
	Token      string
	SIPEndpoint string
}

// Options configures room and token creation.
type Options struct {
	APIKey             string
	APIURL             string
	RoomExpDurationHrs float64
	TokenExpDurationHrs float64
	// SIP
	SIPCallerPhone string
	SIPEnableVideo bool
	SIPNumEndpoints int
}

// Configure creates a Daily room and meeting token. When SIPCallerPhone is set, the room is created with SIP (dial-in) and SIPEndpoint is set on the result.
func Configure(ctx context.Context, opts Options) (*Config, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DAILY_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("DAILY_API_KEY is required (get one from https://dashboard.daily.co/developers)")
	}
	apiURL := opts.APIURL
	if apiURL == "" {
		apiURL = os.Getenv("DAILY_API_URL")
	}
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	roomExpHrs := opts.RoomExpDurationHrs
	if roomExpHrs <= 0 {
		roomExpHrs = 2
	}
	tokenExpHrs := opts.TokenExpDurationHrs
	if tokenExpHrs <= 0 {
		tokenExpHrs = 2
	}
	expiry := time.Now().Add(time.Duration(roomExpHrs * float64(time.Hour))).Unix()
	tokenExpiry := time.Now().Add(time.Duration(tokenExpHrs * float64(time.Hour))).Unix()

	roomName := "pipecat-" + uuid.New().String()[:8]
	if opts.SIPCallerPhone != "" {
		roomName = "pipecat-sip-" + uuid.New().String()[:8]
	}

	// Build room properties
	roomProps := map[string]interface{}{
		"exp":               expiry,
		"eject_at_room_exp": true,
	}
	if opts.SIPCallerPhone != "" {
		roomProps["sip"] = map[string]interface{}{
			"display_name":  opts.SIPCallerPhone,
			"video":         opts.SIPEnableVideo,
			"sip_mode":      "dial-in",
			"num_endpoints": opts.SIPNumEndpoints,
		}
		roomProps["enable_dialout"] = true
		roomProps["start_video_off"] = !opts.SIPEnableVideo
	}

	roomBody := map[string]interface{}{
		"name":       roomName,
		"properties": roomProps,
	}
	roomJSON, _ := json.Marshal(roomBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/rooms", bytes.NewReader(roomJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("daily create room: %s", resp.Status)
	}
	var roomResp struct {
		URL    string `json:"url"`
		Name   string `json:"name"`
		Config struct {
			SIPEndpoint string `json:"sip_endpoint"`
		} `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&roomResp); err != nil {
		return nil, err
	}
	if roomResp.URL == "" {
		return nil, fmt.Errorf("daily create room: empty url in response")
	}

	// Create meeting token
	tokenBody := map[string]interface{}{
		"properties": map[string]interface{}{
			"room_name": roomName,
			"exp":       tokenExpiry,
			"is_owner":  true,
			"user_name": "Pipecat Bot",
		},
	}
	tokenJSON, _ := json.Marshal(tokenBody)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/meeting-tokens", bytes.NewReader(tokenJSON))
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+apiKey)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("daily create token: %s", resp2.Status)
	}
	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.Token == "" {
		return nil, fmt.Errorf("daily create token: empty token in response")
	}

	out := &Config{RoomURL: roomResp.URL, Token: tokenResp.Token}
	if roomResp.Config.SIPEndpoint != "" {
		out.SIPEndpoint = roomResp.Config.SIPEndpoint
	}
	return out, nil
}
