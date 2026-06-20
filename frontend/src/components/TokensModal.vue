<template>
  <Modal :model-value="modelValue" title="API Tokens" size="lg" ok-title="Done" @ok="close" @update:model-value="close">
    <p class="token-help">
      Use API tokens to upload and manage files from scripts without logging in.
    </p>

    <div class="form-group row create-row">
      <div class="col">
        <input type="text" class="form-control" placeholder="Token name (e.g. ci-laptop)" v-model="name" @keyup.enter="create()">
      </div>
      <div class="col-auto">
        <button class="btn btn-primary" type="button" :disabled="!name" @click="create()">
          <i class="fas fa-plus" style="margin-right: 5px"></i>Generate
        </button>
      </div>
    </div>

    <div v-if="newToken" class="new-token-alert">
      <i class="fas fa-exclamation-triangle" style="margin-right:6px"></i>
      Copy token <strong>{{ newTokenName }}</strong> now — it won't be shown again.
      <div class="input-group mt-1">
        <input type="text" class="form-control token-value" :value="newToken" readonly @focus="$event.target.select()">
        <div class="input-group-append">
          <button class="btn btn-success" type="button" @click="copy(newToken)" v-tooltip:left="'Copy token'">
            <i class="fas fa-copy"></i>
          </button>
        </div>
      </div>
    </div>

    <div class="cmd-section">
      <div class="cmd-section-header">
        <span>Ready-to-use commands</span>
        <span class="cmd-section-hint">paste or generate a token above</span>
      </div>
      <div class="cmd-token-row">
        <span class="cmd-label">Token</span>
        <div class="input-group">
          <input type="text" class="form-control cmd-input" v-model="cmdToken" placeholder="<paste token here>">
          <div class="input-group-append">
            <button class="btn btn-outline-secondary" type="button" @click="copy(cmdToken)" :disabled="!cmdToken" v-tooltip:left="'Copy token'">
              <i class="fas fa-copy"></i>
            </button>
          </div>
        </div>
      </div>
      <div class="cmd-row">
        <span class="cmd-label"><i class="fas fa-terminal"></i> bash/cmd</span>
        <div class="input-group">
          <input type="text" class="form-control cmd-input" :value="curlCmd" readonly @focus="$event.target.select()">
          <div class="input-group-append">
            <button class="btn btn-outline-secondary" type="button" @click="copy(curlCmd)" v-tooltip:left="'Copy curl command'">
              <i class="fas fa-copy"></i>
            </button>
          </div>
        </div>
      </div>
      <div class="cmd-row">
        <span class="cmd-label"><i class="fab fa-windows"></i> PowerShell</span>
        <div class="input-group">
          <input type="text" class="form-control cmd-input" :value="psCmd" readonly @focus="$event.target.select()">
          <div class="input-group-append">
            <button class="btn btn-outline-secondary" type="button" @click="copy(psCmd)" v-tooltip:left="'Copy PowerShell command'">
              <i class="fas fa-copy"></i>
            </button>
          </div>
        </div>
      </div>
    </div>

    <hr>

    <div v-if="tokens.length === 0" class="no-tokens">No API tokens yet.</div>
    <table v-else class="table token-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Token</th>
          <th>Last used</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="t in tokens" :key="t.id">
          <td class="clip">{{ t.name }}</td>
          <td><code>{{ t.token_hint }}</code></td>
          <td>{{ lastUsed(t.last_used) }}</td>
          <td class="text-right row-actions">
            <button class="btn btn-sm btn-primary btn-circle-sm" @click="useToken(t.token)" v-tooltip:left="'Use in commands above'">
              <i class="fas fa-terminal"></i>
            </button>
            <button class="btn btn-sm btn-secondary btn-circle-sm" @click="copy(t.token)" v-tooltip:left="'Copy token'">
              <i class="fas fa-copy"></i>
            </button>
            <button class="btn btn-sm btn-danger btn-circle-sm" @click="revoke(t.id)" v-tooltip:left="'Revoke this token'">
              <i class="fas fa-times"></i>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </Modal>
</template>

<script>
import api from '../api'
import Modal from './Modal.vue'

