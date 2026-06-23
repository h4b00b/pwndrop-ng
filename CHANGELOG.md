****v2.0.8****
- [x] Fix: the v2.0.7 update banner (`position: fixed`, full-width, top of viewport) sat on top of the `.top-left-bar` and `.top-right-bar` circle buttons (settings / tokens / downloads / filters / kill / logout), making them un-clickable while the banner was up. The two top bars now get a `.with-banner` modifier (driven by a `banner` computed that's true when either the kill banner or the update banner is visible) which shifts their `top` from `15px` to `56px`, clearing the 36 px banner strip + a bit of breathing room. Same fix incidentally cleans up the long-standing overlap with the existing kill-switch banner.
- [x] Updater archive extraction now falls back to "the only regular file in the tarball" when no entry's basename matches the running binary. Covers the local-build case where the binary is named `pwndrop-ng-linux-amd64` but the release tar carries `pwndrop-ng/pwndrop-ng`, and any future repackaging that keeps one binary per tarball. Strict basename match still wins when present so a multi-file archive can't accidentally pick the wrong target.
- [x] Version constant bumped to `2.0.8`; frontend `package.json` synced.

****v2.0.7****
- [x] In-place self-update from the admin UI. New `core/updater` package hits `https://api.github.com/repos/h4b00b/pwndrop-ng/releases/latest`, picks the `pwndrop-ng-linux-<arch>.tar.gz` asset matching `runtime.GOOS+"-"+runtime.GOARCH`, streams the HTTP body straight through `gzip.NewReader` → `tar.NewReader` and extracts the embedded binary (matched by basename — tolerates both root-level and `pwndrop-ng/pwndrop-ng` archive layouts) directly into a sibling temp file in the binary's directory. The `.tar.gz` never lands on disk, so there is nothing to garbage-collect: on success the temp gets `rename(2)`'d over the running ELF (the kernel keeps the old inode mapped for the live process) and `syscall.Exec` swaps the process image in place — PID is preserved so systemd doesn't notice. On any failure the temp file is removed. Windows fallback spawns a detached child and exits.
- [x] Two new authenticated endpoints: `GET /api/v1/update/check` returns `{current, latest, available, notes, release_url, asset_name, asset_url, published_at}`, `POST /api/v1/update/apply` acknowledges with HTTP 200 then performs the swap+re-exec in a goroutine 250 ms later so the client gets a clean response before the process vanishes. Concurrency-guarded by a package-level mutex (one update at a time). Hard download cap of 200 MiB and 10-minute HTTP timeout. Trust model is "HTTPS + GitHub tag" — no checksum file, no signature; the repo (`h4b00b/pwndrop-ng`) is hard-coded so a compromised admin token can't redirect the update source.
- [x] Admin UI banner (blue, fixed top, sibling of the red kill-switch banner) appears on login when `available=true`, shows current → latest with a link to the release notes and an "Update now" button. After confirmation the frontend POSTs `/update/apply`, then polls `/version` every 2 s for up to 2 min; on first response with a different version it `window.location.reload()`s. Check fires once per session on the `session.isLoggedIn` watcher — silent on network failure so an offline server doesn't pester the admin.
- [x] Version constant bumped to `2.0.7`; frontend `package.json` synced.

****v2.0.6****
- [x] SHA256 of every stored blob, computed on the upload / paste / replace / chunked-complete paths and surfaced both to the operator (chip in the Edit modal, `sha256` field in `DbFile`) and to the downloader via `X-Content-SHA256` (hex) plus the RFC-3230 `Digest: sha-256=…` (base64) headers on both the HTTP and WebDAV serve paths. Headers are suppressed when the served bytes differ from the stored blob (watermark or auto-wrap on) and a `X-Content-Watermarked: true` flag is sent instead.
- [x] New `asn` filter match-type alongside `ip` / `cidr` / `country` / `ua_regex`. Pattern accepts `14618`, `AS14618` or `as14618`. The GeoIP cache entry now carries both `country` and `asn`, so one provider round-trip serves both rule types; decoder handles ipwho.is (`connection.asn`), ipapi.co (`asn`) and legacy ip-api.com (`"as":"AS14618 Amazon…"`). UI dropdown in `FilterTable.vue` shows AS14618/AS8075/AS396982 as placeholder examples (AWS / Azure / GCP).
- [x] Per-download watermarking (`DbFile.Watermark`): every served body gets a unique `\x00PWN:<32hex>\n` suffix appended, the tag is stored in `DbDownloadLog.Watermark` against IP/UA/timestamp so a leaked sample (grep `PWN:`) maps back to a single recipient. Tolerated by PE/ELF/most ZIP-based formats; documented as unsafe for strict-parser containers (PDF, ISO). New "Watermark" column in the Downloads modal.
- [x] Auto-wrap container at serve time (`DbFile.WrapAs`): `zip` repackages the blob into a single-entry STORE-method zip streamed via `archive/zip`, `Content-Disposition` forces the `.zip` extension and `X-Content-Wrapped: zip` is emitted. When watermark is also on, the suffix goes inside the wrapped file so the operator can still grep the extracted payload. ISO9660 is reserved for a follow-on iteration — the `wrapKinds` registry in `core/wrap.go` is open for one more case.
- [x] Per-IP rate limit (`core/ratelimit.go`): token bucket sitting at the very front of `runGate`, before kill switch and DB lookup, so a scanner storm from a single address is soaked up without touching storage. Configurable via `rate_limit_enabled` / `rate_limit_per_minute` in Settings → Hardening (default off, default budget 60 req/min); idle buckets reaped by the regular cleanup tick. Log status `rate-limit`.
- [x] RFC-7233 Range / resume support (`range_enabled` in `DbConfig`) via `http.ServeContent`. Auto-disabled per file when `MaxDownloads > 0`, `BurnAfterRead`, watermark or wrap are on — those need single-shot delivery semantics that partials would break. Log status `ok-range` when the request carries a `Range:` header.
- [x] Operator note field on `DbFile` (`note`, free-text). Surfaced as a textarea in the Edit modal; never serialised on the download path or emitted in any notification sink.
- [x] Version constant bumped to `2.0.6`; frontend `package.json` synced.

****v2.0.5****
- [x] Chunked upload protocol (`/api/v1/files/chunked/{init,append,complete,replace,abort}`) so files >100 MB can be uploaded through a Cloudflare free-plan proxied deployment without disabling the proxy. Frontend in `FileView.vue` switches to the chunked path for files over 80 MiB; smaller uploads keep the existing multipart flow. Strict append-only (`X-Chunk-Offset` must equal server-tracked `Received`), 95 MiB hard cap per chunk, 50 GiB total cap, in-memory upload registry with per-id mutex, stale uploads swept after 24h of inactivity by the cleanup loop. Replace flow `/files/chunked/{id}/replace/{fid}` mirrors `FileReplaceHandler`: swaps blob, resets `DownloadCount`, keeps URL/policy/password/filters.
- [x] Version constant bumped to `2.0.5`; frontend `package.json` synced.

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
