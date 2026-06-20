<p align="center">
  <img alt="pwndrop logo" src="media/pwndrop-logo-512.png" height="120" />
  <p align="center">
    <img alt="pwndrop title" src="media/pwndrop-title-black-512.png" height="40" />
  </p>
</p>

> **pwndrop NG** is a fork of the original **pwndrop** by Kuba Gretzky ([@mrgretzky](https://twitter.com/mrgretzky)), maintained by **Hab00b**. It keeps the upstream feature set and license (GPL3) and layers on top API tokens, download notifications & log, per-file delivery controls (expire / quota / password), a target-filter chain for anti-sandbox use, paste + burn-after-read, a kill switch, a modernized Vue 3 / Vite frontend, and a full security-hardening pass (see [CHANGELOG.md](CHANGELOG.md)).

**pwndrop** is a self-deployable file hosting service for sending out red teaming payloads or securely sharing your private files over HTTP and WebDAV.

If you've ever needed to quickly set up an nginx/apache web server to host your files and you were never happy with the limitations of `python -m http.server`, **pwndrop** is definitely for you!

<p align="center">
  <img alt="demo" src="media/demo1.gif" height="500" />
</p>

With **pwndrop** you can:
- [x] Upload and immediately share multiple files using your own private VPS, using drag & drop.
- [x] Decide to make files available or unavailable for download with a single click.
- [x] Set up custom download URLs, for shared files, without playing with directory structure.
- [x] Set up facade files, which will be served instead of the original file whenever you feel like it.
- [x] Set up automatic redirects to spoof the file's extension in a shared link.
- [x] Change MIME type of the served file to change browser's behavior when a download link is clicked.
- [x] Serve files over HTTP, HTTPS and WebDAV.
- [x] Install and setup everything using a bash oneliner.
- [x] Set up **pwndrop** to work as a nameserver and respond with a valid DNS A record to any sub-domain you choose.
- [x] Protect your admin panel behind a custom secret URL path and log in securely with your own username and password.
- [x] Never worry about setting up HTTPS certificates as **pwndrop** does everything for you in the background (including auto-renewals).

With **pwndrop NG** you also can:
- [x] Issue long-lived **API tokens** and drive uploads / management from `curl`, PowerShell or any script (Bearer header or `X-Pwndrop-Token`).
- [x] Get **download notifications** in real time via generic JSON webhook, Telegram or Slack, with a configurable status allowlist to cut scanner noise.
- [x] Inspect a persistent **download log** (IP, User-Agent, Referer, status, timestamp) directly from the panel.
- [x] Configure **per-file delivery controls**: expiry timestamp, max-downloads quota (1 = one-time link), and bcrypt-hashed access password served as HTTP Basic.
- [x] Apply a **target-filter chain** (IP literal, CIDR, country via GeoIP, User-Agent regex) with `allow` / `deny` / `facade` / `redirect` actions — perfect for anti-sandbox and operator-scoped delivery — per-file or global, evaluated before the password prompt.
- [x] **Paste text** straight into a hosted entry (bash, PowerShell, JSON, HTML, …) and ship it through the same delivery pipeline.
- [x] Enable **burn-after-read** so the first successful download wipes the record, the blob and the per-file rules.
- [x] Hit a global **kill switch** to 404-out every public download instantly while keeping the admin panel reachable.
- [x] **Bulk-act** on multiple files (enable / disable / pause / unpause / delete), **re-upload** behind an existing URL, **rotate** the random folder of a burned link, and grab a **QR code** for any payload URL.
- [x] Get a **rule tester** and **per-rule hit counter** so you can verify filter behavior without spinning up a real client.
- [x] Deploy as a **single static binary** for `linux/amd64` or `linux/arm64` — frontend is built with Vite and embedded into the binary via `go:embed`, no `admin/` directory to ship alongside.

Its main goal is to make file sharing as easy and intuitive as possible, while implementing extra features to aid in red team assessments.

Frontend of **pwndrop NG** is a Vue 3 single-page application built with Vite and embedded into the Go binary at compile time (`go:embed`). The backend serves a REST API and manages a local database, powered by Go.

## Upstream resources

These cover the original **pwndrop** workflow and still apply to the NG fork for everything outside the NG-specific features:

- Blog write-up by Kuba Gretzky: <https://breakdev.org/pwndrop>
- Video walk-through by Luke Turvey ([@TurvSec](https://twitter.com/TurvSec)):

  [![File and Phishing Payload Hosting using PwnDrop (Red Team) - Luke Turvey](https://img.youtube.com/vi/e3veSyIFvOE/0.jpg)](https://www.youtube.com/watch?v=e3veSyIFvOE)

NG-specific changes (API tokens, notifications, filters, paste, burn, kill switch, …) are documented in [CHANGELOG.md](CHANGELOG.md).

## Prerequisites

Register a domain and point its DNS A records to your VPS IP. You can also point the domain's `ns1` and `ns2` nameservers at the **pwndrop** instance IP — it will respond with valid DNS A replies on its own.

1. Registered domain name pointing to the **pwndrop** instance IP as a DNS A record or as a nameserver.
2. Server with at least 512 MB RAM.

If you want to set up **pwndrop** without a domain, see below for a local instance — it won't auto-generate HTTPS certificates.

## Installation

Make sure there aren't any DNS or HTTP(S) servers running before you attempt to install **pwndrop**.

#### Oneliner

Don't run oneliners without reading them first, but if you're in a hurry:
```
curl https://raw.githubusercontent.com/h4b00b/pwndrop-ng/main/install_linux.sh | sudo bash
```

Auto-detects `amd64` / `arm64`, pulls the matching release binary, installs and starts the daemon.

#### From binary

Grab the release package for your architecture from: https://github.com/h4b00b/pwndrop-ng/releases

Then:

```
tar zxvf pwndrop-ng-linux-amd64.tar.gz   # or -arm64
cd pwndrop-ng
./pwndrop-ng stop
./pwndrop-ng install
./pwndrop-ng start
./pwndrop-ng status
```

#### From source code

You need Go **1.25+** (see [go.mod](go.mod)) and Node **22+** for the frontend build.

```
sudo apt-get -y install git make
git clone https://github.com/h4b00b/pwndrop-ng
cd pwndrop-ng
make            # builds the Vue frontend then embeds it via go:embed into the Go binary
sudo make install
```

`make` builds for the host architecture; use `make build-amd64`, `make build-arm64`, or `make build-all` to cross-compile.

## Quickstart

Make sure the **pwndrop** is running.

1. Open the secret URL to authorize your browser: `https://yourdomain.com/pwndrop` (this is a default value; make sure to use the secret path, you've pre-configured)
2. Open the admin panel URL in your browser: `https://yourdomain.com/` (since you've authorized your browser, you will now see an admin panel login page)
3. Create your admin account or login.
4. Click the configuration cog in top-left corner and make sure you change the secret path to something other than `/pwndrop`.

You're good to go!

## Running from CLI

You don't have to install **pwndrop** as a daemon and you can run it straight from the console.

```
usage: pwndrop-ng [start|stop|install|remove|status] [-config <config_path>] [-debug] [-no-autocert] [-no-dns] [-h]

daemon management:
    start           : start the daemon
    stop            : stop the daemon
    install         : install the daemon using the available system manager (systemd, systemv and upstart supported)
    remove          : uninstall the daemon
    status          : check status of the installed daemon

parameters:
    -config         : specify a custom path to a config file (def. 'pwndrop.ini' in same directory as the executable)
    -debug          : enable debug output 
    -no-autocert    : disable automatic TLS certificate retrieval from LetsEncrypt; useful when you want to connect over IP or/and in a local network
    -no-dns         : do not run a DNS server on port 53 UDP; use this if you don't want to use pwndrop as a nameserver
    -h              : usage help
```

## Configuration

On first launch, **pwndrop**, by default, will create a new configuration file `pwndrop.ini` in the same directory as an executable. You can later modify it or supply your own, for example to pre-configure **pwndrop** before the installation to automate the deployment of a tool even better.

Here is an example config file with all available config variables with commentary:
```
[pwndrop]
listen_ip = "190.33.86.22"                  # the external IP of your pwndrop instance (must be set if you want to use the nameserver feature)
http_port = 80                              # listening port for HTTP and WebDAV
https_port = 443                            # listening port for HTTPS
data_dir = "./data"                         # directory path where data storage will reside (relative paths are from executable directory path)
admin_dir = "./admin"                       # OPTIONAL fallback: NG embeds the admin panel into the binary via go:embed; this is only used if the embedded FS is unavailable

[setup]                                     # optional: put in if you want to pre-configure pwndrop (section will be deleted from the config file on first run)
username = "admin"                          # username of the admin account
password = "secretpassword"                 # password of the admin account
redirect_url = "https://www.somedomain.com" # URL to which visitors will be redirected to if they supply a path, which doesn't point to any shared file (put blank if you want to return 404)
secret_path = "/pwndrop"                    # secret URL path, which upon visiting will allow your browser to access the login page of the admin panel (make sure to change the default value)
```

If you want to pre-configure your **pwndrop NG** instance before deployment using any of the installation scripts, put your configuration file at `/usr/local/pwndrop-ng/pwndrop.ini` and it will be parsed the moment the daemon is first executed.

## Credits

- Original **pwndrop** by Kuba Gretzky ([@mrgretzky](https://twitter.com/mrgretzky)) — upstream design, original Vue panel, daemon harness, WebDAV/autocert plumbing.
- **pwndrop NG** maintenance, hardening pass, and the NG-only features (API tokens, notifications, filters, paste, burn, kill switch, Vite/Vue 3 rebuild, embedded frontend) by **Hab00b**.

## License

**pwndrop** is made by Kuba Gretzky ([@mrgretzky](https://twitter.com/mrgretzky)) and released under GPL3.

**pwndrop NG** is a fork maintained by **Hab00b**, released under the same GPL3 license. All modifications since upstream `v1.0.2` are documented in [CHANGELOG.md](CHANGELOG.md), as required by GPL3 §5(a). The original copyright notice is preserved unchanged.
