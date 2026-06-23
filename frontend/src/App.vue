<template>
  <div id="app">
    <div v-if="!isDead && session.isLoggedIn" class="top-left-bar" :class="{ 'with-banner': banner.shown }">
      <button class="btn btn-primary btn-circle" style="margin-right: 8px" @click="showConfig()" v-tooltip:bottom="'Settings'">
        <i class="fas fa-cog"></i>
      </button>
      <button class="btn btn-primary btn-circle" style="margin-right: 8px" @click="tokensShow = true" v-tooltip:bottom="'API tokens'">
        <i class="fas fa-key"></i>
      </button>
      <button class="btn btn-primary btn-circle" style="margin-right: 8px" @click="downloadsShow = true" v-tooltip:bottom="'Download log'">
        <i class="fas fa-history"></i>
      </button>
      <button class="btn btn-primary btn-circle" style="margin-right: 8px" @click="filtersShow = true" v-tooltip:bottom="'Target filters'">
        <i class="fas fa-shield-alt"></i>
      </button>
      <button
        class="btn btn-circle kill-btn"
        :class="killOn ? 'kill-on' : 'btn-primary'"
        style="margin-right: 8px"
        @click="toggleKill()"
        v-tooltip:bottom="killOn ? 'KILL SWITCH ACTIVE — click to resume serving' : 'Kill switch — emergency stop'">
        <i class="fas fa-power-off"></i>
      </button>
    </div>
    <div v-if="!isDead" class="top-right-bar" :class="{ 'with-banner': banner.shown }">
      <button class="btn btn-primary btn-circle" @click="logout()">
        <i class="fas fa-sign-out-alt"></i>
      </button>
    </div>

    <div v-if="killOn" class="kill-banner">
      <i class="fas fa-exclamation-triangle"></i>
      KILL SWITCH ACTIVE — every public download returns 404
    </div>

    <div v-if="!isDead && session.isLoggedIn && update.available" class="update-banner">
      <i class="fas fa-cloud-download-alt"></i>
      New version <strong>{{ update.latest }}</strong> available
      (you have {{ update.current }}) ·
      <a :href="update.release_url" target="_blank" rel="noopener">release notes</a>
      <button class="btn btn-sm btn-primary update-btn" :disabled="update.applying" @click="applyUpdate()">
        <span v-if="!update.applying">Update now</span>
        <span v-else><i class="fas fa-spinner fa-spin"></i> Updating…</span>
      </button>
    </div>

    <div class="bg-title">
      <a href="#/"><img src="/img/pwndrop-title.png" alt="pwndrop NG" /></a>
    </div>
    <div class="bg-footer">
      made by <a href="https://twitter.com/mrgretzky" target="_blank">@mrgretzky</a> &middot;
      NG fork by <a href="https://github.com/h4b00b" target="_blank">@h4b00b</a>
    </div>
    <div class="bg-version">
      version {{ version }}
    </div>

    <Modal
      v-model="configShow"
      title="Settings"
      size="lg"
      ok-title="Save"
      :ok-disabled="!isConfigComplete"
      @ok="saveConfig()"
    >
      <form>
        <div class="form-group row">
          <label for="redirect-url" class="col-sm-3 col-form-label label-help">Redirect URL:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Visitors will be redirected to this URL if they provide a wrong download URL or are unauthorized to view the admin panel'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="redirect-url" spellcheck="false" v-model="config.redirect_url">
          </div>
        </div>
        <div class="form-group row">
          <label for="secret-path" class="col-sm-3 col-form-label label-help">Secret Path:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Visiting this path in a browser will authorize the visitor to view the admin panel (IMPORTANT! CHANGE FROM DEFAULT)'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="secret-path" spellcheck="false" v-model="config.secret_path">
          </div>
        </div>
        <div class="form-group row">
          <label for="cookie-name" class="col-sm-3 col-form-label label-help">Secret-Cookie Name:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Secret cookie name, which is used for authorizing the visitor to view the admin panel'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="cookie-name" spellcheck="false" v-model="config.cookie_name">
          </div>
        </div>
        <div class="form-group row">
          <label for="cookie-token" class="col-sm-3 col-form-label label-help">Secret-Cookie Value:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Secret cookie value, which is used for authorizing the visitor to view the admin panel'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="cookie-token" spellcheck="false" v-model="config.cookie_token">
          </div>
        </div>

        <hr>
        <h6 class="section-title">Download notifications</h6>

        <div class="form-group row">
          <label for="notify-enabled" class="col-sm-3 col-form-label label-help">Enabled:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Master switch — when off, downloads are still logged but no outbound notifications fire'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="notify-enabled" v-model="config.notify_enabled">
          </div>
        </div>
        <div class="form-group row">
          <label for="notify-webhook" class="col-sm-3 col-form-label label-help">Generic webhook:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'POSTed with the full event JSON: file_name, url_path, remote_ip, user_agent, referer, status, timestamp'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="notify-webhook" spellcheck="false" placeholder="https://example.com/hook" v-model="config.notify_webhook_url">
          </div>
        </div>
        <div class="form-group row">
          <label for="notify-tg-token" class="col-sm-3 col-form-label label-help">Telegram bot token:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'BotFather token. Leave empty to disable Telegram'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="notify-tg-token" spellcheck="false" placeholder="123456:ABC-DEF..." v-model="config.notify_telegram_token">
          </div>
        </div>
        <div class="form-group row">
          <label for="notify-tg-chat" class="col-sm-3 col-form-label label-help">Telegram chat ID:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Numeric chat ID or @channelname'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="notify-tg-chat" spellcheck="false" v-model="config.notify_telegram_chat_id">
          </div>
        </div>
        <div class="form-group row">
          <label for="notify-slack" class="col-sm-3 col-form-label label-help">Slack webhook:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Slack incoming-webhook URL. Leave empty to disable Slack'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="notify-slack" spellcheck="false" placeholder="https://hooks.slack.com/services/..." v-model="config.notify_slack_webhook">
          </div>
        </div>
        <div class="form-group row">
          <label for="notify-status" class="col-sm-3 col-form-label label-help">Only notify on status:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Comma-separated list of statuses to notify on (e.g. ok,paused-facade). Empty = notify on every download event including scanner blocks.'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="notify-status" spellcheck="false" placeholder="ok,paused-facade  (empty = all)" v-model="notifyStatusCsv">
          </div>
        </div>

        <hr>
        <h6 class="section-title">Network</h6>

        <div class="form-group row">
          <label for="trust-proxy" class="col-sm-3 col-form-label label-help">Trust X-Forwarded-For:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Use the first X-Forwarded-For (or X-Real-IP) entry as the client IP for filters, blacklist, login throttle, and logs. Enable ONLY when pwndrop is behind a trusted reverse proxy — otherwise any client can spoof its IP via a header.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="trust-proxy" v-model="config.trust_proxy">
          </div>
        </div>

        <hr>
        <h6 class="section-title">Hardening</h6>

        <div class="form-group row">
          <label for="rl-enabled" class="col-sm-3 col-form-label label-help">Rate limit per IP:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'In-process token bucket. When a single client IP exceeds the budget, the request is 404d and logged as &quot;rate-limit&quot; — keeps a scanner storm out of the DB.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="rl-enabled" v-model="config.rate_limit_enabled">
          </div>
        </div>
        <div class="form-group row">
          <label for="rl-per-min" class="col-sm-3 col-form-label label-help">Requests / minute:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Per-IP cap. 0 (or empty) = use default (60).'"></i>
          </label>
          <div class="col-sm-9">
            <input type="number" min="0" class="form-control" id="rl-per-min" v-model.number="config.rate_limit_per_minute">
          </div>
        </div>
        <div class="form-group row">
          <label for="range-enabled" class="col-sm-3 col-form-label label-help">Range / resume:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Honor HTTP Range requests (RFC 7233). Auto-disabled per file when MaxDownloads or burn-after-read are set — those need single-shot delivery semantics.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="range-enabled" v-model="config.range_enabled">
          </div>
        </div>

        <hr>
        <h6 class="section-title">Auto-cleanup</h6>

        <div class="form-group row">
          <label for="cleanup-enabled" class="col-sm-3 col-form-label label-help">Enabled:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Runs hourly. Deletes files whose expiry was hit more than N days ago, and trims the download log to the configured cap.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="cleanup-enabled" v-model="config.cleanup_enabled">
            <button type="button" class="btn btn-sm btn-secondary" style="margin-left:12px" @click="runCleanupNow()" v-tooltip:right="'Force a cleanup pass now'">
              <i class="fas fa-broom" style="margin-right:5px"></i>Run now
            </button>
          </div>
        </div>
        <div class="form-group row">
          <label for="cleanup-days" class="col-sm-3 col-form-label label-help">Delete expired files older than (days):
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'0 = never auto-delete files'"></i>
          </label>
          <div class="col-sm-9">
            <input type="number" min="0" class="form-control" id="cleanup-days" v-model.number="config.cleanup_expired_after_days">
          </div>
        </div>
        <div class="form-group row">
          <label for="cleanup-log" class="col-sm-3 col-form-label label-help">Cap download log to (entries):
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'0 = no cap'"></i>
          </label>
          <div class="col-sm-9">
            <input type="number" min="0" class="form-control" id="cleanup-log" v-model.number="config.cleanup_log_max_entries">
          </div>
        </div>
      </form>
    </Modal>

    <TokensModal v-model="tokensShow" />
    <DownloadsModal v-model="downloadsShow" />
    <FiltersModal v-model="filtersShow" />

    <div class="bg-logo"></div>
    <div class="container">
      <div v-if="isDead" class="text-center">
        <span class="big-icon">
          <i class="fas fa-dizzy"></i>
        </span>
      </div>
      <div v-else-if="!isLoaded"></div>
      <div v-else>
        <router-view></router-view>
      </div>
    </div>
  </div>
