package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type telegramChannel struct {
	token  string
	chatID string
	client *http.Client
}

func newTelegramChannel(token, chatID string, client *http.Client) *telegramChannel {
	return &telegramChannel{
		token:  strings.TrimSpace(token),
		chatID: strings.TrimSpace(chatID),
		client: client,
	}
}

func (c *telegramChannel) Name() string { return "telegram" }

func (c *telegramChannel) Send(ctx context.Context, e Event) error {
	if c == nil || c.token == "" || c.chatID == "" {
		return nil
	}
	text := strings.TrimSpace(e.Title)
	if e.Body != "" {
		if text != "" {
			text += "\n"
		}
		text += e.Body
	}
	payload, err := json.Marshal(map[string]interface{}{
		"chat_id":                  c.chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return err
	}
	url := "https://api.telegram.org/bot" + c.token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram status %d", resp.StatusCode)
	}
	return nil
}
