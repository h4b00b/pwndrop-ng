package storage

import "github.com/asdine/storm/q"

// DbFilter is one rule in the target-filter chain. Filters are evaluated per
// download request: the first rule that matches decides the action. Rules
// with FileId=0 are global; rules with FileId>0 apply only to that file (and
// are evaluated before global rules).
//
// MatchType:
//   - "ip":         Pattern is a single IPv4/IPv6 address (literal match).
//   - "cidr":       Pattern is a CIDR block (e.g. "10.0.0.0/8").
//   - "country":    Pattern is a 2-letter ISO country code (e.g. "IT", "RU").
//                   Resolved via the GeoIP cache in core/filter.go.
//   - "asn":        Pattern is an autonomous system number (e.g. "14618" or
//                   "AS14618"). Useful to nuke whole cloud providers in one
//                   rule: AS14618=AWS, AS8075=Azure, AS396982=GCP, AS16509=AWS,
//                   AS14061=DigitalOcean, AS63949=Linode. Resolved via the
//                   same GeoIP cache.
//   - "ua_regex":   Pattern is a Go regexp applied to the User-Agent header.
//
// Action:
//   - "allow":     serve the payload as usual.
//   - "deny":      404 (link looks broken).
//   - "facade":    serve the facade sub-file if one is attached, else 404.
//   - "redirect":  302 to the global RedirectUrl.
type DbFilter struct {
	ID         int    `json:"id" storm:"id,increment"`
	Enabled    bool   `json:"enabled"`
	FileId     int    `json:"file_id" storm:"index"`
	Priority   int    `json:"priority"`
	MatchType  string `json:"match_type"`
	Pattern    string `json:"pattern"`
	Action     string `json:"action"`
	Note       string `json:"note"`
	CreateTime int64  `json:"create_time"`

	// HitCount: how many download requests this rule has matched. Owned by
	// the evaluator (incremented via FilterIncrementHits), surfaced in the
	// admin UI so operators can tell which rules are actually doing work.
	HitCount int `json:"hit_count"`
}

// FilterIncrementHits bumps HitCount by 1 — best-effort, errors are not
// surfaced to the download path because logging a stat shouldn't block a
// serve. Same read-modify-write pattern as FileIncrementDownloads.
func FilterIncrementHits(id int) {
	r, err := FilterGet(id)
	if err != nil {
		return
	}
	_ = db.UpdateField(&DbFilter{ID: id}, "HitCount", r.HitCount+1)
}

func FilterCreate(o *DbFilter) (*DbFilter, error) {
	if err := db.Save(o); err != nil {
		return nil, err
	}
	return o, nil
}

func FilterUpdate(id int, o *DbFilter) (*DbFilter, error) {
	o.ID = id
	if err := db.Save(o); err != nil {
		return nil, err
	}
	return o, nil
}

func FilterDelete(id int) error {
	return db.DeleteStruct(&DbFilter{ID: id})
}

func FilterGet(id int) (*DbFilter, error) {
	var o DbFilter
	if err := db.One("ID", id, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

// FilterList returns rules sorted by Priority ascending. fileId<0 means all
// rules (admin UI). fileId==0 means only global. fileId>0 means only the
// rules tied to that specific file.
func FilterList(fileId int) ([]DbFilter, error) {
	var os []DbFilter
	query := db.Select()
	if fileId >= 0 {
		query = db.Select(q.Eq("FileId", fileId))
	}
	err := query.OrderBy("Priority").Find(&os)
	if err != nil {
		if err.Error() == "not found" {
			return []DbFilter{}, nil
		}
		return nil, err
	}
	return os, nil
}

// FilterListForEval returns the in-priority-order chain that should be
// evaluated for a single request on the given file: per-file rules first,
// then global rules. Disabled rules are skipped.
func FilterListForEval(fileId int) ([]DbFilter, error) {
	var perFile []DbFilter
	if fileId > 0 {
		ps, err := FilterList(fileId)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if p.Enabled {
				perFile = append(perFile, p)
			}
		}
	}
	globals, err := FilterList(0)
	if err != nil {
		return nil, err
	}
	for _, g := range globals {
		if g.Enabled {
			perFile = append(perFile, g)
		}
	}
	return perFile, nil
}

// FilterDeleteForFile removes any per-file rules tied to the given file id.
// Called from the file delete path so orphans don't pile up. Guards against
// fileId<=0 because FilterList(0) returns ALL global rules — a stray call with
// id=0 would wipe the entire global chain.
func FilterDeleteForFile(fileId int) error {
	if fileId <= 0 {
		return nil
	}
	rules, err := FilterList(fileId)
	if err != nil {
		return err
	}
	for _, r := range rules {
		_ = db.DeleteStruct(&r)
	}
	return nil
}
