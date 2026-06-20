<template>
  <teleport to="body">
    <div v-if="modelValue">
      <div class="modal-backdrop fade show"></div>
      <div class="modal fade show" style="display: block" tabindex="-1" @click.self="close">
        <div class="modal-dialog" :class="{ 'modal-lg': size === 'lg' }">
          <div class="modal-content">
            <div v-if="!hideHeader" class="modal-header">
              <h5 class="modal-title">{{ title }}</h5>
              <button type="button" class="close" aria-label="Close" @click="close">
                <span aria-hidden="true">&times;</span>
              </button>
            </div>
            <div class="modal-body">
              <slot></slot>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" @click="close">Cancel</button>
              <button type="button" class="btn btn-primary" :disabled="okDisabled" @click="$emit('ok')">
                {{ okTitle }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </teleport>
</template>

<script>
// Minimal Bootstrap-4-styled modal, replacing bootstrap-vue's <b-modal>.
// Controlled via v-model; the OK button emits "ok" without auto-closing so the
// parent can close it only after a successful action.
export default {
  name: 'Modal',
  props: {
    modelValue: { type: Boolean, default: false },
    title: { type: String, default: '' },
    size: { type: String, default: '' },
    hideHeader: { type: Boolean, default: false },
    okTitle: { type: String, default: 'OK' },
    okDisabled: { type: Boolean, default: false },
  },
  emits: ['update:modelValue', 'ok'],
  methods: {
    close() {
      this.$emit('update:modelValue', false)
    },
  },
}
</script>
