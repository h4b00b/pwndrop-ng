<template>
  <div class="row align-items-center">
    <div class="col-sm-1 col-md-2 col-lg-3"></div>
    <div class="col-sm-10 col-md-8 col-lg-6 v-middle">
      <div class="login-panel">
        <form>
          <div class="form-group row">
            <label for="login-username" class="col-sm-12 col-form-label">Username:</label>
            <div class="col-sm-12">
              <input
                type="text"
                class="form-control"
                id="login-username"
                spellcheck="false"
                autocomplete="off"
                v-model="username"
                required
              >
            </div>
          </div>
          <div class="form-group row">
            <label for="login-password" class="col-sm-12 col-form-label">Password:</label>
            <div class="col-sm-12">
              <input
                type="password"
                class="form-control"
                id="login-password"
                spellcheck="false"
                v-model="password"
                required
              >
            </div>
          </div>
          <div v-show="status !== ''" class="form-group row">
            <div class="col-sm-12">
              <div class="login-status">{{ status }}</div>
            </div>
          </div>
          <hr>
          <div class="form-group row">
            <div class="col-sm-12">
              <button
                class="btn btn-primary btn-lg btn-login"
                type="submit"
                :disabled="!isComplete"
                @click.prevent="login()"
              >Login</button>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api'
import { session } from '../state'

export default {
  name: 'Login',
  data() {
    return {
      username: '',
      password: '',
      status: '',
    }
  },
  computed: {
    isComplete() {
      return !!(this.username && this.password)
    },
  },
  methods: {
    login() {
      if (!this.username || !this.password) {
        return
      }
      api
        .post(
          '/login',
          { username: this.username, password: this.password },
          { headers: { 'content-type': 'application/json' } }
        )
        .then((response) => {
          session.isLoggedIn = true
          session.username = response.data.data.username
          this.$router.push('/').catch(() => {})
        })
        .catch((error) => {
          if (error.response && error.response.status === 401) {
            this.status = 'Incorrect username or password'
          } else {
            this.status = 'Internal server error'
          }
        })
    },
  },
}
</script>
