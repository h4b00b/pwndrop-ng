<template>
  <Modal :model-value="modelValue" title="Download log" size="lg" ok-title="Done" @ok="close" @update:model-value="close">
    <div class="dl-header">
      <p class="dl-help">
        Each successful download (or redirect/facade hit) of a hosted file is logged with IP, User-Agent, and referer.
      </p>
      <div class="dl-actions">
        <button class="btn btn-sm btn-secondary" @click="refresh()" v-tooltip:bottom="'Reload'">
          <i class="fas fa-sync"></i>
        </button>
        <button class="btn btn-sm btn-danger" :disabled="downloads.length === 0" @click="clear()" v-tooltip:bottom="'Clear all log entries'">
          <i class="fas fa-trash"></i>
        </button>
      </div>
    </div>

    <div v-if="downloads.length === 0" class="dl-empty">No downloads recorded yet.</div>
    <div v-else class="dl-table-wrap">
      <table class="table dl-table">
        <thead>
          <tr>
            <th>When</th>
            <th>File</th>
            <th>Path</th>
            <th>IP</th>
            <th>User-Agent</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in downloads" :key="d.id">
            <td class="dl-ts">{{ fmtTime(d.timestamp) }}</td>
            <td class="dl-file">{{ d.file_name }}</td>
            <td class="dl-path"><code>{{ d.url_path }}</code></td>
            <td class="dl-ip"><code>{{ d.remote_ip }}</code></td>
            <td class="dl-ua" :title="d.user_agent">{{ d.user_agent }}</td>
            <td><span class="dl-badge" :class="'dl-badge-' + d.status">{{ d.status }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </Modal>
</template>

<script>
import api from '../api'
import Modal from './Modal.vue'

export default {
  name: 'DownloadsModal',
  components: { Modal },
  props: {
    modelValue: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      downloads: [],
    }
  },
  watch: {
    modelValue(open) {
      if (open) {
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
        .get('/downloads')
        .then((response) => {
          this.downloads = response.data.data.downloads || []
        })
        .catch((error) => console.log(error))
    },
    clear() {
      if (!confirm('Clear the entire download log?')) {
        return
      }
      api
        .delete('/downloads')
        .then(() => this.refresh())
        .catch((error) => console.log(error))
    },
    fmtTime(ts) {
      if (!ts) {
        return ''
      }
      return new Date(ts * 1000).toLocaleString()
    },
  },
}
</script>

<style scoped>
.dl-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
.dl-help {
  font-size: 13px;
  color: var(--pwn-slite);
  margin: 0;
}
.dl-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
.dl-empty {
  color: var(--pwn-slite);
  text-align: center;
  padding: 14px;
}
.dl-table-wrap {
  max-height: 60vh;
  overflow-y: auto;
}
.dl-table {
  color: #ddd;
  font-size: 12px;
  margin-bottom: 0;
}
.dl-table th {
  border-top: none;
  color: var(--pwn-slite);
  font-weight: normal;
  position: sticky;
  top: 0;
  background: var(--pwn-black);
  z-index: 1;
}
.dl-table td,
.dl-table th {
  border-color: var(--pwn-black-hr);
  vertical-align: middle;
  padding: 6px 8px;
}
.dl-table code {
  color: var(--pwn-sslite);
  font-size: 11px;
}
.dl-ts {
  white-space: nowrap;
  color: var(--pwn-slite);
}
.dl-file {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dl-path {
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dl-ip {
  white-space: nowrap;
}
.dl-ua {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--pwn-sslite);
}
.dl-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.dl-badge-ok {
  background: rgba(40, 167, 69, 0.2);
  color: #8bc34a;
}
.dl-badge-redirect {
  background: rgba(255, 193, 7, 0.2);
  color: #ffc107;
}
.dl-badge-paused-facade {
  background: rgba(108, 117, 125, 0.25);
  color: #adb5bd;
}
</style>
