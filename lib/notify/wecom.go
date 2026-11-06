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

type wecomChannel struct {
	webhook string
	client  *http.Client
}

func newWecomChannel(webhook string, client *http.Client) *wecomChannel {
	return &wecomChannel{webhook: strings.TrimSpace(webhook), client: client}
}

func (c *wecomChannel) Name() string { return "wecom" }

func (c *wecomChannel) Send(ctx context.Context, e Event) error {
	if c == nil || c.webhook == "" {
		return nil
	}
	content := strings.TrimSpace(e.Title)
	if e.Body != "" {
		if content != "" {
			content += "\n"
		}
		content += e.Body
	}
	payload, err := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhook, bytes.NewReader(payload))
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
		return fmt.Errorf("wecom status %d", resp.StatusCode)
	}
	return nil
}
