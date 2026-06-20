<template>
  <Modal :model-value="modelValue" title="Target filters" size="lg" ok-title="Done" @ok="close" @update:model-value="close">
    <p class="fm-help">
      Rules are evaluated in priority order (low&nbsp;→&nbsp;high). First match decides the action.
      <strong>Per-file rules</strong> (set in the file's Edit modal) win over these global rules.
    </p>

    <div class="form-group row align-items-center">
      <label class="col-sm-4 col-form-label">Master switch:</label>
      <div class="col-sm-8 d-flex align-items-center">
        <input type="checkbox" v-model="enabled" @change="saveConfig()">
        <span class="fm-side">{{ enabled ? 'filters are evaluated' : 'all downloads pass through' }}</span>
      </div>
    </div>
    <div class="form-group row align-items-center">
      <label class="col-sm-4 col-form-label">Default action (no match):</label>
      <div class="col-sm-8">
        <select class="form-control fm-sel" v-model="defaultAction" @change="saveConfig()">
          <option value="allow">allow — serve the payload</option>
          <option value="deny">deny — 404</option>
          <option value="facade">facade — serve the facade file (or 404)</option>
          <option value="redirect">redirect — to the configured redirect URL</option>
        </select>
      </div>
    </div>

    <hr>
    <h6 class="fm-title">Global rules</h6>
    <FilterTable :file-id="0" />

    <hr>
    <h6 class="fm-title">Rule tester</h6>
    <p class="fm-help">
      Simulate a download request and see which rule would fire. No actual traffic is generated.
    </p>
    <div class="fm-test-row">
      <input type="text" class="form-control fm-test-input" placeholder="remote IP (e.g. 1.2.3.4)" v-model="test.remoteIp">
      <input type="text" class="form-control fm-test-input" placeholder="User-Agent (optional)" v-model="test.userAgent">
      <input type="number" min="0" class="form-control fm-test-input fm-test-fid" placeholder="file_id (0 = global only)" v-model.number="test.fileId">
      <button class="btn btn-primary" :disabled="!test.remoteIp" @click="runTest()">
        <i class="fas fa-vial" style="margin-right:5px"></i>Test
      </button>
    </div>
    <div v-if="testResult" class="fm-test-result" :class="'fm-test-' + testResult.action">
      <strong>{{ testResult.summary }}</strong>
      <div class="fm-test-meta">
        action=<code>{{ testResult.action }}</code> · log_tag=<code>{{ testResult.log_tag }}</code>
      </div>
    </div>
  </Modal>
</template>

<script>
import api from '../api'
import Modal from './Modal.vue'
import FilterTable from './FilterTable.vue'

export default {
  name: 'FiltersModal',
  components: { Modal, FilterTable },
  props: {
    modelValue: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      enabled: false,
      defaultAction: 'allow',
      // Keep the rest of the config around so saveConfig() doesn't wipe other
      // fields when toggling the filter knobs.
      _config: {},
      test: { remoteIp: '', userAgent: '', fileId: 0 },
      testResult: null,
    }
  },
  watch: {
    modelValue(open) {
      if (open) this.refresh()
    },
  },
  methods: {
    close() {
      this.$emit('update:modelValue', false)
    },
    refresh() {
      api
        .get('/config')
        .then((r) => {
          this._config = r.data.data
          this.enabled = !!this._config.filters_enabled
          this.defaultAction = this._config.filters_default_action || 'allow'
        })
        .catch((e) => console.log(e))
    },
    runTest() {
      if (!this.test.remoteIp) return
      api
        .post(
          '/filters/test',
          {
            file_id: Number(this.test.fileId) || 0,
            remote_ip: this.test.remoteIp,
            user_agent: this.test.userAgent,
          },
          { headers: { 'content-type': 'application/json' } }
        )
        .then((r) => {
          this.testResult = r.data.data
        })
        .catch((e) => console.log(e))
    },
    saveConfig() {
      const merged = {
        ...this._config,
        filters_enabled: this.enabled,
        filters_default_action: this.defaultAction,
      }
      api
        .post('/config', merged, { headers: { 'content-type': 'application/json' } })
        .then(() => {
          this._config = merged
        })
        .catch((e) => console.log(e))
    },
  },
}
</script>

<style scoped>
.fm-help {
  font-size: 13px;
  color: var(--pwn-slite);
}
.fm-side {
  margin-left: 10px;
  color: var(--pwn-slite);
  font-size: 12px;
}
.fm-title {
  color: var(--pwn-slite);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 1px;
  margin: 0 0 10px 0;
}
select.form-control.fm-sel {
  background: var(--pwn-black);
  color: #fff;
  border: 1px solid var(--pwn-med);
  border-radius: 21px;
  padding: 6px 14px;
}
.fm-test-row {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 8px;
}
.fm-test-input { flex: 1; }
.fm-test-fid { flex: 0 0 90px !important; }
.fm-test-result {
  padding: 10px 12px;
  border-radius: 12px;
  font-size: 13px;
  color: #fff;
}
.fm-test-meta {
  font-size: 11px;
  color: var(--pwn-slite);
  margin-top: 4px;
}
.fm-test-result code { color: #ddd; }
.fm-test-allow { background: rgba(40,167,69,0.2); border: 1px solid rgba(40,167,69,0.4); }
.fm-test-deny { background: rgba(220,53,69,0.2); border: 1px solid rgba(220,53,69,0.4); }
.fm-test-facade { background: rgba(108,117,125,0.2); border: 1px solid rgba(108,117,125,0.4); }
.fm-test-redirect { background: rgba(255,193,7,0.2); border: 1px solid rgba(255,193,7,0.4); }
</style>