export default {
  name: 'TokensModal',
  components: { Modal },
  props: {
    modelValue: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      tokens: [],
      name: '',
      newToken: '',
      newTokenName: '',
      cmdToken: '',
    }
  },
  computed: {
    baseUrl() {
      return window.location.origin
    },
    curlCmd() {
      const tok = this.cmdToken || '<TOKEN>'
      return `curl -H "Authorization: Bearer ${tok}" -F "file=@./yourfile" ${this.baseUrl}/api/v1/files`
    },
    psCmd() {
      const tok = this.cmdToken || '<TOKEN>'
      return `curl.exe -H "Authorization: Bearer ${tok}" -F "file=@.\\yourfile" ${this.baseUrl}/api/v1/files`
    },
  },
  watch: {
    modelValue(open) {
      if (open) {
        this.newToken = ''
        this.newTokenName = ''
        this.cmdToken = ''
        this.name = ''
        this.refresh()
      }
    },
  },
  methods: {
    close() {
      this.$emit('update:modelValue', false)
    },
    refresh() {
      api
        .get('/tokens')
        .then((response) => {
          this.tokens = response.data.data.tokens || []
        })
        .catch((error) => console.log(error))
    },
    create() {
      if (!this.name) {
        return
      }
      api
        .post('/tokens', { name: this.name }, { headers: { 'content-type': 'application/json' } })
        .then((response) => {
          this.newToken = response.data.data.token
          this.newTokenName = response.data.data.name
          this.cmdToken = response.data.data.token
          this.name = ''
          this.refresh()
        })
        .catch((error) => console.log(error))
    },
    revoke(id) {
      api
        .delete('/tokens/' + id)
        .then(() => this.refresh())
        .catch((error) => console.log(error))
    },
    useToken(token) {
      this.cmdToken = token
    },
    copy(text) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).catch((e) => console.log(e))
      }
    },
    lastUsed(ts) {
      if (!ts) {
        return 'never'
      }
      return new Date(ts * 1000).toLocaleString()
    },
  },
}
</script>

<style scoped>
.token-help {
  font-size: 13px;
  color: var(--pwn-slite);
  margin-bottom: 10px;
}
.create-row {
  margin-bottom: 10px;
}
.new-token-alert {
  background: rgba(40, 167, 69, 0.15);
  border: 1px solid rgba(40, 167, 69, 0.4);
  border-radius: 4px;
  padding: 8px 12px;
  font-size: 13px;
  color: #8bc34a;
  margin-bottom: 12px;
}
.token-value {
  font-family: monospace;
  font-size: 12px;
  background: var(--pwn-black2);
  color: #ddd;
  border-color: var(--pwn-black-hr);
}
.cmd-section {
  background: var(--pwn-black2);
  border: 1px solid var(--pwn-black-hr);
  border-radius: 4px;
  padding: 10px 12px;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.cmd-section-header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 13px;
  color: #ddd;
}
.cmd-section-hint {
  font-size: 11px;
  color: var(--pwn-slite);
}
.cmd-token-row,
.cmd-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.cmd-label {
  font-size: 11px;
  color: var(--pwn-slite);
  white-space: nowrap;
  width: 82px;
  flex-shrink: 0;
  text-align: right;
}
.cmd-input {
  font-size: 11px;
  font-family: monospace;
  background: var(--pwn-black);
  color: var(--pwn-sslite);
  border-color: var(--pwn-black-hr);
}
.cmd-input:focus {
  background: var(--pwn-black);
  color: var(--pwn-sslite);
}
.no-tokens {
  color: var(--pwn-slite);
  text-align: center;
  padding: 10px;
}
.token-table {
  color: #ddd;
}
.token-table th {
  border-top: none;
  color: var(--pwn-slite);
  font-weight: normal;
}
.token-table td,
.token-table th {
  border-color: var(--pwn-black-hr);
  vertical-align: middle;
}
.token-table code {
  color: var(--pwn-sslite);
}
.row-actions {
  white-space: nowrap;
}
.row-actions .btn {
  margin-left: 4px;
}
</style>
