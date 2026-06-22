****v2.0.4****
- [x] Admin page footer now also credits the NG fork (`made by @mrgretzky · NG fork by @h4b00b`) in `frontend/src/App.vue`; original upstream credit preserved. Frontend bundle rebuilt by CI.
- [x] Version constant bumped to `2.0.4`; frontend `package.json` synced.

****v2.0.3****
- [x] Renamed the build target and installed daemon from `pwndrop` to `pwndrop-ng` across `Makefile`, `build.sh`, the CI workflow, `install_linux.sh`, and the embedded constants (`SERVICE_NAME`, `INSTALL_DIR=/usr/local/pwndrop-ng`, `EXEC_NAME=pwndrop-ng`, `usage:` banner). Release artifacts are now published as `pwndrop-ng-linux-<arch>.tar.gz` and the oneliner / `./pwndrop-ng <cmd>` flow follows.
- [x] **Upgrade note:** an existing v2.0.2 deployment must be torn down before installing v2.0.3 — run `./pwndrop remove` against the old binary first, then re-install with the new oneliner. The config filename (`pwndrop.ini`) is unchanged, but it now lives under `/usr/local/pwndrop-ng/` instead of `/usr/local/pwndrop/`; move it across manually if you want to preserve existing settings (`data/pwndrop.db` carries the actual state — back that up the same way).
- [x] Version constant bumped to `2.0.3`; frontend `package.json` synced.

****v2.0.2****
- [x] Over-engineering / dead-code sweep across the Go side: removed unused exported funcs (`SessionGet`, `UserGet`, `ConfigDelete`, `DownloadLogDeleteOlderThan`, `SetManagedHostnames`, `NullLogger`, `EnableOutput`, `SetOutput`, `GenRandomUint64`, `handleNotFound`, `pdom`), the unused `API_ERROR_PASSWORDS_DONT_MATCH` constant, the dead `stderr` / `enableOutput` package vars, the commented-out cipher-suite block in `core/server.go` and the commented `FileList` loop in `storage/db_file.go`.
- [x] Migrated off the deprecated `io/ioutil` package: `ioutil.Discard` → `io.Discard`, `ioutil.ReadFile` → `os.ReadFile` (`log/log.go`, `core/gen_cert.go`).
- [x] Minor shrinks: deduped a `Cfg.GetListenIP()` call in the DNS handler, dropped a commented `gorilla/mux` import and a dead `_ = parent_file` placeholder in the sub-file upload handler.
- [x] README rewritten so all upstream-specific bits (logo/title/demo asset URLs, oneliner, binary releases link, source `git clone`, Go 1.13, the `breakdev.org`/Digital-Ocean referral framing, the personal "Credits" thanks) point at `h4b00b/pwndrop-ng` or are clearly framed as upstream resources. `python -m SimpleHTTPServer` updated to Python 3. Build instructions now mention Go 1.25+, Node 22+, and the `make build-amd64 / build-arm64 / build-all` cross-compile targets.
- [x] `install_linux.sh` now pulls release artifacts from `github.com/h4b00b/pwndrop-ng/releases/latest/download/` instead of upstream.
- [x] Version constant bumped to `2.0.2`; frontend `package.json` synced to match.

****v2.0.1****
- [x] Rebranded the project to **pwndrop NG**: frontend title, console banner, package name (`pwndrop-ng-frontend`), and topbar alt text now say "pwndrop NG". License unchanged (GPL3), original author credits preserved.
- [x] Version constant bumped to `2.0.1`.

