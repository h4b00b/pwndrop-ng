import { createRouter, createWebHashHistory } from 'vue-router'
import FileView from './components/FileView.vue'
import Login from './components/Login.vue'
import CreateAccount from './components/CreateAccount.vue'

// Hash history mirrors the original (Vue Router 3 default) and avoids the need
// for server-side SPA fallback routing.
export default createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', component: FileView },
    { path: '/login', component: Login },
    { path: '/create_account', component: CreateAccount },
  ],
})
