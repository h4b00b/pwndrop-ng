<template>
  <div>
    <Modal
      v-model="editShow"
      title="Edit"
      size="lg"
      hide-header
      ok-title="Save"
      :ok-disabled="!isComplete"
      @ok="updateFile()"
    >
      <form>
        <div class="form-group row">
          <label for="edit-name" class="col-sm-3 col-form-label label-help">Name:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Friendly name for your eyes only'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="edit-name" spellcheck="false" v-model="file_edit.name">
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-mime" class="col-sm-3 col-form-label label-help">
            <a class="help-link" href="https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types" target="_blank">MIME Type:</a>
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'File will be retrieved with the following MIME type'"></i>
          </label>
          <div class="col-sm-9">
            <div class="input-group">
              <input type="text" class="form-control" id="edit-mime" spellcheck="false" v-model="file_edit.mime_type">
              <div class="input-group-append">
                <button class="btn btn-secondary" type="button" @click="file_edit.mime_type = file_edit.orig_mime_type">
                  <i class="fas fa-undo"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-http-path" class="col-sm-3 col-form-label label-help">Path:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'URL path when sharing it over HTTP or WebDAV. Paths for WebDAV must be under a subdirectory (e.g. /subdir/payload.docx)'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="edit-http-path" spellcheck="false" v-model="file_edit.url_path">
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-redirect-path" class="col-sm-3 col-form-label label-help">Redirect Path:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'URL path which the request will be redirected to. Useful if you want to spoof the extension (e.g. /subdir/payload.docx.exe)'"></i>
          </label>
          <div class="col-sm-9">
            <div class="input-group">
              <input type="text" class="form-control" id="edit-redirect-path" spellcheck="false" v-model="file_edit.redirect_path">
              <div class="input-group-append">
                <button class="btn btn-secondary" type="button" @click="file_edit.redirect_path = file_edit.url_path">
                  <i class="fas fa-copy"></i>
                </button>
              </div>
            </div>
          </div>
        </div>

        <hr>
        <h6 class="policy-title">Delivery controls</h6>
        <div class="form-group row">
          <label for="edit-expire" class="col-sm-3 col-form-label label-help">Expires:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'After this moment the link returns 404. Leave empty for no expiry.'"></i>
          </label>
          <div class="col-sm-9">
            <div class="input-group">
              <input type="datetime-local" class="form-control" id="edit-expire" v-model="file_edit.expire_local">
              <div class="input-group-append">
                <button class="btn btn-secondary" type="button" @click="file_edit.expire_local = ''" v-tooltip:left="'Clear'">
                  <i class="fas fa-times"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-max" class="col-sm-3 col-form-label label-help">Max downloads:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Auto-disable after N successful downloads. 0 = unlimited. Set to 1 for a one-time link.'"></i>
          </label>
          <div class="col-sm-9">
            <div class="input-group">
              <input type="number" min="0" class="form-control" id="edit-max" v-model.number="file_edit.max_downloads">
              <div class="input-group-append">
                <span class="input-group-text">used: {{ file_edit.download_count }}</span>
              </div>
            </div>
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-pwd" class="col-sm-3 col-form-label label-help">Password:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Downloader must pass HTTP Basic auth (any username, password = this value). Leave blank to keep current; tick the box to remove.'"></i>
          </label>
          <div class="col-sm-9">
            <div class="input-group">
              <input type="text" class="form-control" id="edit-pwd" spellcheck="false" :placeholder="file_edit.has_password ? '<unchanged — currently set>' : 'no password'" v-model="file_edit.password">
              <div class="input-group-append" v-if="file_edit.has_password">
                <span class="input-group-text">
                  <input type="checkbox" id="edit-pwd-clear" v-model="file_edit.clear_password">
                  <label for="edit-pwd-clear" style="margin-left:6px;margin-bottom:0">remove</label>
                </span>
              </div>
            </div>
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-mute" class="col-sm-3 col-form-label label-help">Mute notifications:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'When on, downloads of this file are still logged but no outbound notification (webhook/Telegram/Slack) is sent.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="edit-mute" v-model="file_edit.notify_muted">
          </div>
        </div>
        <div class="form-group row">
          <label for="edit-burn" class="col-sm-3 col-form-label label-help">Burn after read:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'After the first successful download, the file record and blob are permanently deleted from the server.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="edit-burn" v-model="file_edit.burn_after_read">
          </div>
        </div>
        <div class="form-group row">
          <label class="col-sm-3 col-form-label label-help">Replace file:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Swap the on-disk content while keeping URL, redirect, policy, password and filters. Resets the download counter to 0.'"></i>
          </label>
          <div class="col-sm-9">
            <input type="file" ref="replaceInput" style="display:none" @change="replaceFile($event)">
            <button type="button" class="btn btn-sm btn-secondary" @click="$refs.replaceInput.click()">
              <i class="fas fa-upload" style="margin-right:5px"></i>Choose new file…
            </button>
            <span v-if="file_edit.replace_status" class="replace-status">{{ file_edit.replace_status }}</span>
          </div>
        </div>
        <hr>
        <h6 class="policy-title">Target filters (per-file)</h6>
        <p class="policy-hint">
          Evaluated before the global rules. If none match here, the global chain runs next.
        </p>
        <FilterTable v-if="file_edit.id > 0" :file-id="file_edit.id" />

        <hr>
        <transition name="sub-modal-anim" mode="out-in">
          <div class="row" v-if="file_edit.sub_progress < 100" key="uploading">
            <div class="file-progress col">
              <div class="progress">
                <div
                  class="progress-bar progress-bar-striped progress-bar-animated bg-success"
                  role="progressbar"
                  :style="{ width: file_edit.sub_progress + '%' }"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="file_edit.sub_progress"
                ></div>
              </div>
            </div>
          </div>
          <div class="row" v-else-if="file_edit.ref_sub_file == 0" key="empty">
            <div class="sub-info">
              <small>Upload a facade file, which will be served instead of the original one, only when facade is enabled.</small>
            </div>
            <div id="sub-dropzone" :class="[isSubDragging ? 'drag' : '']">
              <span class="icon">
                <i class="fas fa-upload"></i>
              </span>
              <input
                type="file"
                @change="handleSubFile($event)"
                @dragover.prevent
                @dragenter="isSubDragging = true"
                @drop.prevent="handleSubDrop($event)"
                @dragleave="isSubDragging = false"
              >
            </div>
          </div>
          <div class="sub-item" v-else key="uploaded">
            <div class="sub-info">
              <small>Facade file</small>
            </div>
            <div class="form-group row desc">
              <label for="edit-sub-name" class="col-sm-3 col-form-label label-help">Name:
                <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Facade name for display purposes only'"></i>
              </label>
              <div class="col">
                <input type="text" class="form-control" spellcheck="false" v-model="file_edit.sub_name">
              </div>
              <div class="d-none d-sm-block col-auto shrink">
                <span class="fsize">{{ $prettyBytes(file_edit.sub_size) }}</span>
              </div>
              <div class="controls">
                <button class="btn btn-sm btn-danger btn-circle-sm" @click.prevent="deleteSubFile(file_edit.id, file_edit.ref_sub_file)">
                  <i class="fas fa-times"></i>
                </button>
              </div>
            </div>
            <div class="form-group row desc">
              <label for="edit-sub-mime" class="col-sm-3 col-form-label label-help">
                <a class="help-link" href="https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types" target="_blank">MIME Type:</a>
                <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Facade file will be retrieved with the following MIME type'"></i>
              </label>
              <div class="col-sm-9">
                <div class="input-group">
                  <input type="text" class="form-control" id="edit-sub-mime" spellcheck="false" v-model="file_edit.sub_mime_type">
                  <div class="input-group-append">
                    <button class="btn btn-secondary" type="button" @click="file_edit.sub_mime_type = file_edit.orig_mime_type">
                      <i class="fas fa-undo"></i>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </transition>
      </form>
    </Modal>

    <div
      id="dropzone"
      :class="[isDragging ? 'drag' : '']"
      @dragover.prevent
      @dragenter="isDragging = true"
      @drop.prevent="handleDrop($event)"
      @dragleave="isDragging = false"
    ></div>

    <!-- QR code preview modal -->
    <Modal v-model="qrShow" title="QR code" size="sm" ok-title="Done" @ok="qrShow = false" @update:model-value="qrShow = false">
      <div class="qr-wrap">
        <canvas ref="qrCanvas"></canvas>
        <div class="qr-url"><code>{{ qrUrl }}</code></div>
      </div>
    </Modal>

    <!-- Bulk action bar -->
    <div v-if="selected.length > 0" class="bulk-bar">
      <span class="bulk-count">{{ selected.length }} selected</span>
      <button class="btn btn-sm btn-success" @click="bulk('enable')">
        <i class="fas fa-power-off"></i> Enable
      </button>
      <button class="btn btn-sm btn-secondary" @click="bulk('disable')">
        <i class="fas fa-power-off"></i> Disable
      </button>
      <button class="btn btn-sm btn-info" @click="bulk('pause')">
        <i class="fas fa-mask"></i> Facade
      </button>
      <button class="btn btn-sm btn-info" @click="bulk('unpause')">
        <i class="fas fa-mask"></i> Unfacade
      </button>
      <button class="btn btn-sm btn-danger" @click="bulkDelete()">
        <i class="fas fa-trash"></i> Delete
      </button>
      <button class="btn btn-sm btn-outline-secondary" @click="selected = []">
        Clear
      </button>
    </div>

    <div class="row row-file">
      <div class="col-xs-12 col-sm-8 offset-sm-2 col-md-6 offset-md-3">
        <form enctype="multipart/form-data" novalidate>
          <button class="btn btn-lg btn-primary btn-file btn-block">
            Upload
            <input type="file" multiple @change="handleFiles($event)">
          </button>
        </form>
        <button type="button" class="btn btn-lg btn-secondary btn-block paste-btn" @click="openPaste()">
          <i class="fas fa-paste" style="margin-right:8px"></i>Paste text
        </button>
      </div>
    </div>

    <Modal
      v-model="pasteShow"
      title="Paste text"
      size="lg"
      hide-header
      ok-title="Create"
      :ok-disabled="!paste.name || !paste.mime_type"
      @ok="createPaste()"
    >
      <form>
        <div class="form-group row">
          <label for="paste-name" class="col-sm-3 col-form-label label-help">Name:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Filename shown at the end of the URL. The extension drives how the browser handles it.'"></i>
          </label>
          <div class="col-sm-9">
            <input type="text" class="form-control" id="paste-name" spellcheck="false" v-model="paste.name">
          </div>
        </div>
        <div class="form-group row">
          <label for="paste-mime" class="col-sm-3 col-form-label label-help">MIME type:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Common: text/plain, application/json, text/x-shellscript, application/x-powershell, text/html'"></i>
          </label>
          <div class="col-sm-9">
            <div class="input-group">
              <select class="form-control paste-mime-select" v-model="paste.mime_type">
                <option value="text/plain; charset=utf-8">text/plain</option>
                <option value="application/json">application/json</option>
                <option value="text/x-shellscript">text/x-shellscript (.sh)</option>
                <option value="application/x-powershell">application/x-powershell (.ps1)</option>
                <option value="application/javascript">application/javascript</option>
                <option value="text/html">text/html</option>
                <option value="application/xml">application/xml</option>
                <option value="text/x-python">text/x-python</option>
                <option value="__custom__">Custom…</option>
              </select>
            </div>
            <input v-if="paste.mime_type === '__custom__'" type="text" class="form-control paste-mime-custom" spellcheck="false" placeholder="application/octet-stream" v-model="paste.mime_custom">
          </div>
        </div>
        <div class="form-group row">
          <label for="paste-burn" class="col-sm-3 col-form-label label-help">Burn after read:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'After the first successful download, the paste is permanently deleted from the server.'"></i>
          </label>
          <div class="col-sm-9 d-flex align-items-center">
            <input type="checkbox" id="paste-burn" v-model="paste.burn_after_read">
          </div>
        </div>
        <div class="form-group row">
          <label for="paste-content" class="col-sm-3 col-form-label label-help">Content:
            <i class="fas fa-question-circle label-qmark" v-tooltip:bottom="'Raw text body — served as-is at the public URL.'"></i>
          </label>
          <div class="col-sm-9">
            <textarea id="paste-content" class="form-control paste-textarea" spellcheck="false" v-model="paste.content" rows="14" placeholder="Paste your content here…"></textarea>
            <small class="paste-hint">{{ paste.content.length }} chars</small>
          </div>
        </div>
      </form>
    </Modal>

    <div class="row row-info">
      <div class="server-status col-xs-12 col-sm-8 offset-sm-2 col-md-6 offset-md-3">
        free: <strong>{{ $prettyBytes(server_info.disk_free) }}</strong> &bull; used: <strong>{{ $prettyBytes(server_info.disk_used) }}</strong>
      </div>
    </div>

    <transition-group name="upload-list">
      <div class="row upload-block" v-for="upload in uploads" :key="upload.key">
        <FileItem
          :file="upload"
          :selected="selected.includes(upload.id)"
          @editFile="editFile"
          @deleteFile="deleteFile"
          @enableFile="enableFile"
          @pauseFile="pauseFile"
          @toggleSelect="toggleSelect"
          @showQr="showQr"
          @rotateFile="rotateFile"
        ></FileItem>
      </div>
    </transition-group>
  </div>
