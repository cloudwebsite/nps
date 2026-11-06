package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type webhookChannel struct {
	url     string
	headers map[string]string
	client  *http.Client
}

func newWebhookChannel(url, headerRaw string, client *http.Client) *webhookChannel {
	return &webhookChannel{
		url:     strings.TrimSpace(url),
		headers: parseWebhookHeaders(headerRaw),
		client:  client,
	}
}

func parseWebhookHeaders(raw string) map[string]string {
	raw = strings.ReplaceAll(raw, `\n`, "\n")
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func (c *webhookChannel) Name() string { return "webhook" }

type webhookPayload struct {
	Version  int               `json:"version"`
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Time     string            `json:"time"`
	ClientID int               `json:"client_id"`
	Host     string            `json:"host"`
	Target   string            `json:"target"`
	Addr     string            `json:"addr"`
	Details  map[string]string `json:"details"`
}

func (c *webhookChannel) Send(ctx context.Context, e Event) error {
	if c == nil || c.url == "" {
		return nil
	}
	details := e.Details
	if details == nil {
		details = map[string]string{}
	}
	ts := e.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	payload, err := json.Marshal(webhookPayload{
		Version:  1,
		Type:     e.Type,
		Title:    e.Title,
		Body:     e.Body,
		Time:     ts.Format(time.RFC3339),
		ClientID: e.ClientID,
		Host:     e.Host,
		Target:   e.Target,
		Addr:     e.Addr,
		Details:  details,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}
