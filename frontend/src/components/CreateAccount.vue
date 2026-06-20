<template>
  <div class="row align-items-center">
    <div class="col-sm-1 col-md-2 col-lg-3"></div>
    <div class="col-sm-10 col-md-8 col-lg-6 v-middle">
      <div class="login-panel">
        <form>
          <div class="form-group row">
            <label for="ca-username" class="col-sm-12 col-form-label">Username:</label>
            <div class="col-sm-12">
              <input
                type="text"
                class="form-control"
                id="ca-username"
                spellcheck="false"
                autocomplete="off"
                v-model="username"
                required
              >
            </div>
          </div>
          <div class="form-group row">
            <label for="ca-password" class="col-sm-12 col-form-label">Password:</label>
            <div class="col-sm-12">
              <input
                type="password"
                class="form-control"
                id="ca-password"
                spellcheck="false"
                v-model="password"
                required
              >
              <div v-show="password && password.length < 6" class="form-error">
                Password must be at least 6 characters
              </div>
            </div>
          </div>
          <div class="form-group row">
            <label for="ca-retype-password" class="col-sm-12 col-form-label">Retype Password:</label>
            <div class="col-sm-12">
              <input
                type="password"
                class="form-control"
                id="ca-retype-password"
                spellcheck="false"
                v-model="retypePassword"
                required
              >
              <div v-show="retypePassword && password !== retypePassword" class="form-error">
                Passwords do not match
              </div>
            </div>
          </div>
          <hr>
          <div class="form-group row">
            <div class="col-sm-12">
              <button
                class="btn btn-primary btn-lg btn-login"
                type="submit"
                :disabled="!isComplete"
                @click.prevent="createAccount()"
              >Create Account</button>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api'

export default {
  name: 'CreateAccount',
  data() {
    return {
      username: '',
      password: '',
      retypePassword: '',
    }
  },
  computed: {
    isComplete() {
      return !!(
        this.username &&
        this.password &&
        this.password.length >= 6 &&
        this.password === this.retypePassword
      )
    },
  },
  methods: {
    createAccount() {
      if (!this.isComplete) {
        return
      }
      api
        .post(
          '/create_account',
          { username: this.username, password: this.password },
          { headers: { 'content-type': 'application/json' } }
        )
        .then(() => {
          this.$router.push('/login').catch(() => {})
        })
        .catch((error) => {
          console.log(error)
        })
    },
  },
}
</script>
