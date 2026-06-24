<template>
  <div ref="uploadItem" class="upload-item" :class="{ 'upload-disabled': !file.is_enabled, 'upload-paused': file.is_paused, 'upload-selected': selected }">
    <div class="row upload-desc">
      <div class="settings">
        <span class="btn-col">
          <button
            class="btn btn-sm btn-circle-sm select-btn"
            :class="selected ? 'btn-warning' : 'btn-outline-secondary'"
            @click="$emit('toggleSelect', file.id)"
            v-tooltip:top="'Select for bulk action'">
            <i :class="selected ? 'fas fa-check' : 'far fa-square'"></i>
          </button>
        </span>
        <span class="btn-col">
          <button class="btn btn-sm btn-primary btn-circle-sm" @click="$emit('editFile', file.id)" v-tooltip:top="'Change file settings'">
            <i class="fas fa-cog"></i>
          </button>
        </span>
        <span class="btn-col">
          <button class="btn btn-sm btn-circle-sm" :class="{ 'btn-outline-secondary': !file.is_enabled, 'btn-success': file.is_enabled }" @click="$emit('enableFile', file.id)" v-tooltip:top="'Make file available for download'">
            <i class="fas fa-power-off"></i>
          </button>
        </span>
        <span class="btn-col">
          <button class="btn btn-sm btn-circle-sm" :class="{ 'btn-outline-secondary': !file.is_paused, 'btn-gray': file.is_paused }" @click="$emit('pauseFile', file.id)" v-tooltip:top="'Enable the facade and serve the facade file instead of the original one'">
            <i class="fas fa-mask"></i>
          </button>
        </span>
      </div>
      <div class="col clip trans" :class="{ 'text-dim': file.is_paused }">
        <span class="title">{{ file.name }}</span>
      </div>
      <div v-if="file.sub_file != null && !file.is_paused" class="col-auto shrink">
        <i class="fas fa-arrow-left"></i>
      </div>
      <div v-else-if="file.sub_file != null" class="col-auto shrink">
        <i class="fas fa-arrow-right"></i>
      </div>
      <div v-if="file.sub_file != null" class="col clip trans" :class="{ 'text-dim': !file.is_paused }">
        <span class="title">{{ file.sub_name }}</span>
      </div>
      <div class="d-none d-sm-block col-auto shrink text-right clip">
        <span class="fsize">{{ $prettyBytes(file.fsize) }}</span>
        <span
          v-if="file.download_count > 0 || file.max_downloads > 0"
          class="dl-count"
          :class="quotaHit ? 'dl-count-hit' : ''"
          v-tooltip:top="quotaTip">
          <i class="fas fa-download"></i>
          {{ file.download_count || 0 }}<template v-if="file.max_downloads > 0">/{{ file.max_downloads }}</template>
        </span>
      </div>
      <div class="controls">
        <button class="btn btn-sm btn-danger btn-circle-sm" @click="deleteItem(file.id)">
          <i class="fas fa-times"></i>
        </button>
      </div>
    </div>
    <div class="row upload-info" v-show="!file.progress || file.progress == 100">
      <div class="col-auto shrink clip">
        <span class="btn-col">
          <a class="btn-copy" href @click.prevent="copyHttpUrl()">
            <button class="btn btn-sm btn-outline-success btn-copy-link" v-tooltip:bottom="'Copy HTTP link to clipboard'">
              <i class="fas fa-copy" style="margin-right: 5px"></i>HTTP
            </button>
          </a>
        </span>
        <span class="btn-col">
          <a class="btn-copy" href @click.prevent="copyWebdavUrl()">
            <button class="btn btn-sm btn-outline-success btn-copy-link" v-tooltip:bottom="'Copy WebDAV link to clipboard'">
              <i class="fas fa-copy" style="margin-right: 5px"></i>WebDAV
            </button>
          </a>
        </span>
        <span class="btn-col">
          <button class="btn btn-sm btn-outline-success btn-copy-link" @click="$emit('showQr', file.id)" v-tooltip:bottom="'Show QR code for the HTTP link'">
            <i class="fas fa-qrcode"></i>
          </button>
        </span>
        <span class="btn-col">
          <button class="btn btn-sm btn-outline-warning btn-copy-link" @click="$emit('rotateFile', file.id)" v-tooltip:bottom="'Rotate URL — generate a new random path (kept payload, password, filters, counters)'">
            <i class="fas fa-sync-alt"></i>
          </button>
        </span>
      </div>
      <div class="col-auto grow trans" :class="{ 'text-lg': file.is_paused }">
        <small>{{ file.url_path }}</small>
      </div>
      <div class="d-none d-sm-block col text-right clip trans" :class="{ 'text-lg': file.is_paused }">
        <small>{{ file.mime_type }}</small>
      </div>
    </div>
    <div class="row">
      <div class="file-progress col" v-if="file.progress < 100">
        <div class="progress">
          <div
            class="progress-bar progress-bar-striped progress-bar-animated bg-success"
            role="progressbar"
            :style="{ width: file.progress + '%' }"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-valuenow="file.progress"
          ></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'FileItem',
  props: {
    file: { type: Object, required: true },
    selected: { type: Boolean, default: false },
  },
  emits: ['editFile', 'enableFile', 'pauseFile', 'deleteFile', 'toggleSelect', 'showQr', 'rotateFile'],
  computed: {
    quotaHit() {
      return this.file.max_downloads > 0 && this.file.download_count >= this.file.max_downloads
    },
    quotaTip() {
      if (this.file.max_downloads > 0) {
        return `${this.file.download_count || 0} of ${this.file.max_downloads} downloads used`
      }
      return `${this.file.download_count || 0} downloads served`
    },
  },
  methods: {
    copyHttpUrl() {
      const l = window.location
      let url = l.protocol + '//' + l.hostname
      if (l.port !== '' && l.port != 443 && l.port != 80) {
        url += ':' + l.port
      }
      url += encodeURI(this.file.url_path)
      this.copyToClipboard(url)
    },
    copyWebdavUrl() {
      const l = window.location
      let url = '\\\\' + l.hostname + '@80'
      url += encodeURI(this.file.url_path).replace(/\//g, '\\')
      this.copyToClipboard(url)
    },
    copyToClipboard(text) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).catch((e) => console.log(e))
      }
    },
    deleteItem(id) {
      this.$refs.uploadItem.style.width = this.$refs.uploadItem.offsetWidth + 'px'
      this.$emit('deleteFile', id)
    },
  },
}
</script>

<style scoped>
/* Four circle buttons (select, cog, power, mask) need more horizontal room
   than the original three. Scoped so it only widens this component's row.
   The .settings block is absolute-positioned by the global stylesheet, so
   we only push the title's left padding outward. */
.upload-item .upload-desc {
  padding-left: 150px;
}
.upload-selected {
  outline: 2px solid #ffc107;
  outline-offset: -2px;
}
/* Make the select-button slightly more discoverable when it's "off". */
.select-btn {
  background: transparent;
}
.dl-count {
  display: inline-block;
  margin-left: 10px;
  padding: 1px 7px;
  font-size: 11px;
  line-height: 1.4;
  border-radius: 10px;
  background: var(--pwn-black-hr);
  color: var(--pwn-slite);
  vertical-align: middle;
}
.dl-count i {
  font-size: 10px;
  margin-right: 3px;
  opacity: 0.7;
}
.dl-count-hit {
  background: #6b3030;
  color: #ffb3b3;
}
</style>
