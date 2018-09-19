// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General
// Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package bolt is a BoltDB TiddlerStore backend.
package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	bolt "go.etcd.io/bbolt"

	"../../store"
)

const (
	TypeName = "bbolt"
)

// boltStore is a BoltDB store for tiddlers.
type boltStore struct {
	db *bolt.DB
	maxRev int
}

func init() {
	err := store.RegBackend(TypeName, Open)
	if err != nil {
		panic("multi backends with same type at the same time!")
	}
}

func copyOf(p []byte) []byte {
	q := make([]byte, len(p), len(p))
	copy(q, p)
	return q
}

// Open opens the BoltDB file specified as dataSource,
// creates the necessary buckets and returns a TiddlerStore.
func Open(dataSource string) (store.TiddlerStore, error) {
	db, err := bolt.Open(dataSource, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("tiddler"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("tiddler_history"))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &boltStore{db, -1}, nil
}

func (s *boltStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Get retrieves a tiddler from the store by key (title).
func (s *boltStore) Get(_ context.Context, key string) (*store.Tiddler, error) {
	var meta []byte
	var tiddler []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("tiddler"))
		meta = b.Get([]byte(key + "|1"))
		if meta == nil {
			return store.ErrNotFound
		}
		meta = copyOf(meta)
		tiddler = b.Get([]byte(key + "|2"))
		if tiddler != nil {
			tiddler = copyOf(tiddler)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return store.NewTiddler(meta, tiddler)
}


// All retrieves all the tiddlers (mostly skinny) from the store.
// Special tiddlers (like global macros) are returned fat.
func (s *boltStore) All(_ context.Context) ([]*store.Tiddler, error) {
	tiddlers := make([]*store.Tiddler, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("tiddler"))
		c := b.Cursor()
		for k, meta := c.First(); k != nil; k, meta = c.Next() {
			if len(meta) == 0 {
				c.Next()
				continue
			}

			var tiddler []byte
			_, text := c.Next()
			if bytes.Contains(meta, []byte(`"$:/tags/Macro"`)) {
				tiddler = copyOf(text)
			}

			t, _ := store.NewTiddler(copyOf(meta), tiddler)
			tiddlers = append(tiddlers, t)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tiddlers, nil
}

func getLastRevision(b *bolt.Bucket, mkey []byte) int {
	var meta struct{ Revision int }
	data := b.Get(mkey)
	if data != nil && json.Unmarshal(data, &meta) == nil {
		return meta.Revision
	}
	return 1
}

// delete all revision <= rev
func (s *boltStore) trimRevision(b *bolt.Bucket, key string, rev int) (err error) {
	c := b.Cursor()
	prefix := []byte(fmt.Sprintf("%s#", key))
	for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		idx := bytes.LastIndexByte(k, byte('#'))
		if idx < 0 {
			return
		}

		krev64, _ := strconv.ParseInt(string(k[idx+1:]), 10, 64)
		krev := int(krev64)
		if krev <= rev {
			err := b.Delete(k)
			if err != nil {
				return err
			}
		}
	}
	return
}

// Put saves tiddler to the store, incrementing and returning revision.
// The tiddler is also written to the tiddler_history bucket.
func (s *boltStore) Put(ctx context.Context, tiddler store.Tiddler) (int, error) {
	var rev int
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("tiddler"))
		mkey := []byte(tiddler.Key + "|1")

		rev = getLastRevision(b, mkey) + 1
		tiddler.Js["revision"] = rev

		var data []byte
		var err error
		if s.maxRev != 0 && !tiddler.IsDraft && !tiddler.IsSys { // skip Draft & system key history
			data, err = tiddler.MarshalJSON() // meta with text & rev
			if err != nil {
				return err
			}
		}

		text, _ := tiddler.Js["text"].(string)
		delete(tiddler.Js, "text")
		meta, err := json.Marshal(tiddler.Js)
		if err != nil {
			return err
		}

		err = b.Put(mkey, meta)
		if err != nil {
			return err
		}
		err = b.Put([]byte(tiddler.Key+"|2"), []byte(text))
		if err != nil {
			return err
		}

		// skip Draft & system key history
		if s.maxRev != 0 && !tiddler.IsDraft && !tiddler.IsSys {
			history := tx.Bucket([]byte("tiddler_history"))

			// remove old history
			if s.maxRev > 0 && rev - s.maxRev > 1 {
				s.trimRevision(history, tiddler.Key, rev - 1 - s.maxRev)
			}

			err = history.Put([]byte(fmt.Sprintf("%s#%d", tiddler.Key, rev)), data)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}
	return rev, nil
}

// Delete deletes a tiddler with the given key (title) from the store.
func (s *boltStore) Delete(ctx context.Context, key string) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("tiddler"))
		mkey := []byte(key + "|1")

		rev := getLastRevision(b, mkey)

		err := b.Delete(mkey)
		if err != nil {
			return err
		}
		err = b.Delete([]byte(key+"|2"))
		if err != nil {
			return err
		}

		// remove all history
		history := tx.Bucket([]byte("tiddler_history"))
		err = s.trimRevision(history, key, rev)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *boltStore) SetMaxHistory(rev int) {
	s.maxRev = rev
}

