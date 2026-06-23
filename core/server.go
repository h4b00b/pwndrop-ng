package core

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme"

	"github.com/kgretzky/pwndrop/api"
	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
	"github.com/kgretzky/pwndrop/utils"
)

const (
	API_PATH = "api/v1"
)

// WebFS holds the admin panel filesystem. When set (the default, from the
// embedded front-end) it is served directly from the binary; if nil, the panel
// is served from disk at Cfg.GetAdminDir() as a fallback.
var WebFS fs.FS

type Server struct {
	srv       *http.Server
	listenTLS net.Listener
	listen    net.Listener
	wdav      *WebDav
	http      *Http
	cdb       *CertDb
	ns        *Nameserver
	r         *mux.Router
	blacklist map[string]*BlacklistItem
	bl_mtx    sync.Mutex
}

func NewServer(host string, port_plain int, port_tls int, enable_letsencrypt bool, enable_dns bool, ch_exit *chan bool) (*Server, error) {
	var err error
	s := &Server{
		blacklist: make(map[string]*BlacklistItem),
		bl_mtx:    sync.Mutex{},
	}

	hostname := fmt.Sprintf("%s:%d", host, port_plain)
	hostname_tls := fmt.Sprintf("%s:%d", host, port_tls)

	s.cdb, err = NewCertDb(Cfg.GetDataDir())
	if err != nil {
		return nil, err
	}

	cert, err := LoadTLSCertificate(filepath.Join(Cfg.GetDataDir(), "public.crt"), filepath.Join(Cfg.GetDataDir(), "private.key"))
	if err != nil {
		log.Warning("certificate: %s", err)
		cert, err = GenerateTLSCertificate(host)
		if err != nil {
			return nil, err
		}
		log.Info("generated self-signed certificate")
	} else {
		log.Info("using TLS certificate from data directory")
		enable_letsencrypt = false
	}

	tls_cfg := &tls.Config{}
	tls_cfg.Certificates = append(tls_cfg.Certificates, *cert)
	if enable_letsencrypt {
		log.Info("autocert: enabled")
		tls_cfg.GetCertificate = s.cdb.AutocertMgr.GetCertificate
		tls_cfg.NextProtos = []string{
			"h2", "http/1.1", // enable HTTP/2
			acme.ALPNProto, // enable tls-alpn ACME challenges
		}
	} else {
		log.Info("autocert: disabled")
	}

	s.wdav, err = NewWebDav(s)
	if err != nil {
		return nil, err
	}
	s.http, err = NewHttp(s)
	if err != nil {
		return nil, err
	}

	s.setupRouter()

	s.srv = &http.Server{
		Handler: http.Handler(s),
		Addr:    hostname,
		// WriteTimeout / ReadTimeout are intentionally 0: large uploads and
		// downloads on slow links need to be allowed to run for many minutes
		// (this was the explicit upstream choice — see CHANGELOG 1.0.2).
		// ReadHeaderTimeout, however, is cheap insurance against slow-loris
		// style attacks where a client dribbles request headers forever
		// (each connection pinning a goroutine) without ever committing to a
		// body upload. 15s is plenty for any real client and tight enough
		// that an attacker burns more of their own resources than ours.
		WriteTimeout:      0,
		ReadTimeout:       0,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       5 * time.Second,
		TLSConfig:         tls_cfg,
	}

	s.listenTLS, err = tls.Listen("tcp", hostname_tls, tls_cfg)
	if err != nil {
		return nil, err
	}
	s.listen, err = net.Listen("tcp", hostname)
	if err != nil {
		return nil, err
	}

	log.Info("starting HTTP/WebDAV server at %s", hostname)
	log.Info("starting HTTPS server at %s", hostname_tls)

	if enable_dns {
		s.ns, err = NewNameserver(ch_exit)
		if err != nil {
			return nil, err
		}
	}

	go func() {
		err := s.srv.Serve(s.listen)
		if err != nil {
			log.Fatal("failed to start HTTP/WebDAV server at %s", hostname)
			*ch_exit <- false
		}
	}()

	go func() {
		err := s.srv.Serve(s.listenTLS)
		if err != nil {
			log.Fatal("failed to start HTTPS server at %s", hostname_tls)
			*ch_exit <- false
		}
	}()

	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("%s %s", r.Method, r.URL.Path)

	cfg, _ := storage.ConfigGet(1)
	trustProxy := cfg != nil && cfg.TrustProxy
	from_ip := utils.ClientIP(r, trustProxy)

	if s.isBlacklisted(from_ip) {
		err := s.killConnection(w, -1)
		if err != nil {
			log.Error("http: %s (%s)", err, from_ip)
			w.Header().Set("Connection", "close")
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(500)
		}
		return
	}

	if !s.isWebDavRequest(r) {

		cookie_name := Cfg.GetCookieName()
		cookie_token := Cfg.GetCookieToken()

		if r.URL.Path == Cfg.GetSecretPath() {
			ck := &http.Cookie{
				Domain:   "",
				Path:     "/",
				Expires:  time.Now().AddDate(0, 3, 0),
				HttpOnly: true,
				Secure:   r.TLS != nil,
				SameSite: http.SameSiteLaxMode,
				Name:     cookie_name,
				Value:    cookie_token,
			}
			http.SetCookie(w, ck)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		if !s.FileExists(r.URL.Path) {
			// Scripted API clients presenting a valid auth token (header or
			// cookie) are let through without the secret-path cookie, so that
			// uploads/management work from curl/PowerShell with just a token.
			if _, err := api.AuthSession(r); err == nil {
				s.r.ServeHTTP(w, r)
				return
			}

			if ck, err := r.Cookie(cookie_name); err == nil {
				if ck.Value == cookie_token {
					s.r.ServeHTTP(w, r)
					return
				}
			}

			s.addBlacklistHit(from_ip)
			if len(Cfg.GetRedirectUrl()) > 0 {
				http.Redirect(w, r, Cfg.GetRedirectUrl(), http.StatusFound)
				return
			}
		}

		// http
		s.http.ServeHTTP(w, r)
	} else {
		// webdav — apply the same gate the HTTP path uses BEFORE delegating
		// to the dav handler, so scanners cannot bypass kill switch / filter
		// chain / per-file password / expire / quota by spoofing the WebDAV
		// UA. Counter increment + burn-after-read only fire for GET because
		// PROPFIND/HEAD don't actually fetch the blob.
		//
		// Method allowlist: this FS is read-only. PUT/DELETE/MKCOL/COPY/MOVE/
		// PROPPATCH/LOCK/UNLOCK would otherwise reach OpenFile/RemoveAll/etc.
		// and an unauthenticated client could truncate or attempt to mutate
		// hosted blobs just by sending a WebDAV-flavoured UA.
		switch r.Method {
		case "GET", "HEAD", "OPTIONS", "PROPFIND":
		default:
			s.killConnection(w, http.StatusMethodNotAllowed)
			return
		}
		f, action := s.runGate(w, r, from_ip, cfg, true)
		if action == gateBlocked {
			return
		}
		isGet := r.Method == "GET"
		if f != nil && isGet && (f.MaxDownloads > 0 || f.BurnAfterRead) {
			release := lockFile(f.ID)
			defer release()
			// countHits=false: the first runGate above already credited the
			// matching rule for this request.
			fresh, act2 := s.runGate(w, r, from_ip, cfg, false)
			if act2 == gateBlocked {
				return
			}
			if fresh == nil {
				logBlock(f, r, from_ip, "gone", f.NotifyMuted)
				s.killConnection(w, 404)
				return
			}
			f = fresh
		}
		// Wrap the response so we can tell whether the dav handler actually
		// delivered the body — OpenFile errors, scanner HEAD-then-cancel,
		// client TCP-RST mid-body and Range partials would otherwise still
		// consume the quota and trigger burn-after-read.
		var cw http.ResponseWriter = w
		var dcw *davCountWriter
		if f != nil && isGet {
			dcw = &davCountWriter{ResponseWriter: w}
			cw = dcw
			// Advertise the blob hash on GETs too so DAV clients can verify
			// what they fetched. Header is set before delegating so it lands
			// in the response before the dav handler writes the status line.
			if f.SHA256 != "" {
				if raw, err := hex.DecodeString(f.SHA256); err == nil && len(raw) == 32 {
					w.Header().Set("X-Content-SHA256", f.SHA256)
					w.Header().Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(raw))
				}
			}
		}
		s.wdav.Handler().ServeHTTP(cw, r)
		if f != nil && isGet {
			delivered := (dcw.status == 0 || dcw.status == 200) && dcw.written >= f.FileSize
			if !delivered {
				logBlock(f, r, from_ip, "aborted-webdav", f.NotifyMuted)
				return
			}
			if n, err := storage.FileIncrementDownloads(f.ID); err == nil {
				if f.MaxDownloads > 0 && n >= f.MaxDownloads {
					storage.FileEnable(f.ID, false)
				}
			}
			logBlock(f, r, from_ip, "ok-webdav", f.NotifyMuted)
			if f.BurnAfterRead {
				BurnFile(f.ID)
			}
		}
	}
}

