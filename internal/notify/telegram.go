// Package notify delivers best-effort operational notifications.
package notify

import (
	"log"
	"net/http"
	"net/url"
	"time"
)

// NewTelegram returns a notifier that posts HTML messages to a Telegram chat
// through the bot API. Without a token and chat ID configured it returns a
// no-op, so dev/local deployments stay quiet without special-casing callers.
// Notifications are advisory: failures are logged, never surfaced to requests.
func NewTelegram(token, chatID string) func(string) {
	if token == "" || chatID == "" {
		return func(string) {}
	}

	client := &http.Client{Timeout: 3 * time.Second}
	endpoint := "https://api.telegram.org/bot" + token + "/sendMessage"

	return func(text string) {
		form := url.Values{
			"chat_id":                  {chatID},
			"text":                     {text},
			"parse_mode":               {"HTML"},
			"disable_web_page_preview": {"true"},
		}
		resp, err := client.PostForm(endpoint, form)
		if err != nil {
			log.Printf("telegram notify: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("telegram notify: status %d", resp.StatusCode)
		}
	}
}