</template>

<script>
import api from '../api'
import QRCode from 'qrcode'
import Modal from './Modal.vue'
import FileItem from './FileItem.vue'
import FilterTable from './FilterTable.vue'

export default {
  name: 'FileView',
  components: { Modal, FileItem, FilterTable },
  data() {
    return {
      isDragging: false,
      isSubDragging: false,
      editShow: false,
      uploads: [],
      next_key: 0,
      file_edit: {
        create_time: 0,
        fsize: 0,
        id: 0,
        mime_type: '',
        sub_mime_type: '',
        name: '',
        orig_mime_type: '',
        ref_sub_file: 0,
        sub_ctime: 0,
        sub_name: '',
        sub_progress: 100,
        sub_size: 0,
        url_path: '',
        redirect_path: '',
        wdav_path: '',
        expire_at: 0,
        expire_local: '',
        max_downloads: 0,
        download_count: 0,
        has_password: false,
        password: '',
        clear_password: false,
        notify_muted: false,
        burn_after_read: false,
        replace_status: '',
      },
      selected: [],
      qrShow: false,
      qrUrl: '',
      pasteShow: false,
      paste: {
        name: 'paste.txt',
        mime_type: 'text/plain; charset=utf-8',
        mime_custom: '',
        burn_after_read: false,
        content: '',
      },
      server_info: {
        disk_free: 0,
        disk_used: 0,
      },
    }
  },
  computed: {
    isComplete() {
      return !!(
        this.file_edit.name &&
        this.file_edit.mime_type &&
        this.file_edit.url_path &&
        (this.file_edit.ref_sub_file == 0 || (this.file_edit.sub_name && this.file_edit.sub_mime_type))
      )
    },
  },
  methods: {
    handleFiles($event) {
      const files = []
      for (let i = 0; i < $event.target.files.length; i++) {
        files.push($event.target.files[i])
      }
      if (files.length > 0) {
        this.uploadFiles(files)
      }
    },
    handleDrop($event) {
      this.isDragging = false
      const files = []
      if ($event.dataTransfer.items) {
        for (let i = 0; i < $event.dataTransfer.items.length; i++) {
          files.push($event.dataTransfer.items[i].getAsFile())
        }
      } else {
        for (let i = 0; i < $event.dataTransfer.files.length; i++) {
          files.push($event.dataTransfer.files[i])
        }
      }
      if (files.length > 0) {
        this.uploadFiles(files)
      }
    },
    uploadFiles(files) {
      const vm = this
      const ctime = new Date().getTime()
      for (let i = 0; i < files.length; i++) {
        const file = files[i]
        const item_id = ctime + i
        const item = {
          id: item_id,
          name: file.name,
          fsize: file.size,
          mime_type: file.type,
          sub_mime_type: file.sub_mime_type,
          orig_mime_type: file.type,
          url_path: '',
          redirect_path: '',
          wdav_path: '',
          progress: 0,
          key: this.next_key,
          is_enabled: true,
          is_paused: false,
          sub_name: '',
          sub_file: null,
        }
        vm.uploads.push(item)
        this.next_key += 1

        const formData = new FormData()
        formData.append('file', file)
        api
          .post('/files', formData, {
            headers: { 'content-type': 'multipart/form-data' },
            onUploadProgress(progressEvent) {
              const j = vm.findFileIndexById(item_id)
              if (j != -1 && progressEvent.total) {
                vm.uploads[j].progress = Math.floor((progressEvent.loaded / progressEvent.total) * 100)
              }
            },
          })
          .then((response) => {
            const j = vm.findFileIndexById(item_id)
            if (j != -1) {
              const it = response.data.data
              const ut = vm.uploads[j]
              ut.id = it.id
              ut.url_path = it.url_path
              ut.redirect_path = it.redirect_path
              ut.wdav_path = it.wdav_path
              ut.mime_type = it.mime_type
              ut.sub_mime_type = it.sub_mime_type
              ut.orig_mime_type = it.orig_mime_type
              ut.progress = 100
            }
            vm.syncServerInfo()
          })
          .catch((error) => {
            console.log(error)
            const j = vm.findFileIndexById(item_id)
            if (j != -1) {
              vm.uploads.splice(j, 1)
            }
          })
      }
    },
    handleSubFile($event) {
      const files = []
      for (let i = 0; i < $event.target.files.length; i++) {
        files.push($event.target.files[i])
        break
      }
      if (files.length > 0) {
        this.uploadSubFiles(this.file_edit.id, files)
      }
    },
    handleSubDrop($event) {
      this.isSubDragging = false
      const files = []
      if ($event.dataTransfer.items) {
        for (let i = 0; i < $event.dataTransfer.items.length; i++) {
          files.push($event.dataTransfer.items[i].getAsFile())
          break
        }
      } else {
        for (let i = 0; i < $event.dataTransfer.files.length; i++) {
          files.push($event.dataTransfer.files[i])
          break
        }
      }
      if (files.length > 0) {
        this.uploadSubFiles(this.file_edit.id, files)
      }
    },
    uploadSubFiles(parent_id, files) {
      const vm = this
      for (let i = 0; i < files.length; i++) {
        const file = files[i]
        const formData = new FormData()
        formData.append('file', file)
        api
          .post('/files/' + parent_id + '/sub', formData, {
            headers: { 'content-type': 'multipart/form-data' },
            onUploadProgress(progressEvent) {
              if (progressEvent.total) {
                vm.file_edit.sub_progress = Math.floor((progressEvent.loaded / progressEvent.total) * 100)
              }
            },
          })
          .then((response) => {
            vm.file_edit.sub_progress = 100
            const it = response.data.data
            vm.file_edit.ref_sub_file = it.id
            vm.file_edit.sub_name = it.name
            vm.file_edit.sub_size = it.fsize
            vm.file_edit.sub_ctime = it.create_time

            const j = vm.findFileIndexById(parent_id)
            if (j != -1) {
              const f = vm.uploads[j]
              f.ref_sub_file = it.id
              f.sub_file = {
                create_time: it.create_time,
                fid: parent_id,
                fname: it.fname,
                fsize: it.fsize,
                id: it.id,
                name: it.name,
                uid: it.uid,
              }
              f.sub_name = it.name
            }
            vm.syncServerInfo()
          })
          .catch((error) => console.log(error))
      }
    },
    editFile(id) {
      const i = this.findFileIndexById(id)
      if (i == -1) {
        console.log('file not found: ' + id)
        return
      }
      this.file_edit.id = id
      this.file_edit.name = this.uploads[i].name
      this.file_edit.mime_type = this.uploads[i].mime_type
      this.file_edit.sub_mime_type = this.uploads[i].sub_mime_type
      this.file_edit.orig_mime_type = this.uploads[i].orig_mime_type
      this.file_edit.url_path = this.uploads[i].url_path
      this.file_edit.redirect_path = this.uploads[i].redirect_path
      this.file_edit.wdav_path = this.uploads[i].wdav_path
      this.file_edit.ref_sub_file = this.uploads[i].ref_sub_file
      this.file_edit.expire_at = this.uploads[i].expire_at || 0
      this.file_edit.expire_local = this.tsToLocalInput(this.uploads[i].expire_at)
      this.file_edit.max_downloads = this.uploads[i].max_downloads || 0
      this.file_edit.download_count = this.uploads[i].download_count || 0
      this.file_edit.has_password = !!this.uploads[i].has_password
      this.file_edit.password = ''
      this.file_edit.clear_password = false
      this.file_edit.notify_muted = !!this.uploads[i].notify_muted
      this.file_edit.burn_after_read = !!this.uploads[i].burn_after_read
      this.file_edit.replace_status = ''
      this.file_edit.sub_name = '<unknown>'
      this.file_edit.sub_size = 0
      this.file_edit.sub_ctime = 0
      this.file_edit.sub_progress = 100
      if (this.uploads[i].sub_file) {
        this.file_edit.sub_name = this.uploads[i].sub_name
        this.file_edit.sub_size = this.uploads[i].sub_file.fsize
        this.file_edit.sub_ctime = this.uploads[i].sub_file.create_time
      } else {
        this.file_edit.ref_sub_file = 0
      }
      this.editShow = true
    },
    updateFile() {
      const vm = this
      if (!this.file_edit || !this.isComplete) {
        return
      }
      const id = this.file_edit.id
      api
        .put(
          '/files/' + id,
          {
            name: this.file_edit.name,
            url_path: this.file_edit.url_path,
            redirect_path: this.file_edit.redirect_path,
            mime_type: this.file_edit.mime_type,
            sub_mime_type: this.file_edit.sub_mime_type,
            sub_name: this.file_edit.sub_name,
            expire_at: this.localInputToTs(this.file_edit.expire_local),
            max_downloads: Number(this.file_edit.max_downloads) || 0,
            password: this.file_edit.password,
            clear_password: this.file_edit.clear_password,
            notify_muted: this.file_edit.notify_muted,
            burn_after_read: this.file_edit.burn_after_read,
          },
          { headers: { 'content-type': 'application/json' } }
        )
        .then((response) => {
          vm.editShow = false
          const i = vm.findFileIndexById(id)
          if (i != -1) {
            const f = response.data.data
            vm.uploads[i].name = f.name
            vm.uploads[i].sub_name = f.sub_name
            vm.uploads[i].url_path = f.url_path
            vm.uploads[i].redirect_path = f.redirect_path
            vm.uploads[i].mime_type = f.mime_type
            vm.uploads[i].sub_mime_type = f.sub_mime_type
            vm.uploads[i].expire_at = f.expire_at
            vm.uploads[i].max_downloads = f.max_downloads
            vm.uploads[i].download_count = f.download_count
            vm.uploads[i].has_password = !!f.has_password
            vm.uploads[i].notify_muted = !!f.notify_muted
            vm.uploads[i].burn_after_read = !!f.burn_after_read
          }
        })
        .catch((error) => console.log(error))
    },
    openPaste() {
      this.paste = {
        name: 'paste.txt',
        mime_type: 'text/plain; charset=utf-8',
        mime_custom: '',
        burn_after_read: false,
        content: '',
      }
      this.pasteShow = true
    },
    createPaste() {
      const vm = this
      let mime = this.paste.mime_type
      if (mime === '__custom__') {
        mime = (this.paste.mime_custom || '').trim() || 'application/octet-stream'
      }
      api
        .post(
          '/files/paste',
          {
            name: this.paste.name,
            mime_type: mime,
            content: this.paste.content,
            burn_after_read: this.paste.burn_after_read,
          },
          { headers: { 'content-type': 'application/json' } }
        )
        .then((response) => {
          const it = response.data.data
          const item = {
            ...it,
            key: vm.next_key,
            progress: 100,
            sub_file: null,
          }
          vm.next_key += 1
          vm.uploads.unshift(item)
          vm.pasteShow = false
          vm.syncServerInfo()
        })
        .catch((error) => console.log(error))
    },
    replaceFile($event) {
      const file = $event.target.files && $event.target.files[0]
      if (!file) return
      const id = this.file_edit.id
      const fd = new FormData()
      fd.append('file', file)
      this.file_edit.replace_status = 'uploading…'
      api
        .post('/files/' + id + '/replace', fd, { headers: { 'content-type': 'multipart/form-data' } })
        .then((response) => {
          const f = response.data.data
          this.file_edit.replace_status = 'replaced (' + this.$prettyBytes(f.fsize) + ')'
          const i = this.findFileIndexById(id)
          if (i != -1) {
            this.uploads[i].fsize = f.fsize
            this.uploads[i].mime_type = f.mime_type
            this.uploads[i].download_count = 0
            this.file_edit.download_count = 0
          }
          this.syncServerInfo()
        })
        .catch((error) => {
          console.log(error)
          this.file_edit.replace_status = 'replace failed'
        })
    },
    toggleSelect(id) {
      const i = this.selected.indexOf(id)
      if (i >= 0) this.selected.splice(i, 1)
      else this.selected.push(id)
    },
    bulk(action) {
      const ids = [...this.selected]
      api
        .post('/files/bulk', { action, ids }, { headers: { 'content-type': 'application/json' } })
        .then(() => {
          this.selected = []
          this.refresh()
          this.syncServerInfo()
        })
        .catch((e) => console.log(e))
    },
    bulkDelete() {
      if (!confirm('Delete ' + this.selected.length + ' file(s)? This cannot be undone.')) return
      this.bulk('delete')
    },
    showQr(id) {
      const i = this.findFileIndexById(id)
      if (i === -1) return
      const l = window.location
      let url = l.protocol + '//' + l.hostname
      if (l.port !== '' && l.port != 443 && l.port != 80) url += ':' + l.port
      url += encodeURI(this.uploads[i].url_path)
      this.qrUrl = url
      this.qrShow = true
      // Wait for the modal canvas to mount before drawing.
      this.$nextTick(() => {
        const cvs = this.$refs.qrCanvas
        if (cvs) {
          QRCode.toCanvas(cvs, url, { width: 256, margin: 1 }, (e) => { if (e) console.log(e) })
        }
      })
    },
    tsToLocalInput(ts) {
      if (!ts) return ''
      const d = new Date(ts * 1000)
      const pad = (n) => String(n).padStart(2, '0')
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    },
    localInputToTs(s) {
      if (!s) return 0
      const t = new Date(s).getTime()
      return isNaN(t) ? 0 : Math.floor(t / 1000)
    },
    rotateFile(id) {
      const i = this.findFileIndexById(id)
      if (i == -1) return
      const oldPath = this.uploads[i].url_path
      if (!confirm('Rotate the public URL for "' + this.uploads[i].name + '"? Anyone using the current link (' + oldPath + ') will get 404.')) return
      api
        .post('/files/' + id + '/rotate')
        .then((response) => {
          const j = this.findFileIndexById(id)
          if (j != -1) {
            this.uploads[j].url_path = response.data.data.url_path
          }
        })
        .catch((error) => console.log(error))
    },
    deleteFile(id) {
      api
        .delete('/files/' + id)
        .then(() => {
          const i = this.findFileIndexById(id)
          if (i != -1) {
            this.uploads.splice(i, 1)
          }
          this.syncServerInfo()
        })
        .catch((error) => console.log(error))
    },
    enableFile(id) {
      const i = this.findFileIndexById(id)
      if (i == -1) {
        return
      }
      let path = '/enable'
      if (this.uploads[i].is_enabled) {
        path = '/disable'
      }
      api
        .post('/files/' + id + path)
        .then((response) => {
          const j = this.findFileIndexById(id)
          if (j != -1) {
            const f = response.data.data
            this.uploads[j].is_enabled = f.is_enabled
            this.uploads[j].is_paused = f.is_paused
          }
        })
        .catch((error) => console.log(error))
    },
    pauseFile(id) {
      const i = this.findFileIndexById(id)
      if (i == -1) {
        return
      }
      if (this.uploads[i].sub_file == null) {
        this.uploads[i].is_paused = false
        return
      }
      let path = '/pause'
      if (this.uploads[i].is_paused) {
        path = '/unpause'
      }
      api
        .post('/files/' + id + path)
        .then((response) => {
          const j = this.findFileIndexById(id)
          if (j != -1) {
            const f = response.data.data
            this.uploads[j].is_enabled = f.is_enabled
            this.uploads[j].is_paused = f.is_paused
          }
        })
        .catch((error) => console.log(error))
    },
    deleteSubFile(parent_id, sub_id) {
      api
        .delete('/files/' + parent_id + '/sub/' + sub_id)
        .then(() => {
          this.file_edit.ref_sub_file = 0
          const i = this.findFileIndexById(parent_id)
          if (i != -1) {
            this.uploads[i].ref_sub_file = 0
            this.uploads[i].sub_name = ''
            this.uploads[i].sub_file = null
            this.uploads[i].is_paused = false
          }
          this.syncServerInfo()
        })
        .catch((error) => console.log(error))
    },
    findFileIndexById(id) {
      let ret = -1
      this.uploads.forEach(function (it, i) {
        if (it.id == id) {
          ret = i
        }
      })
      return ret
    },
    refresh() {
      api
        .get('/files')
        .then((response) => {
          const files = response.data.data.uploads
          this.uploads = []
          let i = 0
          for (i = 0; i < files.length; i++) {
            this.uploads.push(files[i])
            this.uploads[i].key = i
          }
          this.next_key = i + 1
        })
        .catch((error) => console.log(error))
    },
    syncServerInfo() {
      api
        .get('/server_info')
        .then((response) => {
          const r = response.data.data
          this.server_info.disk_free = r.disk_free
          this.server_info.disk_used = r.disk_used
        })
        .catch((error) => console.log(error))
    },
  },
  created() {
    const t = this
    window.addEventListener('dragenter', function () {
      if (!t.editShow) {
        t.isDragging = true
      }
    })
    this.syncServerInfo()
    this.refresh()
  },
}
</script>

