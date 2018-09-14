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

// Package flatFile is a file base TiddlerStore backend.
package flatFile

import (
	"bytes"
	"context"
	"strings"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"io/ioutil"
	"regexp"

	"../../store"
)

// flatFileStore is a file base store for tiddlers.
type flatFileStore struct {
	storePath string
	tiddlersPath string
	tiddlerHistoryPath string
}

func init() {
	if store.MustOpen != nil {
		panic("attempt to use two different backends at the same time!")
	}
	store.MustOpen = MustOpen
}

func exists(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil { return true, nil }
    if os.IsNotExist(err) { return false, nil }
    return true, err
}

func checkExt(pathS string, ext string) []string {
	var files []string
	filepath.Walk(pathS, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			r, err := regexp.MatchString(ext, f.Name())
			if err == nil && r {
				files = append(files, f.Name())
			}
		}
		return nil
	})
	return files
}

// MustOpen opens the BoltDB file specified as dataSource,
// creates the necessary buckets and returns a TiddlerStore.
// MustOpen panics if there is an error.
func MustOpen(dataSource string) store.TiddlerStore {
	storePath := filepath.Join(".", dataSource)
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
	    os.Mkdir(storePath, os.ModePerm)
	}

	tiddlersPath := filepath.Join(storePath, "tiddlers")
	if _, err := os.Stat(tiddlersPath); os.IsNotExist(err) {
	    os.Mkdir(tiddlersPath, os.ModePerm)
	}

	tiddlerHistoryPath := filepath.Join(storePath, "tiddlerHistory")
	if _, err := os.Stat(tiddlerHistoryPath); os.IsNotExist(err) {
	    os.Mkdir(tiddlerHistoryPath, os.ModePerm)
	}
	return &flatFileStore{storePath, tiddlersPath, tiddlerHistoryPath}
}

func key2File(key string) string {
	illegalChar := `<>:"/\|?*^`
	mapFn := func(r rune) rune {
		if strings.ContainsRune(illegalChar, r) {
			return '_'
		} else {
			return r
		}
	}
	return strings.Map(mapFn, key)
}

// Get retrieves a tiddler from the store by key (title).
func (s *flatFileStore) Get(_ context.Context, key string) (*store.Tiddler, error) {
	isSys := strings.HasPrefix(key, "$:/")
	key = key2File(key)
	tiddlerPath := filepath.Join(s.tiddlersPath, key + ".tid")
	tiddlerMetaPath := filepath.Join(s.tiddlersPath, key + ".meta")
	if _, err := os.Stat(tiddlerMetaPath); os.IsNotExist(err) {
		return nil, store.ErrNotFound
	}

	meta, err := ioutil.ReadFile(tiddlerMetaPath)
	if err != nil {
		return nil, err
	}

	var tiddler []byte
	if !isSys {
		tiddler, err = ioutil.ReadFile(tiddlerPath)
		if err != nil {
			return nil, err
		}
	}

	return store.NewTiddler(meta, tiddler)
}

// All retrieves all the tiddlers (mostly skinny) from the store.
// Special tiddlers (like global macros) are returned fat.
func (s *flatFileStore) All(_ context.Context) ([]*store.Tiddler, error) {
	tiddlers := make([]*store.Tiddler, 0)
	files := checkExt(s.tiddlersPath, ".meta")
	for _, file := range files {
		var tiddler []byte
		meta, _ := ioutil.ReadFile(filepath.Join(s.tiddlersPath, file))
		if bytes.Contains(meta, []byte(`"$:/tags/Macro"`)) {
			var extension = filepath.Ext(file)
			var tiddlerPath = file[0:len(file)-len(extension)]
			tiddler, _ = ioutil.ReadFile(tiddlerPath + ".tid")
		}
		t, _ := store.NewTiddler(meta, tiddler)
		tiddlers = append(tiddlers, t)
	}
	return tiddlers, nil
}

func getLastRevision(s *flatFileStore, key string) int {
	key = key2File(key)
	rev := 1 // start with 1

	tiddlerMetaPath := filepath.Join(s.tiddlersPath, key + ".meta")
	if _, err := os.Stat(tiddlerMetaPath); os.IsNotExist(err) {
		return rev
	}else {
		meta, err := ioutil.ReadFile(tiddlerMetaPath)
		if err != nil {
			return rev
		}

		t, _ := store.NewTiddler(meta, nil)
		rev = t.GetRevision()
	}

	return rev
}

// Put saves tiddler to the store, incrementing and returning revision.
// The tiddler is also written to the tiddler_history bucket.
func (s *flatFileStore) Put(ctx context.Context, tiddler store.Tiddler) (int, error) {
	var err error
	key := key2File(tiddler.Key)

	rev := getLastRevision(s, key) + 1
	tiddler.Js["revision"] = rev

	// skip system history, only save meta & data to single file
	if tiddler.IsSys {
		meta, err := tiddler.MarshalJSON() // meta with text & rev
		if err != nil {
			return 0, err
		}

		err = ioutil.WriteFile(filepath.Join(s.tiddlersPath, key + ".meta"), meta, 0644)
		if err != nil {
			return 0, err
		}
		return rev, nil
	}

	// skip Draft history
	if !tiddler.IsDraft {
		data, err := tiddler.MarshalJSON()
		err = ioutil.WriteFile(filepath.Join(s.tiddlerHistoryPath, fmt.Sprintf("%s#%d", key, rev)), data, 0644)
		if err != nil {
			return rev, err
		}
	}

	text, _ := tiddler.Js["text"].(string)
	delete(tiddler.Js, "text")
	meta, err := json.Marshal(tiddler.Js) // meta without text
	if err != nil {
		return 0, err
	}

	err = ioutil.WriteFile(filepath.Join(s.tiddlersPath, key + ".tid"), []byte(text), 0644)
	if err != nil {
		return 0, err
	}
	err = ioutil.WriteFile(filepath.Join(s.tiddlersPath, key + ".meta"), meta, 0644)
	if err != nil {
		return 0, err
	}

	return rev, nil
}

// Delete deletes a tiddler with the given key (title) from the store.
func (s *flatFileStore) Delete(ctx context.Context, key string) error {
	key = key2File(key)
	err := os.Remove(filepath.Join(s.tiddlersPath, key + ".meta"))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(s.tiddlersPath, key + ".tid"))
	if err != nil {
		return err
	}
	return nil
}
