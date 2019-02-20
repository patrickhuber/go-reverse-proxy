package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

type locationRewriteRoundTripper struct {
	next         http.RoundTripper
	forwardedURL *url.URL
	pathPrefix   string
}

func (rt *locationRewriteRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// call the next middleware
	res, err := rt.next.RoundTrip(r)
	if err != nil {
		return nil, err
	}

	// check the response header 'location', if missing bail
	location := res.Header.Get("Location")
	if strings.TrimSpace(location) == "" {
		return res, nil
	}

	target, _ := url.Parse(location)
	target.Host = res.Request.Host
	target.Scheme = res.Request.URL.Scheme

	if strings.TrimSpace(rt.pathPrefix) != "" && strings.TrimSpace(rt.pathPrefix) != "/" {
		target.Path = singleJoiningSlash(rt.pathPrefix, target.Path)
	} else if strings.HasPrefix(target.Path, rt.forwardedURL.Path) {
		target.Path = strings.TrimPrefix(target.Path, rt.forwardedURL.Path)
	}

	res.Header.Set("location", target.String())

	return res, nil
}

// NewLocationRewrite creates a middleware for location header rewritting
func NewLocationRewrite(forwardedURL *url.URL, pathPrefix string, next http.RoundTripper) http.RoundTripper {
	return &locationRewriteRoundTripper{
		forwardedURL: forwardedURL,
		pathPrefix:   pathPrefix,
		next:         next,
	}
}
