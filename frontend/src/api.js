import axios from 'axios'

// The admin panel is served from the same origin as the API, so a relative
// base URL keeps the session cookie working in both dev (via proxy) and prod.
const api = axios.create({
  baseURL: '/api/v1',
})

export default api