<style scoped>
.policy-title {
  color: var(--pwn-slite);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 1px;
  margin: 0 0 10px 0;
}
.policy-hint {
  font-size: 12px;
  color: var(--pwn-slite);
  margin: 0 0 8px 0;
}
.replace-status {
  margin-left: 10px;
  font-size: 12px;
  color: var(--pwn-slite);
}
.bulk-bar {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--pwn-black2);
  border: 1px solid var(--pwn-black-hr);
  border-radius: 30px;
  padding: 8px 14px;
  display: flex;
  gap: 6px;
  align-items: center;
  z-index: 1000;
  box-shadow: 0 4px 16px rgba(0,0,0,0.5);
}
.bulk-count {
  color: var(--pwn-slite);
  font-size: 12px;
  margin-right: 6px;
}
.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}
.qr-wrap canvas {
  border-radius: 8px;
  background: #fff;
  padding: 6px;
}
.qr-url {
  font-size: 11px;
  word-break: break-all;
  text-align: center;
}
.qr-url code { color: var(--pwn-sslite); }
.input-group-text {
  background: var(--pwn-black2);
  color: #fff;
  border-color: var(--pwn-black-hr);
  font-size: 12px;
  border-radius: 21px;
}
/* Match the global rounded look on inputs that Bootstrap squares-off because
   they sit inside an .input-group. We restore the radius and the dark colors,
   and re-skin the native datetime calendar icon so it stays visible. */
input.form-control[type="datetime-local"],
input.form-control[type="number"] {
  background: var(--pwn-black);
  color: #fff;
  border: 1px solid var(--pwn-med);
  border-radius: 21px;
  padding: 6px 14px;
}
input.form-control[type="datetime-local"]::-webkit-calendar-picker-indicator {
  filter: invert(0.85);
  cursor: pointer;
}
.paste-btn {
  margin-top: 10px;
}
.paste-textarea {
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 13px;
  background: var(--pwn-black);
  color: #fff;
  border: 1px solid var(--pwn-med);
  border-radius: 12px;
  min-height: 280px;
  white-space: pre;
  overflow: auto;
}
.paste-hint {
  display: block;
  text-align: right;
  color: var(--pwn-slite);
  font-size: 11px;
  margin-top: 4px;
}
.paste-mime-select {
  background: var(--pwn-black);
  color: #fff;
  border: 1px solid var(--pwn-med);
  border-radius: 21px;
  padding: 6px 14px;
}
.paste-mime-custom {
  margin-top: 8px;
}
/* Hide the number spinner so the field stays clean on Chromium. */
input.form-control[type="number"]::-webkit-inner-spin-button,
input.form-control[type="number"]::-webkit-outer-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
</style>