// davCountWriter records the response status and bytes written by the dav
// handler so the WebDAV branch can gate increment / log / burn on a real
// delivery (status 200 and full body).
type davCountWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (w *davCountWriter) WriteHeader(s int) {
	if w.status == 0 {
		w.status = s
	}
	w.ResponseWriter.WriteHeader(s)
}

func (w *davCountWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (s *Server) isWebDavRequest(r *http.Request) bool {
	ua := r.Header.Get("user-agent")
	if strings.Index(ua, "WebDAV") >= 0 || strings.Index(ua, "DavClnt") >= 0 {
		return true
	}
	if r.Header.Get("translate") == "f" {
		return true
	}
	return false
}

func (s *Server) setupRouter() {
	admin_path := "/"
	s.r = mux.NewRouter()
	sr := s.r.PathPrefix(admin_path + API_PATH).Subrouter()
	sr.HandleFunc("/auth", api.AuthOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/auth", api.AuthCheckHandler).Methods("GET")
	sr.HandleFunc("/server_info", api.ServerInfoOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/server_info", api.ServerInfoGetHandler).Methods("GET")
	sr.HandleFunc("/version", api.VersionOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/version", api.VersionGetHandler).Methods("GET")
	sr.HandleFunc("/login", api.AuthOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/login", api.LoginUserHandler).Methods("POST")
	sr.HandleFunc("/logout", api.LogoutUserHandler).Methods("GET")
	sr.HandleFunc("/clear_secret", api.ClearSecretSessionHandler).Methods("GET")
	sr.HandleFunc("/create_account", api.AuthOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/create_account", api.CreateUserHandler).Methods("POST")
	sr.HandleFunc("/config", api.ConfigOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/config", api.ConfigGetHandler).Methods("GET")
	sr.HandleFunc("/config", api.ConfigUpdateHandler).Methods("POST")
	sr.HandleFunc("/tokens", api.TokenOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/tokens", api.TokenListHandler).Methods("GET")
	sr.HandleFunc("/tokens", api.TokenCreateHandler).Methods("POST")
	sr.HandleFunc("/tokens/{id}", api.TokenDeleteHandler).Methods("DELETE")
	sr.HandleFunc("/downloads", api.DownloadOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/downloads", api.DownloadListHandler).Methods("GET")
	sr.HandleFunc("/downloads", api.DownloadClearHandler).Methods("DELETE")
	sr.HandleFunc("/filters", api.FilterOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/filters", api.FilterListHandler).Methods("GET")
	sr.HandleFunc("/filters", api.FilterCreateHandler).Methods("POST")
	sr.HandleFunc("/filters/{id}", api.FilterUpdateHandler).Methods("PUT")
	sr.HandleFunc("/filters/{id}", api.FilterDeleteHandler).Methods("DELETE")
	sr.HandleFunc("/filters/test", api.RuleTestHandler).Methods("POST")
	sr.HandleFunc("/cleanup/run", api.CleanupRunHandler).Methods("POST")
	sr.HandleFunc("/files", api.FileOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/files", api.FileListHandler).Methods("GET")
	sr.HandleFunc("/files", api.FileCreateHandler).Methods("POST")
	sr.HandleFunc("/files/paste", api.FilePasteHandler).Methods("POST")
	// Chunked upload routes must be registered before /files/{id} so the
	// literal "chunked" segment isn't swallowed by the {id} matcher. More-
	// specific paths first: /init, /{id}/complete, /{id}/replace/{fid}, then
	// the catch-alls /{id} POST/DELETE for chunk append + abort.
	sr.HandleFunc("/files/chunked/init", api.ChunkedOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/files/chunked/init", api.ChunkedInitHandler).Methods("POST")
	sr.HandleFunc("/files/chunked/{id}/complete", api.ChunkedCompleteHandler).Methods("POST")
	sr.HandleFunc("/files/chunked/{id}/replace/{fid}", api.ChunkedReplaceCompleteHandler).Methods("POST")
	sr.HandleFunc("/files/chunked/{id}", api.ChunkedOptionsHandler).Methods("OPTIONS")
	sr.HandleFunc("/files/chunked/{id}", api.ChunkedAppendHandler).Methods("POST")
	sr.HandleFunc("/files/chunked/{id}", api.ChunkedAbortHandler).Methods("DELETE")
	sr.HandleFunc("/files/{id}", api.FileDeleteHandler).Methods("DELETE")
	sr.HandleFunc("/files/{id}", api.FileUpdateHandler).Methods("PUT")
	sr.HandleFunc("/files/{id}/sub", api.SubFileCreateHandler).Methods("POST")
	sr.HandleFunc("/files/{id}/sub/{sub_id}", api.SubFileDeleteHandler).Methods("DELETE")
	sr.HandleFunc("/files/{id}/enable", api.FileEnableHandler).Methods("POST")
	sr.HandleFunc("/files/{id}/disable", api.FileDisableHandler).Methods("POST")
	sr.HandleFunc("/files/{id}/pause", api.FilePauseHandler).Methods("POST")
	sr.HandleFunc("/files/{id}/unpause", api.FileUnpauseHandler).Methods("POST")
	sr.HandleFunc("/files/{id}/replace", api.FileReplaceHandler).Methods("POST")
	sr.HandleFunc("/files/{id}/rotate", api.FileRotateUrlHandler).Methods("POST")
	sr.HandleFunc("/files/bulk", api.FileBulkHandler).Methods("POST")
	var webHandler http.Handler
	if WebFS != nil {
		webHandler = http.FileServer(http.FS(WebFS))
	} else {
		webHandler = http.FileServer(http.Dir(Cfg.GetAdminDir()))
	}
	s.r.PathPrefix(admin_path).Handler(http.StripPrefix(admin_path, webHandler))
}

func (s *Server) GetFile(url string) (*storage.DbFile, int, error) {
	is_redirect := false
	f, err := storage.FileGetByUrl(url)
	if err != nil {
		f, err = storage.FileGetByRedirectUrl(url)
		if err != nil {
			return nil, 404, err
		}
		is_redirect = true
	}
	if !f.IsEnabled {
		return nil, 404, fmt.Errorf("file is disabled")
	}
	if f.IsPaused {
		if f.RedirectPath != "" && is_redirect {
			return nil, 404, fmt.Errorf("can't access facade via redirect while paused")
		} else if f.RefSubFile > 0 {
			sf, err := storage.SubFileGet(f.RefSubFile)
			if err != nil {
				return nil, 404, fmt.Errorf("facade file not found")
			}
			f.Filename = sf.Filename
			f.FileSize = sf.FileSize
		} else {
			return nil, 404, fmt.Errorf("facade file not set")
		}
	}
	return f, 200, nil
}

func (s *Server) FileExists(url string) bool {
	_, err := storage.FileGetByUrl(url)
	if err != nil {
		_, err = storage.FileGetByRedirectUrl(url)
		if err != nil {
			return false
		}
	}
	return true
}

func (s *Server) killConnection(w http.ResponseWriter, status int) error {
	if status > 0 {
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(status)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("connection hijacking not supported")
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (s *Server) isBlacklisted(ip_addr string) bool {
	s.bl_mtx.Lock()
	defer s.bl_mtx.Unlock()

	ret := false
	if bl, ok := s.blacklist[ip_addr]; ok {
		if bl.hits >= BLACKLIST_HITS_LIMIT {
			if time.Now().Before(bl.last_hit.Add(BLACKLIST_JAIL_TIME_SECS * time.Second)) {
				ret = true
			} else {
				delete(s.blacklist, ip_addr)
				return false
			}
		}
		bl.last_hit = time.Now()
	}
	return ret
}

func (s *Server) addBlacklistHit(ip_addr string) {
	s.bl_mtx.Lock()
	defer s.bl_mtx.Unlock()

	if bl, ok := s.blacklist[ip_addr]; ok {
		bl.hits += 1
	} else {
		bl := &BlacklistItem{
			hits:     1,
			last_hit: time.Now(),
		}
		s.blacklist[ip_addr] = bl
	}
}
