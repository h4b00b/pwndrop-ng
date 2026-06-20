package storage

// DbApiToken is a long-lived, named authentication token for scripted access
// (e.g. uploading from curl/PowerShell). Unlike DbSession it does not expire;
// it is valid until explicitly revoked.
type DbApiToken struct {
	ID         int    `json:"id" storm:"id,increment"`
	Uid        int    `json:"uid" storm:"index"`
	Name       string `json:"name"`
	Token      string `json:"token" storm:"unique"`
	CreateTime int64  `json:"create_time"`
	LastUsed   int64  `json:"last_used"`
}

func ApiTokenCreate(o *DbApiToken) (*DbApiToken, error) {
	err := db.Save(o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func ApiTokenList() ([]DbApiToken, error) {
	var os []DbApiToken
	err := db.All(&os)
	if err != nil {
		return nil, err
	}
	return os, nil
}

func ApiTokenGetByToken(token string) (*DbApiToken, error) {
	var o DbApiToken
	err := db.One("Token", token, &o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func ApiTokenDelete(id int) error {
	o := &DbApiToken{
		ID: id,
	}
	err := db.DeleteStruct(o)
	if err != nil {
		return err
	}
	return nil
}

func ApiTokenTouch(id int, ts int64) error {
	return db.UpdateField(&DbApiToken{ID: id}, "LastUsed", ts)
}
