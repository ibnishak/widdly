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

// Package sqlite is a SQLite3 TiddlerStore backend.
package sqlite

import (
	"bytes"
	"context"
	"encoding/json"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"

	"../../store"
)

const (
	TypeName = "sqlite"
)

// sqliteStore is a sqliteDB store for tiddlers.
type sqliteStore struct {
	db *sql.DB
	maxRev int
}

func init() {
	err := store.RegBackend(TypeName, Open)
	if err != nil {
		panic("multi backends with same type at the same time!")
	}
}

// Open opens the SQLite3 file specified as dataSource,
// creates the necessary tables and returns a TiddlerStore.
func Open(dataSource string) (store.TiddlerStore, error) {
	// open url: _journal_mode
	// SQL: PRAGMA journal_mode = DELETE | TRUNCATE | MEMORY | WAL
	// reduce disk IO for sdcard/NAND(rpi, router etc) : MEMORY > WAL, TRUNCATE, DELETE
	// more disk IO for safety : WAL > TRUNCATE
	// https://www.sqlite.org/pragma.html#pragma_journal_mode

	// open url: _synchronous
	// SQL: PRAGMA synchronous = 0 | OFF | 1 | NORMAL | 2 | FULL | 3 | EXTRA;
	// with UPS: OFF, 0
	// more safety but without UPS: EXTRA, 3 > FULL, 2
	// https://www.sqlite.org/pragma.html#pragma_synchronous
	db, err := sql.Open("sqlite3", dataSource + "?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	initStmt := `
		CREATE TABLE IF NOT EXISTS tiddler (id integer not null primary key AUTOINCREMENT, title text NOT NULL UNIQUE, meta text, content BLOB, revision integer);
		CREATE TABLE IF NOT EXISTS tiddler_history (id integer not null primary key AUTOINCREMENT, title text NOT NULL, meta text, content BLOB, revision integer);
	`
	_, err = db.Exec(initStmt)
	if err != nil {
		return nil, err
	}
	return &sqliteStore{db, -1}, nil
}

func (s *sqliteStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Get retrieves a tiddler from the store by key (title).
func (s *sqliteStore) Get(_ context.Context, key string) (*store.Tiddler, error) {
	getStmt, err := s.db.Prepare(`SELECT meta, content FROM tiddler WHERE title = ?`)
	var meta string
	var content string
	err = getStmt.QueryRow(key).Scan(&meta, &content)
	if err != nil {
		return nil, err
	}
	return store.NewTiddler([]byte(meta), []byte(content))
}

func copyOf(p []byte) []byte {
	q := make([]byte, len(p), len(p))
	copy(q, p)
	return q
}

// All retrieves all the tiddlers (mostly skinny) from the store.
// Special tiddlers (like global macros) are returned fat.
func (s *sqliteStore) All(_ context.Context) ([]*store.Tiddler, error) {
	tiddlers := make([]*store.Tiddler, 0)
	rows, err := s.db.Query(`SELECT meta, content FROM tiddler`)
	defer rows.Close()
	for rows.Next() {
		var meta string
		var content string
		if err := rows.Scan(&meta, &content); err != nil {
		        return nil, err
		}

		var tiddler []byte
		metabuf := []byte(meta)
		if bytes.Contains(metabuf, []byte(`"$:/tags/Macro"`)) {
			tiddler = []byte(content)
		}

		t, _ := store.NewTiddler(metabuf, tiddler)
		tiddlers = append(tiddlers, t)
	}
	if err != nil {
		return nil, err
	}
	return tiddlers, nil
}

func getLastRevision(db *sql.DB, mkey string) int {
	var revision int
	getStmt, err := db.Prepare(`SELECT revision FROM tiddler WHERE title = ?`)
	err = getStmt.QueryRow(mkey).Scan(&revision)
	if err != nil {
		return 1
	}
	return revision
}

// delete all revision <= rev
func (s *sqliteStore) trimRevision(key string, rev int) (err error) {
	deleteStmt, err := s.db.Prepare(`DELETE FROM tiddler_history WHERE title = ? AND revision <= ?`)
	if err != nil {
		return err
	}
	_, err = deleteStmt.Exec(key, rev)
	if err != nil {
		return err
	}
	return
}

// Put saves tiddler to the store, incrementing and returning revision.
// The tiddler is also written to the tiddler_history bucket.
func (s *sqliteStore) Put(ctx context.Context, tiddler store.Tiddler) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 1, err
	}
	defer tx.Rollback()

	rev := getLastRevision(s.db, tiddler.Key) + 1
	insertStmt, err := s.db.Prepare(`INSERT INTO tiddler(title, meta, content, revision) VALUES (?, ?, ?, ?) ON CONFLICT(title) DO UPDATE SET meta = ?, content = ?, revision = ?`)
	if err != nil {
		return 0, err
	}

	tiddler.Js["revision"] = rev
	text, _ := tiddler.Js["text"].(string)
	delete(tiddler.Js, "text")
	meta, err := json.Marshal(tiddler.Js)
	if err != nil {
		return 0, err
	}

	_, err = insertStmt.Exec(tiddler.Key, meta, text, rev, meta, text, rev)
	if err != nil {
		return 0, err
	}

	// skip Draft & system key history
	if s.maxRev != 0 && !tiddler.IsDraft && !tiddler.IsSys {
		// remove old history
		if s.maxRev > 0 && rev - s.maxRev > 1 {
			s.trimRevision(tiddler.Key, rev - 1 - s.maxRev)
		}

		insertStmt, err := s.db.Prepare(`INSERT INTO tiddler_history(title, meta, content, revision) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return 0, err
		}
		_, err = insertStmt.Exec(tiddler.Key, meta, text, rev)
		if err != nil {
			return 0, err
		}
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return rev, err
	}
	return rev, nil
}

// Delete deletes a tiddler with the given key (title) from the store.
func (s *sqliteStore) Delete(ctx context.Context, key string) error {
	deleteStmt, err := s.db.Prepare(`DELETE FROM tiddler WHERE title = ?`)
	if err != nil {
		return err
	}
	_, err = deleteStmt.Exec(key)
	if err != nil {
		return err
	}

	// skip if no history keeping
	if s.maxRev == 0 {
		return nil
	}

	deleteStmt, err = s.db.Prepare(`DELETE FROM tiddler_history WHERE title = ?`)
	if err != nil {
		return err
	}
	_, err = deleteStmt.Exec(key)
	if err != nil {
		return err
	}
	return nil
}

func (s *sqliteStore) SetMaxHistory(rev int) {
	s.maxRev = rev
}

