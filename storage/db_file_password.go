package storage

// DbFilePassword holds the bcrypt password hash for a DbFile. Kept in its own
// bucket so that the DbFile struct can be serialized to API responses without
// any leak-filtering — there is no hash field on DbFile to leak.
//
// Lifecycle is owned by the file: when a file is deleted, FilePasswordDelete
// is called from the file delete path.
type DbFilePassword struct {
	ID   int    `json:"id" storm:"id"`
	Hash string `json:"hash"`
}

func FilePasswordGet(fileId int) (*DbFilePassword, error) {
	var o DbFilePassword
	if err := db.One("ID", fileId, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func FilePasswordDelete(fileId int) error {
	o := &DbFilePassword{ID: fileId}
	err := db.DeleteStruct(o)
	if err != nil && err.Error() == "not found" {
		return nil
	}
	return err
}
