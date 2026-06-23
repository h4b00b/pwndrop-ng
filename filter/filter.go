// Package filter contains the target-filter evaluator and the GeoIP cache.
// It lives in its own package so both core (the download serve path) and api
// (the rule-tester endpoint) can call into it without an import cycle.
package filter

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kgretzky/pwndrop/log"
	"github.com/kgretzky/pwndrop/storage"
)

// Action is what a matched rule (or the default) tells the request path to do
// with the download.
type Action string

const (
	ActionAllow    Action = "allow"
	ActionDeny     Action = "deny"
	ActionFacade   Action = "facade"
	ActionRedirect Action = "redirect"
)

// Decision is what the evaluator returns. RuleId is 0 when the decision came
// from FiltersDefaultAction (no rule matched).
type Decision struct {
	Action  Action
	RuleId  int
	Pattern string
	Type    string
}

// LogTag is the short identifier the HTTP layer puts into the download log
// status ("filter-deny:cidr:10.0.0.0/8"). For the default fallback we emit
// "filter-default" so the operator can tell rule from default in the log.
func (d Decision) LogTag() string {
	if d.RuleId == 0 {
		return fmt.Sprintf("filter-default:%s", d.Action)
	}
	return fmt.Sprintf("filter-%s:%s:%s", d.Action, d.Type, d.Pattern)
}

// Evaluate walks the per-file + global rule chain for this request and
// returns the first matching action, or the configured default. The second
// return value is true when filters are enabled — when false the action is
// ActionAllow and the caller can skip any filter-related bookkeeping.
func Evaluate(r *http.Request, fileId int, fromIp string) (Decision, bool) {
	cfg, err := storage.ConfigGet(1)
	if err != nil || !cfg.FiltersEnabled {
		return Decision{Action: ActionAllow}, false
	}

	rules, err := storage.FilterListForEval(fileId)
	if err != nil {
		log.Error("filter: listing rules: %s", err)
		return Decision{Action: ActionAllow}, false
	}

	ua := r.Header.Get("User-Agent")
	for _, rule := range rules {
		if matchRule(rule, fromIp, ua) {
			return Decision{
				Action:  normalizeAction(rule.Action),
				RuleId:  rule.ID,
				Pattern: rule.Pattern,
				Type:    rule.MatchType,
			}, true
		}
	}

	def := normalizeAction(cfg.FiltersDefaultAction)
	return Decision{Action: def}, true
}

func normalizeAction(a string) Action {
	switch Action(a) {
	case ActionDeny, ActionFacade, ActionRedirect:
		return Action(a)
	default:
		return ActionAllow
	}
}

func matchRule(rule storage.DbFilter, fromIp, ua string) bool {
	switch rule.MatchType {
	case "ip":
		return strings.EqualFold(strings.TrimSpace(rule.Pattern), fromIp)
	case "cidr":
		_, n, err := net.ParseCIDR(strings.TrimSpace(rule.Pattern))
		if err != nil {
			return false
		}
		ip := net.ParseIP(fromIp)
		return ip != nil && n.Contains(ip)
	case "country":
		want := strings.ToUpper(strings.TrimSpace(rule.Pattern))
		got := geoLookup(fromIp).country
		return want != "" && got != "" && want == got
	case "asn":
		want := normalizeASN(rule.Pattern)
		got := geoLookup(fromIp).asn
		return want != "" && got != "" && want == got
	case "ua_regex":
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return false
		}
		return re.MatchString(ua)
	}
	return false
}

// ---------------------------------------------------------------------------
// GeoIP lookup
//
// Provider is configurable via DbConfig.GeoIpEndpoint (must contain "%s" for
// the IP). The default is ipwho.is (HTTPS, free, no key). The legacy
// ip-api.com endpoint is intentionally NOT used over HTTP — that leaked every
// visitor IP in plaintext to a third party and let an on-path attacker forge
// the country code.
//
// Results are cached: 1h on success, 5min on error (so a flaky upstream does
// not get hammered). The cache is capped at geoCacheMax entries — when full,
// expired entries are purged first and, if still full, a small random batch is
// evicted (map iteration order is randomised in Go).
// ---------------------------------------------------------------------------

const (
	geoCacheTTLOk     = 1 * time.Hour
	geoCacheTTLBad    = 5 * time.Minute
	geoTimeout        = 1500 * time.Millisecond
	geoDefaultURL     = "https://ipwho.is/%s"
	geoCacheMax       = 10000
	geoCacheEvictStep = 256
)

