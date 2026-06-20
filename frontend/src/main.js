import { createApp } from 'vue'

// Styles. Order matters: pwndrop's style.css overrides Bootstrap, so it loads last.
import 'bootstrap/dist/css/bootstrap.min.css'
import '@fortawesome/fontawesome-free/css/all.min.css'
import './assets/woff.css'
import './assets/woff2.css'
import './assets/style.css'

import App from './App.vue'
import router from './router'
import { prettyBytes } from './utils'
import tooltip from './directives/tooltip'

const app = createApp(App)
app.use(router)
app.directive('tooltip', tooltip)
app.config.globalProperties.$prettyBytes = prettyBytes
app.mount('#app')
