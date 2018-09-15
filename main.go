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

// widdly is a self-hosted web application which can serve as a personal TiddlyWiki.
package main

import (
	"crypto/subtle"
	"flag"
	"log"
	"net/http"

	"./api"
	"./store"
//	_ "./store/bolt"
//	_ "./store/sqlite"
	_ "./store/flatFile"

)

var (
	addr       = flag.String("http", "127.0.0.1:8080", "HTTP service address")
	dataSource = flag.String("db", "widdly.db", "Database path/file")
)

func main() {
	flag.Parse()

	mux := api.NewRootMux()
	api.InitHandle(mux)

	// Open the data store and tell HTTP handlers to use it.
	api.StoreDb = store.MustOpen(*dataSource)

	api.Authenticate = func(user string, pwd string) (bool) {
		if subtle.ConstantTimeCompare([]byte(pwd), []byte("test")) == 1 &&
			subtle.ConstantTimeCompare([]byte(user), []byte("admin")) == 1 { // DON'T use subtle.ConstantTimeCompare like this!
			return true
		}
		return false
	}

	log.Fatal(http.ListenAndServe(*addr, mux))
}


