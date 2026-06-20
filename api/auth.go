package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
	"github.com/kgretzky/pwndrop/utils"
)

const AUTH_COOKIE_NAME = "t"
const AUTH_SESSION_TIMEOUT_SECS = 24 * 60 * 60

// Login throttling: N failed attempts per IP within window → block for cooldown.
// Tracked in-memory; resets on restart (acceptable: red-team deployments are
// short-lived). Successful login clears the counter for the IP.
const (
	loginMaxFails   = 5
	loginFailWindow = 5 * time.Minute
	loginBlockDur   = 15 * time.Minute
	// Cap the throttle map so an attacker cycling through many source IPs
	// (e.g. an IPv6 /64 or botnet) cannot pin unbounded memory. Eviction is
	// best-effort: expired entries first, then a small random batch.
	loginFailsMax       = 4096
	loginFailsEvictStep = 64
)

type loginFailEntry struct {
	count        int
	firstFail    time.Time
	blockedUntil time.Time
}

var (
	loginFailMu sync.Mutex
	loginFails  = map[string]*loginFailEntry{}
)

// dummyHash is consumed by CompareHashAndPassword when the username does not
// exist, so the response timing matches a real bcrypt check and the username
// cannot be enumerated via timing.
var dummyHash []byte

// bootstrapMu serialises the unauthenticated first-user creation path so that
// two concurrent CreateUserHandler calls cannot both observe an empty user
// list and both succeed. Storm's unique index on SearchName would catch the
// case where the names collide, but NOT the case where two different
// attackers each pick a different name and both end up as admin.
var bootstrapMu sync.Mutex

func init() {
	dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-pwndrop-auth-timing"), 10)
}

// loginClientIP returns the IP used for login throttling, honoring TrustProxy
// so deployments behind a reverse proxy don't bucket every login under the
// proxy IP and lock everyone out together.
func loginClientIP(r *http.Request) string {
	trustProxy := false
	if cfg, err := storage.ConfigGet(1); err == nil && cfg != nil {
		trustProxy = cfg.TrustProxy
	}
	return utils.ClientIP(r, trustProxy)
}

func loginIsBlocked(ip string) (bool, time.Duration) {
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	e, ok := loginFails[ip]
	if !ok {
		return false, 0
	}
	now := time.Now()
	if !e.blockedUntil.IsZero() && now.Before(e.blockedUntil) {
		return true, e.blockedUntil.Sub(now)
	}
	// window expired → garbage-collect the entry on the way out
	if !e.firstFail.IsZero() && now.Sub(e.firstFail) > loginFailWindow && e.blockedUntil.Before(now) {
		delete(loginFails, ip)
	}
	return false, 0
}

func loginRegisterFail(ip string) {
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	now := time.Now()
	e, ok := loginFails[ip]
	if !ok || now.Sub(e.firstFail) > loginFailWindow {
		if len(loginFails) >= loginFailsMax {
			loginFailsEvictLocked(now)
		}
		loginFails[ip] = &loginFailEntry{count: 1, firstFail: now}
		return
	}
	e.count++
	if e.count >= loginMaxFails {
		e.blockedUntil = now.Add(loginBlockDur)
	}
}

// loginFailsEvictLocked keeps loginFails bounded. Caller must hold loginFailMu.
// Drops expired non-blocked entries first; if still over the cap, drops up to
// loginFailsEvictStep more (map iteration order is randomised).
func loginFailsEvictLocked(now time.Time) {
	for ip, e := range loginFails {
		if e.blockedUntil.IsZero() && now.Sub(e.firstFail) > loginFailWindow {
			delete(loginFails, ip)
		} else if !e.blockedUntil.IsZero() && now.After(e.blockedUntil) {
			delete(loginFails, ip)
		}
	}
	if len(loginFails) < loginFailsMax {
		return
	}
	dropped := 0
	for ip, e := range loginFails {
		if !e.blockedUntil.IsZero() && now.Before(e.blockedUntil) {
			continue
		}
		delete(loginFails, ip)
		dropped++
		if dropped >= loginFailsEvictStep {
			return
		}
	}
}

func loginRegisterSuccess(ip string) {
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	delete(loginFails, ip)
}

func AuthOptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
}

func AuthCheckHandler(w http.ResponseWriter, r *http.Request) {
	type AuthResponse struct {
		Status int `json:"status"`
	}

	users, err := storage.UserList()
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	resp := &AuthResponse{}
	if len(users) == 0 {
		resp.Status = 0
		DumpResponse(w, "ok", http.StatusOK, 0, resp)
		return
	}

	_, err = AuthSession(r)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	resp.Status = 1
	DumpResponse(w, "ok", http.StatusOK, 0, resp)
}

