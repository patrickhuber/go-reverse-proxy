package proxies

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/patrickhuber/go-reverse-proxy/middleware"
)

func NewReverseProxy(
	forwardedURL *url.URL,
	pathPrefix string,
	skipTLSValidation bool) (http.Handler, error) {

	// create the request pipeline
	transport := middleware.NewTransport(skipTLSValidation)
	locationRewrite := middleware.NewLocationRewrite(forwardedURL, pathPrefix, transport)
	bodyRewrite := middleware.NewBodyRewrite(forwardedURL, pathPrefix, locationRewrite)

	targetQuery := forwardedURL.RawQuery

	reverseProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			originalHost := r.Host
			r.URL.Host = forwardedURL.Host
			r.URL.Scheme = forwardedURL.Scheme
			r.Host = forwardedURL.Host

			// if the forwarded URL has a path, prepend the forwarded path to the current path
			if strings.TrimSpace(forwardedURL.Path) != "" {
				r.URL.Path = singleJoiningSlash(forwardedURL.Path, r.URL.Path)
			} else if strings.HasPrefix(r.URL.Path, pathPrefix) {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, pathPrefix)
				if !strings.HasPrefix(r.URL.Path, "/") {
					r.URL.Path = "/" + r.URL.Path
				}
			}

			if targetQuery == "" || r.URL.RawQuery == "" {
				r.URL.RawQuery = targetQuery + r.URL.RawQuery
			} else {
				r.URL.RawQuery = targetQuery + "&" + r.URL.RawQuery
			}

			r.Header.Set("X-Forwarded-Host", originalHost)
		},
		Transport: bodyRewrite,
	}
	return reverseProxy, nil
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]

	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