****v2.0.0****
- [x] Login throttling: 5 failed attempts per source IP within 5 minutes triggers a 15-minute block, with a `Retry-After` header on subsequent attempts. Successful login clears the counter for that IP.
- [x] Login response is now a generic `invalid credentials` and unconditionally runs bcrypt against a pre-computed dummy hash when the user does not exist, so username enumeration is no longer possible via response message or via timing.
- [x] Authentication cookies are now set with `HttpOnly` + `SameSite=Lax`, and with `Secure` when the request arrived over TLS. The secret-path cookie picks up the same flags.
- [x] State-changing endpoints `/files/{id}/enable|disable|pause|unpause` migrated from `GET` to `POST` so a cross-origin click cannot toggle file state under a Lax-cookie browser.
- [x] `PUT /api/v1/files/{id}` no longer panics when the client sends an empty `url_path`; the request is now rejected with a 400.
- [x] Client IP parsing rewritten via `net.SplitHostPort`, fixing every IP/CIDR filter and the blacklist for IPv6 clients (the previous naive split on `:` mangled `[::1]:42` into `[`).
- [x] New `trust_proxy` config flag (off by default). When enabled, `X-Forwarded-For` / `X-Real-IP` is honored as the client IP for filters, blacklist, login throttle, and logs. Exposed as a "Trust X-Forwarded-For" checkbox in Settings → Network.
- [x] GeoIP lookups now use HTTPS by default (`https://ipwho.is/{ip}`). The previous `http://ip-api.com` endpoint leaked every visitor IP in plaintext and let an on-path attacker forge the country code. Endpoint is configurable via the new `geoip_endpoint` config field; the decoder accepts both `country_code` and legacy `countryCode` shapes.
- [x] GeoIP cache is now capped at 10 000 entries with on-the-fly eviction of expired and overflow records, so a high-cardinality scanner can no longer grow the cache without bound.
- [x] TOCTOU windows on `max_downloads` and `burn_after_read` closed via a per-file mutex. Concurrent requests against the same file are now serialized around the (re-read state, validate quota, write body, increment counter, burn) sequence, so the quota can no longer be overshot and a burn-after-read file can no longer be served twice.
- [x] Slack download notifications now escape backticks in FileName / UrlPath / RemoteIp / UserAgent / Referer (matching Telegram), so an attacker-controlled value cannot break out of the Markdown code-span.
- [x] All public-URL filename segments (upload, paste, sub-file, replace) are sanitized through `utils.SanitizeUrlSegment`, stripping `/ \ ? #`, control characters and trim-only whitespace, with a `"file"` fallback when the result is empty.
- [x] Request body size limits added across all JSON handlers (1 MiB) and the paste endpoint (50 MiB) via `http.MaxBytesReader`, capping memory pressure from a misbehaving authenticated client.
- [x] Empty Bearer tokens are explicitly rejected before any DB lookup.
- [x] Served files and facade responses now carry `X-Content-Type-Options: nosniff` so the browser sticks to the operator-declared MIME and does not sniff-promote a blob.
- [x] First-account bootstrap (the only unauthenticated user-creation path) now logs a warning with username, source IP and User-Agent so the operator can spot a race for the initial admin.
- [x] Soft 404s (file disabled, expired, exhausted, filter-deny, password mismatch, etc.) now return a plain `HTTP/1.1 404 Not Found` instead of a TCP hijack-and-close. The hijack is retained only for the kill switch and the IP blacklist, where actively tearing the TCP is the point.

****v1.7.0****
- [x] Frontend rewritten as a Vue 3 single-page application built with Vite; output is embedded into the binary via `go:embed` so deployment remains a single static file with no companion `admin/` directory.
- [x] Backend toolchain modernized to Go 1.26, frontend to Node 22 / npm 10 / Vite 6.

****v1.6.0****
- [x] New "Paste text" endpoint `POST /api/v1/files/paste` and matching panel modal: paste a body of text, pick a MIME (text/plain, json, sh, ps1, js, html, xml, python or custom), and the entry behaves like any other file (URL, password, expire, max-downloads, filters, kill switch, log, notify, QR, rotate).
- [x] `burn_after_read` policy on files: the first successful download deletes the record, blob, facade, password and per-file filters. Exposed in the Edit modal alongside the other delivery controls.

