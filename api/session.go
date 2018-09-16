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

package api

import (
	"fmt"
//	"log"
	"net/http"

	"errors"
	"encoding/base64"
	"crypto/rand"
	"sync"
	"time"
)

var (
	ErrCookie = errors.New("Cookie not found")
	ErrRNG = errors.New("Could not successfully read from the system CSPRNG")
)

var (
	CookieName = "sid"
	CookieLifeTime = 24 * 7 * time.Hour
	SessionTimeout = 30 * 60 * time.Second
)

type Store struct {
	lock  sync.RWMutex
	t     time.Time               //last access time
	val   map[string]interface{}  //session store
}

type Session struct {
	lock     sync.RWMutex
	clients  map[string]*Store
}

func NewSession() (*Session) {
	s := &Session {
		clients: make(map[string]*Store),
	}

	go s.cleaner()

	return s
}

func (s *Session) cleaner() {
	for {
		list := make([]string, 0)
		s.lock.RLock()
		for sid, _ := range s.clients {
			list = append(list, sid)
		}
		s.lock.RUnlock()

		for _, sid := range list {
			s.lock.RLock()
			u, ok := s.clients[sid]
			s.lock.RUnlock()
			if !ok {
				continue
			}

			if time.Now().After(u.t) {
				s.lock.Lock()
				delete(s.clients, sid)
				s.lock.Unlock()
			}
		}

		time.Sleep(30 * time.Second)
	}
}

func (s *Session) GetSID(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return "", ErrCookie
	}

	return cookie.Value, nil
}

func (s *Session) newSession(sid string) (*Store) {
	sess := s.getSession(sid)
	if sess != nil {
		sess.ReNew()
		return sess
	}

	sess = NewStore()

	s.lock.Lock()
	s.clients[sid] = sess
	s.lock.Unlock()

	return sess
}

func (s *Session) getSession(sid string) (*Store) {
	s.lock.RLock()
	sess, ok := s.clients[sid]
	s.lock.RUnlock()
	if !ok {
		return nil
	}
	return sess
}

func (s *Session) Start(w http.ResponseWriter, r *http.Request) (*Store, error) {
	var session *Store

	sid, err := s.GetSID(r)
	if err != nil {
		sid, err = genSID()
		if err != nil {
			return nil, err
		}
	}
	session = s.newSession(sid)

	cookie := &http.Cookie{
		Name: CookieName,
		Value: sid,
		Path: "/",
		HttpOnly: true,
		Expires: time.Now().Add(CookieLifeTime),
		MaxAge: int(CookieLifeTime.Seconds()),
	}
	http.SetCookie(w, cookie)

	return session, nil
}

func (s *Session) destroy(sid string) {
	s.lock.RLock()
	_, ok := s.clients[sid]
	s.lock.RUnlock()
	if !ok {
		return
	}

	s.lock.Lock()
	delete(s.clients, sid)
	s.lock.Unlock()
}

func (s *Session) Destroy(w http.ResponseWriter, r *http.Request) {
	sid, err := s.GetSID(r)
	if err != nil {
		return
	}
	s.destroy(sid)

	// force cookie timeout
	cookie := &http.Cookie{
		Name: CookieName,
		HttpOnly: true,
		Expires: time.Now(),
		MaxAge: -1,
	}
	http.SetCookie(w, cookie)
}

func (s *Session) Dump() {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for sid, sess := range s.clients {
		fmt.Println("[dump]", sid, sess)
	}
}

func NewStore() (*Store) {
	s := &Store {
		val: make(map[string]interface{}),
		t: time.Now().Add(SessionTimeout),
	}
	return s
}

func (s *Store) IsLogin() (bool) {
	_, ok := s.Get("uid")
	return ok
}

func (s *Store) Login(user string) {
	s.Set("uid", user)
}

func (s *Store) Get(key string) (interface{}, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	val, ok := s.val[key]
	return val, ok
}

func (s *Store) Set(key string, val interface{}) {
	s.lock.Lock()

	s.val[key] = val
	s.t = time.Now().Add(SessionTimeout)

	s.lock.Unlock()
}

func (s *Store) Del(key string) {
	s.lock.Lock()
	delete(s.val, key)
	s.lock.Unlock()
}

func (s *Store) ReNew() {
	s.lock.Lock()
	s.t = time.Now().Add(SessionTimeout)
	s.lock.Unlock()
}

func genSID() (string, error) {
	b := make([]byte, 18)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", ErrRNG
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

