// Package whatsapp provides WhatsApp Cloud API client and transport for Voxray.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const graphAPIBase = "https://graph.facebook.com/v18.0"

// Client is a WhatsApp Cloud API client for sending messages.
type Client struct {
	httpClient   *http.Client
	accessToken  string
	phoneID      string
}

// NewClient creates a WhatsApp Cloud API client. phoneID is the WhatsApp Business phone number ID.
func NewClient(accessToken, phoneID string) *Client {
	return &Client{
		httpClient:  http.DefaultClient,
		accessToken: accessToken,
		phoneID:     phoneID,
	}
}

// SendText sends a text message to the given recipient (E.164 format, e.g. 15551234567).
func (c *Client) SendText(ctx context.Context, to, text string) error {
	body := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	return c.send(ctx, body)
}

// send posts the JSON body to the messages endpoint.
func (c *Client) send(ctx context.Context, body map[string]any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s/messages", graphAPIBase, c.phoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("whatsapp api: %s", resp.Status)
	}
	return nil
}