****v1.5.0****
- [x] Global kill switch in `DbConfig`: a pulsing red button in the topbar and a fixed banner halt every public download immediately, while the admin panel remains reachable.
- [x] Per-file "Mute notifications" toggle: the download is still logged but no outbound dispatch is sent.
- [x] Notification status filter (CSV in Settings) restricts which event statuses fire a notification, cutting scanner noise.
- [x] Auto-cleanup loop: hourly goroutine deletes files whose expiry passed more than N days ago and trims the download log to a configured cap. "Run now" button forces an immediate pass.
- [x] Per-rule hit counter on filters, shown as a yellow badge in the Filters modal.
- [x] Rule tester endpoint `POST /api/v1/filters/test` and matching UI: simulate a request against the filter chain without spinning up a real client.
- [x] Bulk actions `POST /api/v1/files/bulk` (enable / disable / pause / unpause / delete) on a multi-selection of files in a single round trip; floating action bar in the file view.
- [x] Re-upload action `POST /api/v1/files/{id}/replace`: swap the blob behind an existing file while preserving URL, password, filters and policy; download counter is reset.
- [x] QR code modal per file: the HTTP link rendered as a scannable QR, generated client-side.

****v1.4.0****
- [x] New per-file and global target filter chain (IP literal, CIDR, country via GeoIP, User-Agent regex) with `allow` / `deny` / `facade` / `redirect` actions, evaluated before the password prompt so scanners never see credentials they cannot satisfy.
- [x] Master switch and default action in `DbConfig`. A shield button in the topbar opens the Filters modal; per-file rules live in the Edit modal.
- [x] Download log status carries the matching rule (e.g. `filter-deny:cidr:127.0.0.0/8`).

****v1.3.0****
- [x] Per-file delivery controls added to `DbFile`: `expire_at` (Unix timestamp, 0 = never), `max_downloads` (server-side enforced, 1 = one-time link) and `download_count`.
- [x] Optional per-file password stored separately as a bcrypt hash (so it never leaks into list responses) and enforced via HTTP Basic before the file is served.
- [x] Enforcement is in the serve path: expired → 404 + log `expired`; quota hit → 404 + log `exhausted`; bad password → 401 + log `bad-password`. On a successful download with quota = max, the file is auto-disabled.

****v1.2.0****
- [x] Every download event is now persisted as a `DbDownloadLog` (IP, User-Agent, Referer, status, timestamp) and exposed via `GET /api/v1/downloads` (with `DELETE` to clear).
- [x] Async notification dispatch for each download event: generic JSON webhook, Telegram `sendMessage` (Markdown-formatted) and Slack incoming-webhook. 5-second timeout; failures are logged and do not block the serve path.
- [x] Downloads modal in the panel shows the recent history with a colored badge per status; a Notifications section in Settings configures the sinks.

****v1.1.0****
- [x] Long-lived API tokens for scripted access (`DbApiToken`): create, list and revoke from a new Tokens modal. Authentication accepts `Authorization: Bearer …`, `X-Pwndrop-Token` or the existing session cookie, so the same token works for the panel and for curl / PowerShell.
- [x] Endpoint `/api/v1/tokens` (GET, POST, DELETE). Created tokens are returned in full once; the listing keeps the full value plus a `…last-6` hint for quick recognition.
- [x] Modal includes ready-to-use command snippets (bash / cmd / PowerShell with `curl.exe`) so a fresh token is one click away from being useful.

****v1.0.2****
- [x] Unauthorized connections not pointing to any hosted files, returning 404, are now automatically closed instead of being kept alive. This resolves the issue of pwndrop getting DDoSed quickly with bots hammering requests at it from various sources.
- [x] Anti-DDoS feature has been added, which temporarily blacklists every IP address of a client who made 10 consecutive requests returning 404. Blacklist period is currently 10 minutes.
- [x] Removed timeouts for uploading and downloading files fully. The previous 15 minutes timeout would have not helped with DDoS attacks anyway. 

****v1.0.1****
- [x] Increased the time limit for uploads and downloads from 15 seconds to 15 minutes. Should fix the issue of uploads/downloads being interrupted on slow connections, when handling big files.
