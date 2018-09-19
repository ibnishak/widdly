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
	"flag"
	"log"
	"net/http"
	"context"

	"time"
	"crypto/sha256"
	"crypto/rand"
	"encoding/hex"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"encoding/csv"

	"./api"
	"./store"
	_ "./store/bolt"
	_ "./store/sqlite"
	_ "./store/flatFile"

)

var (
	VERSION = "SELFBUILD" // injected by buildflags

	addr       = flag.String("http", "127.0.0.1:8080", "HTTP service address")
	dataSource = flag.String("db", "widdly.db", "Database path/file")
	dataType   = flag.String("dbt", "flatFile", "Database type")

	gziplv   = flag.Int("gz", 1, "gzip compress level, 0 for disable")
	rev   = flag.Int("rev", -1, "Max keeping history count, 0 for disable, -1 for unlimit")

	accounts   = flag.String("acc", "user.lst", "user list file")
	// eache line : <user>\t<salt>\t<sha256(pwd)>
	// comment start with '#'

	user   = flag.String("u", "", "encode user name to user.lst format")
	pass   = flag.String("p", "", "encode user password to user.lst format")
)

func main() {
	flag.Parse()

	if *user != "" && *pass != "" {
		uid := *user
		salt := genSalt()
		hash := pwdHashStr(*pass, salt)

		fmt.Println("# user\tsalt\thash")
		fmt.Printf("%s\t%s\t%s\n", uid, salt, hash)
		return
	}

	fmt.Println("[server] version =", VERSION)
	fmt.Println("[server] gzip level =", *gziplv)
	fmt.Println("[server] max history count =", *rev)

	// read in accounts
	af, err := os.Open(*accounts)
	if err != nil {
		fmt.Println("[Open Accounts error]", err)
		return
	}

	userlist, err := readTSV(af)
	if err != nil {
		fmt.Println("[Parse Accounts error]", *accounts, err)
		return
	}



	mux := api.NewRootMux()
	api.InitHandle(mux)

	// Open the data store and tell HTTP handlers to use it.
	db, err := store.Open(*dataType, *dataSource)
	if err != nil {
		list := store.ListBackend()
		fmt.Println("[Open backend error]", err)
		fmt.Println("[backend list]", list)
		return
	}
	defer db.Close()
	db.SetMaxHistory(*rev)

	api.StoreDb = db
	api.GzipLevel = *gziplv

	api.Authenticate = func(user string, pwd string) (bool) {
		t0 := time.Now().Add(time.Second)
		defer time.Sleep(time.Until(t0)) // prevent brute force & timing attacks

		u, ok := userlist[user]
		if !ok {
			return false
		}

		hpwd := pwdHashStr(pwd, u.Salt)
		if hpwd == u.Hash {
			return true
		}
		return false
	}

	srv := &http.Server{Addr: *addr, Handler: mux}

	waitClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(waitClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("HTTP server ListenAndServe: %v", err)
	}

	<-waitClosed
}

type User struct {
	UID            string
	Salt           string
	Hash           string
}

func readTSV(input io.ReadCloser) (map[string]*User, error) {
	defer input.Close()

	reader := csv.NewReader(input)
	reader.Comma = '\t' // Use tab-delimited instead of comma
	reader.FieldsPerRecord = -1

	list := make(map[string]*User)
	for idx := 0; ; idx++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("[csv parse error]", err)
			return nil, err
		}

		//Vln(5, idx, row[0], row[0] != "")

		if len(row) < 3 {
			continue
		}

		if row[0] == "" {
			continue
		}
		if strings.HasPrefix(row[0], "#") {
			continue
		}

		uid := row[0]
		salt := row[1]
		hash := row[2]

		list[uid] = &User{
			UID: uid,
			Salt: salt,
			Hash: hash,
		}

	}

	return list, nil
}

/*func readTSV(input io.ReadCloser) (map[string]*User, error) {
	defer input.Close()

	list := make(map[string]*User)
	r := bufio.NewReader(input)
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			break
		}

		row := strings.Split(strings.Trim(line, "\n"), "\t")
		if len(row) < 3 {
			continue
		}

		if row[0] == "" {
			continue
		}
		if strings.HasPrefix(row[0], "#") {
			continue
		}

		uid := row[0]
		salt := row[1]
		hash := row[2]

		list[uid] = &User{
			UID: uid,
			Salt: salt,
			Hash: hash,
		}
	}
}*/

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func genSalt() string {
	buf, err := generateRandomBytes(15)
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(buf)
}

func hashBytes(a []byte) []byte {
	shah := sha256.New()
	shah.Write(a)
	return shah.Sum([]byte(""))
}

func pwdHash(pwd string, salt string) []byte {
	return hashBytes([]byte(pwd + "-:-" + salt))
}

func pwdHashStr(pwd string, salt string) string {
	return hex.EncodeToString(pwdHash(pwd, salt))
}

