package middleware

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type bodyRewriteRoundTripper struct {
	next         http.RoundTripper
	forwardedURL *url.URL
	pathPrefix   string
}

func (rt *bodyRewriteRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.rewriteRequestBody(r)
	resp, err := rt.next.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	rt.rewriteResponseBody(resp)
	// modify response
	return resp, nil
}

func (rt *bodyRewriteRoundTripper) rewriteRequestBody(request *http.Request) {
	bodyBytes, _ := ioutil.ReadAll(request.Body)
	bodyString := string(bodyBytes)

	// replace the forwarded URL with the request url
	bodyString = strings.Replace(bodyString, singleJoiningSlash(request.Host, rt.pathPrefix), rt.forwardedURL.String(), -1)
	bodyBytes = []byte(bodyString)
	request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	request.ContentLength = int64(len(bodyBytes))
	request.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
}

func (rt *bodyRewriteRoundTripper) rewriteResponseBody(response *http.Response) {
	bodyBytes, _ := ioutil.ReadAll(response.Body)
	bodyString := string(bodyBytes)

	bodyString = strings.Replace(bodyString, rt.forwardedURL.String(), singleJoiningSlash(response.Request.Host, rt.pathPrefix), -1)

	bodyBytes = []byte(bodyString)
	response.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
}

func NewBodyRewrite(forwardedURL *url.URL, pathPrefix string, next http.RoundTripper) http.RoundTripper {
	return &bodyRewriteRoundTripper{
		forwardedURL: forwardedURL,
		pathPrefix:   pathPrefix,
		next:         next,
	}
}
