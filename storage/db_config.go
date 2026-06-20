package storage

type DbConfig struct {
	ID          int    `json:"id" storm:"id"`
	Hostname    string `json:"hostname"`
	SecretPath  string `json:"secret_path"`
	RedirectUrl string `json:"redirect_url"`
	CookieName  string `json:"cookie_name"`
	CookieToken string `json:"cookie_token"`

	// Download notification settings. NotifyEnabled is the master switch; any
	// individual sink (webhook/Telegram/Slack) is dispatched only if its URL/
	// token fields are non-empty.
	NotifyEnabled        bool   `json:"notify_enabled"`
	NotifyWebhookUrl     string `json:"notify_webhook_url"`
	NotifyTelegramToken  string `json:"notify_telegram_token"`
	NotifyTelegramChatId string `json:"notify_telegram_chat_id"`
	NotifySlackWebhook   string `json:"notify_slack_webhook"`

	// Target-filter master switch + fallback when no rule in the chain matches.
	// FiltersDefaultAction values mirror DbFilter.Action ("allow"/"deny"/
	// "facade"/"redirect"); empty string is treated as "allow".
	FiltersEnabled       bool   `json:"filters_enabled"`
	FiltersDefaultAction string `json:"filters_default_action"`

	// KillSwitch: when true, every download returns 404 without entering the
	// per-file flow at all. The "oh-shit" button.
	KillSwitch bool `json:"kill_switch"`

	// NotifyStatusFilter narrows which download events trigger an outbound
	// notification. Empty = notify on every status (current behavior). Set to
	// e.g. ["ok","paused-facade"] to skip noise from scanner-blocking events.
	NotifyStatusFilter []string `json:"notify_status_filter"`

	// Auto-cleanup. CleanupExpiredAfterDays>0 means files whose ExpireAt is
	// older than N days will be deleted; CleanupLogMaxEntries>0 caps the
	// download log to the most recent N entries. The tick runs hourly.
	CleanupEnabled          bool `json:"cleanup_enabled"`
	CleanupExpiredAfterDays int  `json:"cleanup_expired_after_days"`
	CleanupLogMaxEntries    int  `json:"cleanup_log_max_entries"`

	// TrustProxy: when true, the request's X-Forwarded-For (first hop) or
	// X-Real-IP is used as the client IP for filters, blacklist, login
	// throttling, and download logs. Must be left false when pwndrop is
	// exposed directly to the internet — otherwise any client can spoof its
	// IP via a header.
	TrustProxy bool `json:"trust_proxy"`

	// GeoIpEndpoint is the URL template used to resolve a country code for
	// "country" filter rules. Must contain a single "%s" placeholder for the
	// IP. Empty = the built-in default (HTTPS, free, no key). Override to use
	// a paid/self-hosted provider for OPSEC (the default still leaks visitor
	// IPs to a third party — just over TLS).
	GeoIpEndpoint string `json:"geoip_endpoint"`
}

func ConfigCreate(o *DbConfig) (*DbConfig, error) {
	err := db.Save(o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func ConfigGet(id int) (*DbConfig, error) {
	var o DbConfig
	err := db.One("ID", id, &o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func ConfigUpdate(id int, o *DbConfig) (*DbConfig, error) {
	o.ID = id
	if err := db.Save(o); err != nil {
		return nil, err
	}
	return o, nil
}