type geoEntry struct {
	country string
	asn     string // bare digits, no "AS" prefix, "" when unknown
	expires time.Time
}

// normalizeASN turns "AS14618", " as14618 ", "14618" into "14618". Empty
// string when input is not a recognisable ASN. Used both to canonicalise
// rule patterns and to compare against the cached ASN.
func normalizeASN(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) >= 2 && (s[0] == 'A' || s[0] == 'a') && (s[1] == 'S' || s[1] == 's') {
		s = s[2:]
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s
}

var (
	geoMu     sync.RWMutex
	geoCache  = map[string]geoEntry{}
	geoClient = &http.Client{Timeout: geoTimeout}
)

func geoEndpoint() string {
	if cfg, err := storage.ConfigGet(1); err == nil && cfg != nil && strings.Contains(cfg.GeoIpEndpoint, "%s") {
		return cfg.GeoIpEndpoint
	}
	return geoDefaultURL
}

func geoLookup(ip string) geoEntry {
	if ip == "" {
		return geoEntry{}
	}
	// Private/loopback never resolves remotely — return empty so a "country"
	// or "asn" rule does not match and the next rule (or default) takes over.
	if isPrivateIP(ip) {
		return geoEntry{}
	}

	geoMu.RLock()
	if e, ok := geoCache[ip]; ok && time.Now().Before(e.expires) {
		geoMu.RUnlock()
		return e
	}
	geoMu.RUnlock()

	cc, asn, err := fetchGeo(ip)
	geoMu.Lock()
	geoEvictIfFullLocked()
	exp := time.Now().Add(geoCacheTTLOk)
	if err != nil {
		exp = time.Now().Add(geoCacheTTLBad)
		cc, asn = "", ""
	}
	entry := geoEntry{country: cc, asn: asn, expires: exp}
	geoCache[ip] = entry
	geoMu.Unlock()
	return entry
}

// geoEvictIfFullLocked keeps geoCache bounded. Caller must hold geoMu (write).
// Strategy: drop all expired entries; if still over the cap, drop up to
// geoCacheEvictStep more (map iteration order is randomised, so this is a
// cheap pseudo-random eviction).
func geoEvictIfFullLocked() {
	if len(geoCache) < geoCacheMax {
		return
	}
	now := time.Now()
	for ip, e := range geoCache {
		if now.After(e.expires) {
			delete(geoCache, ip)
		}
	}
	if len(geoCache) < geoCacheMax {
		return
	}
	dropped := 0
	for ip := range geoCache {
		delete(geoCache, ip)
		dropped++
		if dropped >= geoCacheEvictStep {
			break
		}
	}
}

func fetchGeo(ip string) (string, string, error) {
	resp, err := geoClient.Get(fmt.Sprintf(geoEndpoint(), ip))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("status %d", resp.StatusCode)
	}
	// Decode into a union of shapes so the same code handles ipwho.is,
	// ipapi.co, ipwhois.app (snake_case + nested connection.asn) and legacy
	// ip-api.com (camelCase + bare "as" string like "AS14618 Amazon"). All
	// asn extraction paths funnel through normalizeASN so the cache stores
	// bare digits.
	var body struct {
		CountryCode      string      `json:"country_code"`
		CountryCodeCamel string      `json:"countryCode"`
		Asn              interface{} `json:"asn"`
		AsField          string      `json:"as"`
		Connection       struct {
			Asn interface{} `json:"asn"`
		} `json:"connection"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", err
	}
	cc := body.CountryCode
	if cc == "" {
		cc = body.CountryCodeCamel
	}
	asn := extractASN(body.Connection.Asn)
	if asn == "" {
		asn = extractASN(body.Asn)
	}
	if asn == "" && body.AsField != "" {
		// "AS14618 Amazon-AES" — keep only the leading token.
		first := strings.Fields(body.AsField)
		if len(first) > 0 {
			asn = normalizeASN(first[0])
		}
	}
	return strings.ToUpper(cc), asn, nil
}

// extractASN copes with providers that return ASN as either a number or a
// string. Numbers come through json.Decode as float64; strings may be raw
// digits or prefixed with "AS".
func extractASN(v interface{}) string {
	switch t := v.(type) {
	case float64:
		if t <= 0 {
			return ""
		}
		return fmt.Sprintf("%d", int64(t))
	case string:
		return normalizeASN(t)
	}
	return ""
}

func isPrivateIP(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return true
	}
	return false
}
