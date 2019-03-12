package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"time"

	"github.com/buger/jsonparser"
)

var upstreamFlag = flag.String("upstream", "https://sentry.io", "Upstream Sentry server")

func newHandler(upstream string) http.HandlerFunc {
	u, _ := url.Parse(upstream)
	apiRe := regexp.MustCompile(`^/api/\d+/store/$`)
	proxy := &httputil.ReverseProxy{Director: func(req *http.Request) {}}

	return func(w http.ResponseWriter, req *http.Request) {
		// Don't worry about legacy endpoints, only allow POST
		if req.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		// Only accept requests on API store endpoint
		if !apiRe.MatchString(req.URL.Path) {
			http.Error(w, "not found", 404)
			return
		}
		// Make sure we have a valid client IP address to work with
		clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		// Try to inject our clientIP into the `user.ip_address` chunk
		body, err = jsonparser.Set(body, []byte(`"`+clientIP+`"`), "user", "ip_address")
		if err != nil {
			http.Error(w, "invalid JSON body", 400)
			return
		}
		// Rewrite our request for the proxy
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Header.Set("Host", u.Host)
		req.ContentLength = int64(len(body))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		// Sling out mutated request upstream
		proxy.ServeHTTP(w, req)
	}
}

func main() {
	flag.Parse()
	server := &http.Server{
		Addr:           ":8080",
		Handler:        newHandler(*upstreamFlag),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Println(*upstreamFlag)
	log.Fatal(server.ListenAndServe())
}
