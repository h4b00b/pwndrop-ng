<template>
  <div class="ft">
    <div v-if="filters.length === 0" class="ft-empty">No rules yet.</div>
    <table v-else class="table ft-table">
      <thead>
        <tr>
          <th class="ft-th-on">On</th>
          <th>Match</th>
          <th>Pattern</th>
          <th>Action</th>
          <th>Prio</th>
          <th v-tooltip:bottom="'Number of times this rule has matched a download request'">Hits</th>
          <th>Note</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="f in filters" :key="f.id" :class="{ 'ft-row-off': !f.enabled }">
          <td><input type="checkbox" :checked="f.enabled" @change="toggle(f)"></td>
          <td><span class="ft-badge">{{ f.match_type }}</span></td>
          <td><code>{{ f.pattern }}</code></td>
          <td><span class="ft-badge" :class="'ft-act-' + f.action">{{ f.action }}</span></td>
          <td>{{ f.priority }}</td>
          <td>
            <span class="ft-hits" :class="{ 'ft-hits-zero': !f.hit_count }">{{ f.hit_count || 0 }}</span>
          </td>
          <td class="ft-note">{{ f.note }}</td>
          <td class="text-right">
            <button class="btn btn-sm btn-danger btn-circle-sm" @click="remove(f)" v-tooltip:left="'Delete rule'">
              <i class="fas fa-times"></i>
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="ft-add">
      <div class="ft-add-row">
        <select class="form-control ft-sel" v-model="draft.match_type">
          <option value="ip">ip</option>
          <option value="cidr">cidr</option>
          <option value="country">country (ISO2)</option>
          <option value="ua_regex">ua_regex</option>
        </select>
        <input type="text" class="form-control" :placeholder="patternPlaceholder" v-model="draft.pattern">
        <select class="form-control ft-sel" v-model="draft.action">
          <option value="allow">allow</option>
          <option value="deny">deny</option>
          <option value="facade">facade</option>
          <option value="redirect">redirect</option>
        </select>
        <input type="number" min="0" class="form-control ft-prio" v-model.number="draft.priority" placeholder="prio">
        <button class="btn btn-primary" :disabled="!draft.pattern" @click="add()">
          <i class="fas fa-plus" style="margin-right:5px"></i>Add
        </button>
      </div>
      <input type="text" class="form-control ft-note-input" placeholder="optional note (e.g. 'CI scanner')" v-model="draft.note">
    </div>
  </div>
</template>

<script>
import api from '../api'

export default {
  name: 'FilterTable',
  props: {
    fileId: { type: Number, default: 0 },
  },
  data() {
    return {
      filters: [],
      draft: { match_type: 'cidr', pattern: '', action: 'deny', priority: 0, note: '' },
    }
  },
  computed: {
    patternPlaceholder() {
      switch (this.draft.match_type) {
        case 'ip': return '1.2.3.4'
        case 'cidr': return '10.0.0.0/8'
        case 'country': return 'RU'
        case 'ua_regex': return '(?i)bot|crawler|sandbox'
        default: return ''
      }
    },
  },
  watch: {
    fileId: {
      immediate: true,
      handler() { this.refresh() },
    },
  },
  methods: {
    refresh() {
      api
        .get('/filters?file_id=' + this.fileId)
        .then((r) => {
          this.filters = r.data.data.filters || []
        })
        .catch((e) => console.log(e))
    },
    add() {
      if (!this.draft.pattern) return
      api
        .post(
          '/filters',
          {
            enabled: true,
            file_id: this.fileId,
            priority: Number(this.draft.priority) || 0,
            match_type: this.draft.match_type,
            pattern: this.draft.pattern,
            action: this.draft.action,
            note: this.draft.note,
          },
          { headers: { 'content-type': 'application/json' } }
        )
        .then(() => {
          this.draft.pattern = ''
          this.draft.note = ''
          this.refresh()
        })
        .catch((e) => console.log(e))
    },
    toggle(f) {
      api
        .put(
          '/filters/' + f.id,
          { ...f, enabled: !f.enabled },
          { headers: { 'content-type': 'application/json' } }
        )
        .then(() => this.refresh())
        .catch((e) => console.log(e))
    },
    remove(f) {
      api
        .delete('/filters/' + f.id)
        .then(() => this.refresh())
        .catch((e) => console.log(e))
    },
  },
}
</script>

<style scoped>
.ft-empty {
  color: var(--pwn-slite);
  text-align: center;
  padding: 10px;
}
.ft-table {
  color: #ddd;
  font-size: 12px;
  margin-bottom: 10px;
}
.ft-table th {
  border-top: none;
  color: var(--pwn-slite);
  font-weight: normal;
}
.ft-table td,
.ft-table th {
  border-color: var(--pwn-black-hr);
  vertical-align: middle;
  padding: 6px 8px;
}
.ft-table code {
  color: var(--pwn-sslite);
  font-size: 11px;
}
.ft-row-off {
  opacity: 0.4;
}
.ft-th-on {
  width: 40px;
}
.ft-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 10px;
  text-transform: uppercase;
  background: var(--pwn-black2);
  color: var(--pwn-sslite);
  letter-spacing: 0.5px;
}
.ft-act-allow { background: rgba(40,167,69,0.2); color:#8bc34a; }
.ft-act-deny { background: rgba(220,53,69,0.2); color:#e57373; }
.ft-act-facade { background: rgba(108,117,125,0.25); color:#adb5bd; }
.ft-act-redirect { background: rgba(255,193,7,0.2); color:#ffc107; }
.ft-note { color: var(--pwn-slite); font-size: 11px; max-width: 140px; overflow:hidden; text-overflow: ellipsis; white-space: nowrap; }
.ft-hits {
  display: inline-block;
  min-width: 28px;
  padding: 2px 6px;
  border-radius: 10px;
  background: rgba(255,193,7,0.15);
  color: #ffc107;
  font-size: 11px;
  text-align: center;
}
.ft-hits-zero {
  background: var(--pwn-black2);
  color: var(--pwn-slite);
}
.ft-add {
  background: var(--pwn-black2);
  border: 1px solid var(--pwn-black-hr);
  border-radius: 12px;
  padding: 10px;
}
.ft-add-row {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 6px;
}
.ft-add-row .form-control { flex: 1; }
.ft-sel { flex: 0 0 110px !important; }
.ft-prio { flex: 0 0 70px !important; }
.ft-note-input { font-size: 12px; }

select.form-control {
  background: var(--pwn-black);
  color: #fff;
  border: 1px solid var(--pwn-med);
  border-radius: 21px;
  font-size: 12px;
  padding: 6px 12px;
}
input.form-control[type="number"]::-webkit-inner-spin-button,
input.form-control[type="number"]::-webkit-outer-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
input.form-control[type="number"] {
  border-radius: 21px;
  background: var(--pwn-black);
  color: #fff;
}
</style>
