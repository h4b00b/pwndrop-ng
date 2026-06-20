import { reactive } from 'vue'

// Shared session state, replacing the old global Vue event bus (mainBus).
export const session = reactive({
  isLoggedIn: false,
  username: '',
})