</template>

<script>
import Modal from './components/Modal.vue'
import TokensModal from './components/TokensModal.vue'
import DownloadsModal from './components/DownloadsModal.vue'
import FiltersModal from './components/FiltersModal.vue'
import api from './api'
import { session } from './state'

export default {
  name: 'App',
  components: { Modal, TokensModal, DownloadsModal, FiltersModal },
  setup() {
    return { session }
  },
  data() {
    return {
      isLoaded: false,
      isDead: false,
      config: {},
      notifyStatusCsv: '',
      configShow: false,
      tokensShow: false,
      downloadsShow: false,
      filtersShow: false,
      killOn: false,
      version: '-',
      update: {
        available: false,
        applying: false,
        current: '',
        latest: '',
        release_url: '',
      },
    }
  },
  computed: {
    isConfigComplete() {
      return !!(this.config.secret_path && this.config.cookie_name && this.config.cookie_token)
    },
    banner() {
      // Whether any top-of-page banner is visible — top bars need to shift
      // down to clear it.
      return { shown: this.killOn || (session.isLoggedIn && this.update.available) }
    },
  },
  methods: {
    authCheck() {
      api
        .get('/auth')
        .then((response) => {
          if (response.data.data.status === 0) {
            this.$router.push('/create_account').catch(() => {})
          } else if (response.data.data.status === 1) {
            session.isLoggedIn = true
            this.refreshKillSwitch()
          }
        })
        .catch(() => {
          this.$router.push('/login').catch(() => {})
        })
        .then(() => {
          this.isLoaded = true
        })
    },
    logout() {
      if (session.isLoggedIn) {
        api
          .get('/logout')
          .then(() => {
            session.isLoggedIn = false
            this.$router.push('/login').catch(() => {})
          })
          .catch((error) => console.log(error))
      } else {
        api
          .get('/clear_secret')
          .then(() => {
            this.isDead = true
          })
          .catch((error) => console.log(error))
      }
    },
    showConfig() {
      api
        .get('/config')
        .then((response) => {
          this.config = { ...response.data.data }
          this.notifyStatusCsv = (this.config.notify_status_filter || []).join(',')
          this.configShow = true
        })
        .catch((error) => console.log(error))
    },
    saveConfig() {
      if (!this.isConfigComplete) {
        return
      }
      const payload = { ...this.config }
      payload.notify_status_filter = this.notifyStatusCsv
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      api
        .post('/config', payload, { headers: { 'content-type': 'application/json' } })
        .then(() => {
          this.configShow = false
          this.refreshKillSwitch()
        })
        .catch((error) => console.log(error))
    },
    refreshKillSwitch() {
      api
        .get('/config')
        .then((r) => {
          this.killOn = !!r.data.data.kill_switch
        })
        .catch(() => {})
    },
    toggleKill() {
      const verb = this.killOn ? 'resume serving' : 'activate the kill switch'
      if (!confirm('Are you sure you want to ' + verb + '?')) return
      api
        .get('/config')
        .then((r) => {
          const merged = { ...r.data.data, kill_switch: !this.killOn }
          return api.post('/config', merged, { headers: { 'content-type': 'application/json' } })
        })
        .then(() => this.refreshKillSwitch())
        .catch((e) => console.log(e))
    },
    runCleanupNow() {
      api
        .post('/cleanup/run')
        .then(() => {
          // intentional no-op: the server logs the result
        })
        .catch((e) => console.log(e))
    },
    syncVersion() {
      api
        .get('/version')
        .then((response) => {
          this.version = response.data.data.version
        })
        .catch((error) => console.log(error))
    },
    checkUpdate() {
      api
        .get('/update/check')
        .then((response) => {
          const d = response.data.data || {}
          this.update.available = !!d.available
          this.update.current = d.current || ''
          this.update.latest = d.latest || ''
          this.update.release_url = d.release_url || ''
        })
        .catch(() => {
          // silent — no network / no release shouldn't pester the admin
        })
    },
    applyUpdate() {
      if (!confirm(`Update to version ${this.update.latest}? The service will restart.`)) return
      this.update.applying = true
      api
        .post('/update/apply')
        .then(() => {
          // Server re-execs ~250ms after the response. Poll /version until the
          // reported version changes, then reload.
          this.pollForRestart(this.update.current)
        })
        .catch((e) => {
          this.update.applying = false
          alert('Update failed: ' + (e?.response?.data?.message || e.message))
        })
    },
    pollForRestart(oldVersion, attempt = 0) {
      if (attempt > 60) {
        this.update.applying = false
        alert('Update started but the service did not come back within 2 minutes. Check server logs.')
        return
      }
      setTimeout(() => {
        api
          .get('/version')
          .then((r) => {
            const v = r?.data?.data?.version
            if (v && v !== oldVersion) {
              window.location.reload()
            } else {
              this.pollForRestart(oldVersion, attempt + 1)
            }
          })
          .catch(() => this.pollForRestart(oldVersion, attempt + 1))
      }, 2000)
    },
  },
  watch: {
    'session.isLoggedIn'(v) {
      if (v) this.checkUpdate()
    },
  },
  created() {
    this.syncVersion()
    this.authCheck()
  },
}
</script>

<style scoped>
.section-title {
  color: var(--pwn-slite);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 1px;
  margin: 0 0 10px 0;
}
.kill-btn {
  border: none;
  color: #fff;
}
.kill-on {
  background: #c62828;
  color: #fff;
  animation: kill-pulse 1.2s ease-in-out infinite;
}
.kill-banner {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  background: #c62828;
  color: #fff;
  text-align: center;
  padding: 6px 0;
  font-weight: bold;
  z-index: 1100;
  font-size: 13px;
}
.kill-banner .fas {
  margin-right: 6px;
}
.update-banner {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  background: #1565c0;
  color: #fff;
  text-align: center;
  padding: 6px 12px;
  font-size: 13px;
  z-index: 1099;
}
.update-banner .fas {
  margin-right: 6px;
}
.update-banner a {
  color: #fff;
  text-decoration: underline;
}
.update-btn {
  margin-left: 12px;
}
@keyframes kill-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(198,40,40,0.7); }
  50% { box-shadow: 0 0 0 10px rgba(198,40,40,0); }
}
</style>
