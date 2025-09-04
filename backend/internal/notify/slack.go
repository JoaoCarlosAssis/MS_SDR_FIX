package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	last = map[string]time.Time{}
)

// PostSlack envia uma mensagem para um Incoming Webhook do Slack.
// Aplica antirruído simples: não envia a mesma mensagem mais de 1/min.
func PostSlack(ctx context.Context, webhookURL, text string) error {
	if webhookURL == "" || text == "" {
		return nil
	}

	key := text
	mu.Lock()
	if t, ok := last[key]; ok && time.Since(t) < time.Minute {
		mu.Unlock()
		return nil
	}
	last[key] = time.Now()
	mu.Unlock()

	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}

// PostSlackWithBot envia mensagem usando token de bot (chat.postMessage)
// Requer SLACK_BOT_TOKEN com escopo chat:write e SLACK_CHANNEL_ID (ex.: C01234567)
func PostSlackWithBot(ctx context.Context, botToken, channelID, text string) error {
	if botToken == "" || channelID == "" || text == "" {
		return nil
	}

	key := "bot:" + channelID + ":" + text
	mu.Lock()
	if t, ok := last[key]; ok && time.Since(t) < time.Minute {
		mu.Unlock()
		return nil
	}
	last[key] = time.Now()
	mu.Unlock()

	body, _ := json.Marshal(map[string]any{
		"channel": channelID,
		"text":    text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+botToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack chat.postMessage status %d", resp.StatusCode)
	}
	return nil
}
