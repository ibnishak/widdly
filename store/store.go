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

// Package store contains common types.
package store

import (
	"context"
	"encoding/json"
	"strings"
	"errors"
)

var (
	// ErrNotFound is the error returned by the TiddlerStore when no tiddlers with a given key are found.
	ErrNotFound = errors.New("not found")

	ErrDBExist = errors.New("same backend exist")
	ErrDBNotExist = errors.New("backend not exist")

	backendlist = make(map[string]*TiddlerBackend)
)

type OpenFn (func (string) (TiddlerStore, error))

// Tiddler is a fundamental piece of content in TiddlyWeb.
type Tiddler struct {
	// Get
	Meta     []byte // Meta information (the tiddler serialized to JSON without or with text depned on system key or not)

	// Put
	Key      string // The title of the tiddler
	IsDraft  bool   // check Draft
	IsSys    bool   // check System Key

	// All
	Js map[string]interface{} // for proc
}

func NewTiddler(meta []byte, text []byte) (*Tiddler, error) {
	t := &Tiddler{}
	if text == nil {
		t.Meta = meta
		return t, nil
	}

	t.Js = make(map[string]interface{})
	err := json.Unmarshal(meta, &t.Js)
	if err != nil {
		return nil, err
	}
	t.Js["text"] = string(text)

	return t, nil
}

// MarshalJSON implements json.Marshaler
// If t is skinny (t.WithText is false), it returns t.Meta (not its copy).
func (t *Tiddler) MarshalJSON() ([]byte, error) {
	if t.Meta != nil {
		return t.Meta, nil
	}

	return json.Marshal(t.Js)
}

func (t *Tiddler) GetRevision() (rev int) {
	var meta struct{ Revision int }
	if json.Unmarshal(t.Meta, &meta) == nil {
		return meta.Revision
	}
	return 1
}


// TiddlerStore provides an interface for retrieving, storing and deleting tiddlers.
type TiddlerStore interface {
	// Get retrieves a tiddler from the store by key (title).
	// Get should return ErrNotFound error when no tiddlers with the given key are found.
	Get(ctx context.Context, key string) (*Tiddler, error)

	// All retrieves all the tiddlers from the store.
	// Most tiddlers should be returned skinny, except for special tiddlers,
	// like global macros (tiddlers tagged $:/tags/Macro), which should be
	// returned fat.
	// All must not return deleted tiddlers.
	All(ctx context.Context) ([]*Tiddler, error)

	// Put saves tiddler to the store and returns its revision.
	Put(ctx context.Context, tiddler Tiddler) (int, error)

	// Delete deletes a tiddler by key.
	Delete(ctx context.Context, key string) error

	// Safety close backend.
	Close() error

	// Max keeping history count
	// -1 => unlimit
	// 0 => disable
	SetMaxHistory(rev int)
}

type TiddlerBackend struct {
	Name string
	Open OpenFn
}

// MustOpen is a function variable assigned by the TiddlerStore implementations.
// MustOpen must return a working TiddlerStore given a data source.
func MustOpen(dataSource string) (TiddlerStore) {
	if len(backendlist) == 1 {
		for _, db := range backendlist {
			ss, err := db.Open(dataSource)
			if err != nil {
				panic(err)
			}
			return ss
		}
	}

	panic("has multi backends, please use Open() select one!")
}

func Open(name string, dataSource string) (TiddlerStore, error) {
	name = strings.ToLower(name)
	db, ok := backendlist[name]
	if !ok {
		return nil, ErrDBNotExist
	}
	return db.Open(dataSource)
}

func RegBackend(nameo string, fn OpenFn) (error) {
	if fn == nil {
		return ErrDBNotExist
	}
	name := strings.ToLower(nameo)
	_, ok := backendlist[name]
	if ok {
		return ErrDBExist
	}
	backendlist[name] = &TiddlerBackend{
		Name: nameo,
		Open: fn,
	}
	return nil
}

func ListBackend() ([]string) {
	list := make([]string, 0, len(backendlist))
	for _, db := range backendlist {
		list = append(list, db.Name)
	}
	return list
}

