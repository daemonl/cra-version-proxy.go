package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func doError(rw http.ResponseWriter, req *http.Request, err error) {
	log.Printf("ERROR: %s", err.Error())
	rw.WriteHeader(500)
}

func main() {
	sourceURL, err := url.Parse(os.Getenv("SOURCE_URL"))
	if err != nil {
		log.Fatalf("Invalid url in $SOURCE_URL: %s", err.Error())
	}

	cacheDir := os.Getenv("CACHE_DIR")
	bindAddress := os.Getenv("BIND")
	defaultVersion := os.Getenv("DEFAULT_VERSION")

	var handler http.Handler

	handler = fileServer{
		root:      http.Dir(cacheDir),
		sourceURL: sourceURL,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}

	handler = VersionSwitch(defaultVersion)(handler)
	handler = AppRewrite(handler)

	if err := http.ListenAndServe(bindAddress, handler); err != nil {
		log.Fatal(err.Error())
	}
}

type fileServer struct {
	root      http.Dir
	sourceURL *url.URL
	client    *http.Client
}

func (fs fileServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("X-Cache", "hit")
	name := path.Clean(req.URL.Path)
	err := fs.tryServeFile(rw, req, name)
	if os.IsNotExist(err) {
		rw.Header().Set("X-Cache", "miss")
		if err := fs.doCacheFetch(rw, req, name); err != nil {
			doError(rw, req, err)
			return
		}
		if err := fs.tryServeFile(rw, req, name); err != nil {
			doError(rw, req, err)
			return
		}
	} else if err != nil {
		doError(rw, req, err)
		return
	}
}

func (fs fileServer) doCacheFetch(rw http.ResponseWriter, req *http.Request, name string) error {
	// TODO: Exclusive Lock - Will multiple concurrent fetches corrupt the file
	// or error out?

	urlOut := &url.URL{
		Path:   path.Join(fs.sourceURL.Path, name),
		Scheme: fs.sourceURL.Scheme,
		Host:   fs.sourceURL.Host,
	}

	res, err := fs.client.Get(urlOut.String())
	if err != nil {
		return err
	}

	//  Taken from http.Dir.Open
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return errors.New("http: invalid character in file path")
	}
	fullName := filepath.Join(string(fs.root), filepath.FromSlash(path.Clean("/"+name)))
	// Done with http.Dir.Open clone

	os.MkdirAll(filepath.Dir(fullName), os.FileMode(0770))
	cacheFile, err := os.Create(fullName)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	return res.Write(cacheFile)

}

func (fs fileServer) tryServeFile(rw http.ResponseWriter, req *http.Request, name string) error {
	// http.Dir.Open ensures the file is rooted at root.
	f, err := fs.root.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	buffered := bufio.NewReader(f)
	parsedResponse, err := http.ReadResponse(buffered, nil)
	if err != nil {
		return err
	}
	defer parsedResponse.Body.Close()

	// TODO: Discard and delete if cache is expired.

	rwHeader := rw.Header()
	for key, vals := range parsedResponse.Header {
		for _, val := range vals {
			rwHeader.Add(key, val)
		}
	}

	rw.WriteHeader(parsedResponse.StatusCode)
	_, err = io.Copy(rw, parsedResponse.Body)
	return err
}

const VERSION_COOKIE_NAME = "version-override"

var reVersionUnsafe = regexp.MustCompile(`[^a-zA-Z0-9]`)

// VersionSwitch rewrites requests to a directory prefixed with the requested
// or default version.  The version can be set with a querystirng version= or
// cookie. When the querystring parameter is set, the cookie is sent with the
// response so that requests for resources in HTML pages (css, images etc) will
// also get the correct prefix.
func VersionSwitch(defaultVersion string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

			version := defaultVersion
			if queryVersion := req.URL.Query().Get("version"); queryVersion != "" {
				// read the requested version from the QS
				version = queryVersion

				// Set a cookie so that dependencies are also loaded with the
				// correct version
				versionCookie := &http.Cookie{
					Name: VERSION_COOKIE_NAME,
					// Allowing JS code to view and modify could extend
					// functionality.
					HttpOnly: false,
					Path:     "/",
					Expires:  time.Now().Add(time.Hour),
					Value:    version,
				}
				http.SetCookie(rw, versionCookie)

				// Don't cacne versioned entry points
				rw.Header().Set("Cache-Control", "no-store")
			} else if versionCookie, _ := req.Cookie(VERSION_COOKIE_NAME); versionCookie != nil {
				// read the requested version from the cookie
				version = versionCookie.Value

				// refresh the cookie
				versionCookie.Expires = time.Now().Add(time.Hour)
				http.SetCookie(rw, versionCookie)

				// Don't cache versioned resources (Cookies are not considered
				// by browsers when looking up cached responses)
				rw.Header().Set("Cache-Control", "no-store")
			}

			version = url.PathEscape(version)
			newPath := path.Clean("/" + path.Join(version, req.URL.Path))
			log.Printf("Serve %s with version %s => %s", req.URL.Path, version, newPath)
			req.URL.Path = newPath
			next.ServeHTTP(rw, req)
		})
	}
}

// AppRewrite rewrites all requests without an extension to /index.html
func AppRewrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if ext := path.Ext(req.URL.Path); ext == "" {
			req.URL.Path = "/index.html"
		}
		next.ServeHTTP(rw, req)
	})
}
