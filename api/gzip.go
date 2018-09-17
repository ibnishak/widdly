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

// HTTP handlers for gzip
package api

import (
	"compress/gzip"
	"net/http"
	"strings"
//	"fmt"
)

var (
	GzipLevel = 5 // disable = 0, DefaultCompression = -1, BestSpeed = 1, BestCompression = 9
)

type GzipResponseWriter struct {
	http.ResponseWriter
	gzip *gzip.Writer
}

func (w *GzipResponseWriter) Write(p []byte) (int, error) {
	if w.gzip == nil {
		return w.ResponseWriter.Write(p)
	}

	n, err := w.gzip.Write(p)
	//fmt.Println("[gz]Write()", w, n, err)
	return n, err
}

func (w *GzipResponseWriter) Close() (error) {
	if w.gzip != nil {
		return w.gzip.Close()
	}
	//fmt.Println("[gz]Close()", w)
	return nil
}

func CanAcceptsGzip(r *http.Request) (bool) {
	s := strings.ToLower(r.Header.Get("Accept-Encoding"))
	for _, ss := range strings.Split(s, ",") {
		if strings.HasPrefix(ss, "gzip") {
			return true
		}
	}
	return false
}

func TryGzipResponse(w http.ResponseWriter, r *http.Request) (*GzipResponseWriter) {
	if !CanAcceptsGzip(r) || GzipLevel == 0 {
		return &GzipResponseWriter{w, nil}
	}

	gw, err := gzip.NewWriterLevel(w, GzipLevel)
	if err != nil {
		gw = gzip.NewWriter(w)
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	//fmt.Println("[gz]", r, gw)

	return &GzipResponseWriter{w, gw}
}