func LoginUserHandler(w http.ResponseWriter, r *http.Request) {
	type LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	type LoginResponse struct {
		Username string `json:"username"`
		Token    string `json:"token"`
	}

	ip := loginClientIP(r)
	if blocked, retry := loginIsBlocked(ip); blocked {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retry.Seconds())+1))
		DumpResponse(w, "too many failed attempts", http.StatusTooManyRequests, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	j := LoginRequest{}
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		DumpResponse(w, "bad request", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	log.Debug("username: %s", j.Username)

	// Run bcrypt unconditionally — against the real hash if the user exists,
	// against a dummy hash otherwise — so a failed login does not leak whether
	// the username is known via response timing or via the error string.
	o, errUser := storage.UserGetByName(j.Username)
	hash := dummyHash
	if errUser == nil {
		hash = []byte(o.Password)
	}
	errCmp := bcrypt.CompareHashAndPassword(hash, []byte(j.Password))
	if errUser != nil || errCmp != nil {
		loginRegisterFail(ip)
		DumpResponse(w, "invalid credentials", http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}
	loginRegisterSuccess(ip)

	token := utils.GenRandomHash()
	s := &storage.DbSession{
		Uid:        o.ID,
		Token:      token,
		CreateTime: time.Now().Unix(),
	}

	_, err = storage.SessionCreate(s)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	resp := &LoginResponse{
		Username: o.Name,
		Token:    token,
	}

	ck := &http.Cookie{
		Domain:   "",
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Name:     AUTH_COOKIE_NAME,
		Value:    token,
	}
	http.SetCookie(w, ck)

	DumpResponse(w, "ok", http.StatusOK, 0, resp)
}

func LogoutUserHandler(w http.ResponseWriter, r *http.Request) {
	ck, err := r.Cookie(AUTH_COOKIE_NAME)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	token := ck.Value

	s, err := storage.SessionGetByToken(token)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	err = storage.SessionDelete(s.ID)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	deleteCookie(AUTH_COOKIE_NAME, w, r.TLS != nil)
	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}

func ClearSecretSessionHandler(w http.ResponseWriter, r *http.Request) {
	cookie_name := Cfg.GetCookieName()
	deleteCookie(cookie_name, w, r.TLS != nil)
	DumpResponse(w, "ok", http.StatusOK, 0, nil)
}

func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	type CreateUserRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	type CreateUserResponse struct {
		Username string `json:"username"`
	}

	// Hold the bootstrap mutex across the (list users → decide auth mode →
	// create user) sequence so two simultaneous requests can't both land in
	// the unauthenticated branch and end up creating two admin accounts.
	bootstrapMu.Lock()
	defer bootstrapMu.Unlock()

	users, err := storage.UserList()
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusInternalServerError, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	_, err = AuthSession(r)
	isBootstrap := len(users) == 0
	if !isBootstrap && err != nil {
		DumpResponse(w, err.Error(), http.StatusUnauthorized, API_ERROR_BAD_AUTHENTICATION, nil)
		return
	}

	limitBody(w, r, MaxJSONBody)
	j := CreateUserRequest{}
	err = json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	if j.Username == "" || j.Password == "" {
		DumpResponse(w, "bad request", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}
	// bcrypt truncates to 72 bytes anyway, but a multi-KB password reaching
	// GenerateFromPassword still costs unbounded CPU and bytes for no benefit.
	if len(j.Password) > 72 {
		DumpResponse(w, "password too long", http.StatusBadRequest, API_ERROR_BAD_REQUEST, nil)
		return
	}

	_, err = storage.UserGetByName(j.Username)
	if err == nil {
		DumpResponse(w, "user already exists", http.StatusOK, API_ERROR_USER_ALREADY_EXISTS, nil)
		return
	}

	phash, err := bcrypt.GenerateFromPassword([]byte(j.Password), 10)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	o := &storage.DbUser{
		Name:     j.Username,
		Password: string(phash),
	}

	_, err = storage.UserCreate(o)
	if err != nil {
		DumpResponse(w, err.Error(), http.StatusBadRequest, API_ERROR_FILE_DATABASE_FAILED, nil)
		return
	}

	// First-account bootstrap is the only path that lets an unauthenticated
	// caller create a user — surface it loudly with IP + UA so the operator
	// can spot a race where someone else grabbed it before them.
	if isBootstrap {
		log.Warning("bootstrap admin created: username=%q from ip=%s ua=%q",
			j.Username, loginClientIP(r), r.Header.Get("User-Agent"))
	}

	resp := &CreateUserResponse{
		Username: j.Username,
	}
	DumpResponse(w, "ok", http.StatusOK, 0, resp)
}

// extractToken pulls the auth token from, in order of preference: the
// "Authorization: Bearer <token>" header (also accepting a raw token), the
// "X-Pwndrop-Token" header, or the session cookie. This lets the same token be
// used both by the web panel (cookie) and by scripted clients (header).
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		}
		return strings.TrimSpace(h)
	}
	if h := r.Header.Get("X-Pwndrop-Token"); h != "" {
		return strings.TrimSpace(h)
	}
	if ck, err := r.Cookie(AUTH_COOKIE_NAME); err == nil {
		return ck.Value
	}
	return ""
}

// AuthSession looks up the (session or API) token for the request and returns
// the owning user id, or an error. An empty token is always rejected here
// (belt-and-braces — storm's unique index already won't match empty) before
// it ever reaches the DB layer.

func AuthSession(r *http.Request) (int, error) {
	token := extractToken(r)
	if token == "" {
		return -1, fmt.Errorf("no authentication token")
	}

	// persistent API tokens never expire and take precedence
	if t, err := storage.ApiTokenGetByToken(token); err == nil {
		storage.ApiTokenTouch(t.ID, time.Now().Unix())
		return t.Uid, nil
	}

	// fall back to short-lived login sessions
	s, err := storage.SessionGetByToken(token)
	if err != nil {
		return -1, err
	}

	if time.Now().After(time.Unix(s.CreateTime, 0).Add(AUTH_SESSION_TIMEOUT_SECS * time.Second)) {
		storage.SessionDelete(s.ID)
		return -1, fmt.Errorf("session token expired")
	}

	return s.Uid, nil
}

func deleteCookie(name string, w http.ResponseWriter, secure bool) {
	ck := &http.Cookie{
		Domain:   "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Name:     name,
		Value:    "",
	}
	http.SetCookie(w, ck)
}
