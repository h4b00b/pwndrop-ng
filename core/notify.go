package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
)

// notifyClient is shared by all sinks. Short timeout so a stuck webhook can't
// pin a goroutine for long.
var notifyClient = &http.Client{Timeout: 5 * time.Second}

// Bounded worker pool for outbound notifications. A burst of downloads against
// a slow webhook (5s timeout x 3 sinks) would otherwise spawn one goroutine
// per event indefinitely, retaining the captured cfg/event and exhausting
// memory + sockets. When the queue is full we drop the event with a warning
// rather than block the download request path.
const (
	notifyQueueSize   = 256
	notifyWorkerCount = 4
)

type notifyJob struct {
	cfg *storage.DbConfig
	ev  *storage.DbDownloadLog
}

var (
	notifyQueue    chan notifyJob
	notifyInitOnce sync.Once
)

func notifyInit() {
	notifyQueue = make(chan notifyJob, notifyQueueSize)
	for i := 0; i < notifyWorkerCount; i++ {
		go notifyWorker()
	}
}

func notifyWorker() {
	for j := range notifyQueue {
		dispatchNotifications(j.cfg, j.ev)
	}
}

// LogDownload persists a download event and fans out notifications. Always
// runs the notification dispatch in a goroutine — the caller is on the
// download request path and must not block on outbound HTTP.
//
// muted=true suppresses the outbound dispatch but still persists the log
// entry; used when the file has NotifyMuted set.
func LogDownload(ev *storage.DbDownloadLog, muted bool) {
	ev.Timestamp = time.Now().Unix()
	if _, err := storage.DownloadLogCreate(ev); err != nil {
		log.Error("download log: %s", err)
	}

	cfg, err := storage.ConfigGet(1)
	if err != nil || !cfg.NotifyEnabled || muted {
		return
	}
	// Operator-level status allowlist. Empty list = notify on everything.
	if len(cfg.NotifyStatusFilter) > 0 {
		ok := false
		for _, s := range cfg.NotifyStatusFilter {
			if s == ev.Status {
				ok = true
				break
			}
		}
		if !ok {
			return
		}
	}

	notifyInitOnce.Do(notifyInit)
	select {
	case notifyQueue <- notifyJob{cfg: cfg, ev: ev}:
	default:
		log.Warning("notify: queue full (%d), dropping event for %s", notifyQueueSize, ev.UrlPath)
	}
}

func dispatchNotifications(cfg *storage.DbConfig, ev *storage.DbDownloadLog) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("notify: panic: %v", r)
		}
	}()

	if cfg.NotifyWebhookUrl != "" {
		if err := notifyWebhook(cfg.NotifyWebhookUrl, ev); err != nil {
			log.Error("notify webhook: %s", err)
		}
	}
	if cfg.NotifyTelegramToken != "" && cfg.NotifyTelegramChatId != "" {
		if err := notifyTelegram(cfg.NotifyTelegramToken, cfg.NotifyTelegramChatId, ev); err != nil {
			log.Error("notify telegram: %s", err)
		}
	}
	if cfg.NotifySlackWebhook != "" {
		if err := notifySlack(cfg.NotifySlackWebhook, ev); err != nil {
			log.Error("notify slack: %s", err)
		}
	}
}

// notifyWebhook POSTs the raw event as JSON. The remote endpoint is expected
// to know the pwndrop schema (status/file_name/url_path/remote_ip/user_agent/
// referer/timestamp) and can use any field it wants.
func notifyWebhook(webhookUrl string, ev *storage.DbDownloadLog) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", webhookUrl, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pwndrop-notify/1")
	resp, err := notifyClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

func notifyTelegram(token, chatId string, ev *storage.DbDownloadLog) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", url.PathEscape(token))
	payload := map[string]interface{}{
		"chat_id":    chatId,
		"text":       formatTelegramMessage(ev),
		"parse_mode": "Markdown",
	}
	// scrubTelegramErr below masks the token before any error reaches the log.
	if err := postJSON(endpoint, payload); err != nil {
		return scrubTelegramErr(err, token)
	}
	return nil
}

// scrubTelegramErr replaces the bot token in error strings (net/http includes
// the full URL in transport-level errors) so it never lands in the operator
// log file.
func scrubTelegramErr(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, token) {
		msg = strings.ReplaceAll(msg, token, "<redacted-token>")
	}
	enc := url.PathEscape(token)
	if enc != token && strings.Contains(msg, enc) {
		msg = strings.ReplaceAll(msg, enc, "<redacted-token>")
	}
	return fmt.Errorf("%s", msg)
}

func notifySlack(webhookUrl string, ev *storage.DbDownloadLog) error {
	payload := map[string]interface{}{
		"text": formatSlackMessage(ev),
	}
	return postJSON(webhookUrl, payload)
}

func postJSON(endpoint string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pwndrop-notify/1")
	resp, err := notifyClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}
	return nil
}

func formatTelegramMessage(ev *storage.DbDownloadLog) string {
	var b strings.Builder
	b.WriteString("*pwndrop download*\n")
	fmt.Fprintf(&b, "*file:* `%s`\n", escapeMarkdown(ev.FileName))
	fmt.Fprintf(&b, "*path:* `%s`\n", escapeMarkdown(ev.UrlPath))
	fmt.Fprintf(&b, "*ip:* `%s`\n", escapeMarkdown(ev.RemoteIp))
	if ev.UserAgent != "" {
		fmt.Fprintf(&b, "*ua:* `%s`\n", escapeMarkdown(ev.UserAgent))
	}
	if ev.Referer != "" {
		fmt.Fprintf(&b, "*ref:* `%s`\n", escapeMarkdown(ev.Referer))
	}
	fmt.Fprintf(&b, "*status:* %s", ev.Status)
	return b.String()
}

func formatSlackMessage(ev *storage.DbDownloadLog) string {
	var b strings.Builder
	b.WriteString(":inbox_tray: *pwndrop download*\n")
	fmt.Fprintf(&b, "• file: `%s`\n", escapeMarkdown(ev.FileName))
	fmt.Fprintf(&b, "• path: `%s`\n", escapeMarkdown(ev.UrlPath))
	fmt.Fprintf(&b, "• ip: `%s`\n", escapeMarkdown(ev.RemoteIp))
	if ev.UserAgent != "" {
		fmt.Fprintf(&b, "• ua: `%s`\n", escapeMarkdown(ev.UserAgent))
	}
	if ev.Referer != "" {
		fmt.Fprintf(&b, "• ref: `%s`\n", escapeMarkdown(ev.Referer))
	}
	fmt.Fprintf(&b, "• status: %s", ev.Status)
	return b.String()
}

// escapeMarkdown collapses backticks (the only char that can break a Markdown
// code span) — full MarkdownV2 escaping is overkill since values land inside
// code spans. Used for both Telegram and Slack since they share the same code-
// span syntax and the attacker-controlled fields (FileName/UrlPath/UserAgent/
// Referer) are the same in both paths.
func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "`", "'")
}
