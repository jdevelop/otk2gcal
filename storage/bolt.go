package storage

import "github.com/boltdb/bolt"

var (
	idsBucketName = []byte("ids")
	empty         = []byte("!")[:]
)

func NewBoltEventStorage(path string) (*boltEventsStorage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(idsBucketName)
		return err
	}); err != nil {
		return nil, err
	}
	return &boltEventsStorage{
		db: db,
	}, nil
}

type boltEventsStorage struct {
	db *bolt.DB
}

func (bes *boltEventsStorage) AddEvents(ids ...string) error {
	return bes.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(idsBucketName)
		for _, id := range ids {
			if err := b.Put([]byte(id), empty); err != nil {
				return err
			}
		}
		return nil
	})
}

func (bes *boltEventsStorage) IsExist(id string) (bool, error) {
	var exists = false
	bes.db.View(func(tx *bolt.Tx) error {
		exists = tx.Bucket(idsBucketName).Get([]byte(id)) != nil
		return nil
	})
	return exists, nil
}

func (bes *boltEventsStorage) Close() error {
	return bes.db.Close()
}

var _ EventsStorage = &boltEventsStorage{}
