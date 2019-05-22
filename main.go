package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"time"

	"github.com/buger/jsonparser"
	"github.com/go-http-utils/logger"
)

const Version = "0.0.0"
const MaxRequestLength = 250 * 1024

var (
	upstreamFlag     = UpstreamFlag("upstream", &url.URL{Scheme: "https", Host: "sentry.io"}, "Upstream Sentry server")
	listenFlag       = flag.String("listen", "127.0.0.1:8080", "Address to bind to")
	readTimeoutFlag  = flag.Duration("read-timeout", 10*time.Second, "Read timeout")
	writeTimeoutFlag = flag.Duration("write-timeout", 10*time.Second, "Write timeout")
	versionFlag      = flag.Bool("version", false, "Print program version")
)

func newHandler(upstream *url.URL) http.HandlerFunc {
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
			log.Println(err)
			http.Error(w, err.Error(), 400)
			return
		}
		body, err := ioutil.ReadAll(http.MaxBytesReader(w, req.Body, MaxRequestLength))
		req.Body.Close()
		if err != nil {
			log.Println(err)
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
		req.Host = upstream.Host
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Header.Set("Host", upstream.Host)
		req.ContentLength = int64(len(body))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		// Sling our mutated request upstream
		proxy.ServeHTTP(w, req)
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
}

func main() {
	if *versionFlag {
		fmt.Fprintf(os.Stderr, "sentry-proxy version: %s (%s on %s/%s; %s)\n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.Compiler)
		os.Exit(0)
	}

	handler := logger.DefaultHandler(
		newHandler(upstreamFlag),
	)

	server := &http.Server{
		Addr:           *listenFlag,
		Handler:        handler,
		ReadTimeout:    *readTimeoutFlag,
		WriteTimeout:   *writeTimeoutFlag,
		MaxHeaderBytes: 10 * 1024,
	}

	fmt.Println(`                _
               | |
 ___  ___ _ __ | |_ _ __ _   _ ______ _ __  _ __ _____  ___   _
/ __|/ _ \ '_ \| __| '__| | | |______| '_ \| '__/ _ \ \/ / | | |
\__ \  __/ | | | |_| |  | |_| |      | |_) | | | (_) >  <| |_| |
|___/\___|_| |_|\__|_|   \__, |      | .__/|_|  \___/_/\_\\__, |
                          __/ |      | |                   __/ |
                         |___/       |_|                  |___/
`)
	fmt.Println("- listen: ", *listenFlag)
	fmt.Println("- upstream: ", upstreamFlag)
	fmt.Println("- read-timeout: ", *readTimeoutFlag)
	fmt.Println("- write-timeout: ", *writeTimeoutFlag)
	fmt.Println("\n* Ready to serve.")
	log.Fatal(server.ListenAndServe())
}
