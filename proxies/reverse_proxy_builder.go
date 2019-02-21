package proxies

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// RequestRewriter provides an interface for rewriting http requests
type RequestRewriter interface {
	Rewrite(request *http.Request)
}

// ResponseRewriter provides an interface for rewriting http responses
type ResponseRewriter interface {
	Rewrite(response *http.Response)
}

// RequestRewrite defines a function interface for rewriting requests
type RequestRewrite func(request *http.Request)

// ResponseRewrite defines a function interface for rewriting responses
type ResponseRewrite func(response *http.Response)

type reverseProxyBuilder struct {
	requestRewrites  []RequestRewrite
	responseRewrites []ResponseRewrite
	sourcePathPrefix string
}

// RequestCondition provides a function interface for checking http request condition
type RequestCondition func(r *http.Request) bool

// ResponseCondition provides a function interface for checking http response condition
type ResponseCondition func(r *http.Response) bool

func allRequests(r *http.Request) bool {
	return true
}

func allResponses(r *http.Response) bool {
	return true
}

// ReverseProxyBuilder provides a builder interface for creating a reverse proxy
type ReverseProxyBuilder interface {
	ToReverseProxy(transport http.RoundTripper) *httputil.ReverseProxy
	RequestRewrite(rewrite RequestRewrite) ReverseProxyBuilder
	RewriteHost(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder
	RewriteRedirect(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder
	RewriteRequestBody(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder
	RewriteResponseBody(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder
	RewriteRequestCookies(forwardeURL *url.URL, pathPrefix string) ReverseProxyBuilder
	RewriteResponseCookies(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder
	AddRequestHeader(name string, value string) ReverseProxyBuilder
	AddRequestHeaderIf(name string, value string, condition RequestCondition) ReverseProxyBuilder
	SetRequestHeader(name string, value string) ReverseProxyBuilder
	SetRequestHeaderIf(name string, value string, condition RequestCondition) ReverseProxyBuilder
	CopyRequestHeader(source string, destination string) ReverseProxyBuilder
	CopyRequestHeaderIf(source string, destination string, condition RequestCondition) ReverseProxyBuilder
	DeleteRequestHeader(name string, value string) ReverseProxyBuilder
	DeleteRequestHeaderIf(name string, condition RequestCondition) ReverseProxyBuilder
	ReplaceRequestHeader(name string, match string, replace string) ReverseProxyBuilder
	ReplaceRequestHeaderIf(name string, match string, replace string, condition RequestCondition) ReverseProxyBuilder
	ReplaceRequestHeaderValue(name string, match string, replace string) ReverseProxyBuilder
	ReplaceRequestHeaderValueIf(name string, match string, replace string, condition RequestCondition) ReverseProxyBuilder
	ReplaceRequestBody(match, replace string) ReverseProxyBuilder
	ResponseRewrite(rewrite ResponseRewrite) ReverseProxyBuilder
	ReplaceResponseHeader(name, match, replace string) ReverseProxyBuilder
	ReplaceResponseBody(match, replace string) ReverseProxyBuilder
}

func (builder *reverseProxyBuilder) ToReverseProxy(transport http.RoundTripper) *httputil.ReverseProxy {
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			for _, rewrite := range builder.requestRewrites {
				rewrite(req)
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			for _, rewrite := range builder.responseRewrites {
				rewrite(resp)
			}
			return nil
		},
		Transport: transport,
	}

	return reverseProxy
}

func (builder *reverseProxyBuilder) RewriteHost(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder {
	targetQuery := forwardedURL.RawQuery
	return builder.RequestRewrite(func(r *http.Request) {

		originalHost := r.Header.Get(HeaderXForwardedHost)
		if strings.TrimSpace(originalHost) == "" {
			originalHost = r.Host
			r.Header.Set(HeaderXForwardedHost, originalHost)
		}

		originalPath := r.Header.Get(HeaderXForwardedPath)
		if strings.TrimSpace(originalPath) == "" {
			originalPath = r.URL.Path
			r.Header.Set(HeaderXForwardedPath, originalPath)
		}

		originalProtocol := r.Header.Get(HeaderXForwardedProto)
		if strings.TrimSpace(originalProtocol) == "" {
			originalProtocol = r.URL.Scheme
			if strings.TrimSpace(originalProtocol) != "" {
				r.Header.Set(HeaderXForwardedProto, originalProtocol)
			}
		}

		// todo add X-Forwarded-For

		r.URL.Host = forwardedURL.Host
		r.URL.Scheme = forwardedURL.Scheme
		r.Host = forwardedURL.Host

		// if the path prefix matches the request url trim out the path prefix
		if strings.HasPrefix(r.URL.Path, pathPrefix) {
			r.URL.Path = strings.TrimPrefix(originalPath, pathPrefix)
			if !strings.HasPrefix(r.URL.Path, "/") {
				r.URL.Path = "/" + r.URL.Path
			}
		}

		// if the forwarded URL has a path, prepend the forwarded path to the current path
		if strings.TrimSpace(forwardedURL.Path) != "" {
			r.URL.Path = singleJoiningSlash(forwardedURL.Path, r.URL.Path)
		}

		if targetQuery == "" || r.URL.RawQuery == "" {
			r.URL.RawQuery = targetQuery + r.URL.RawQuery
		} else {
			r.URL.RawQuery = targetQuery + "&" + r.URL.RawQuery
		}
	})
}

func (builder *reverseProxyBuilder) RewriteRedirect(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder {
	return builder.ResponseRewrite(func(response *http.Response) {

		// check the response header 'location', if missing bail
		location := response.Header.Get(HeaderLocation)
		if strings.TrimSpace(location) == "" {
			return
		}

		request := response.Request

		target, _ := url.Parse(location)

		// get the original protocol, host and path from the headers
		target.Host = request.Header.Get(HeaderXForwardedHost)
		if strings.TrimSpace(target.Host) == "" {
			target.Host = request.Host
		}

		target.Scheme = request.Header.Get(HeaderXForwardedProto)
		if strings.TrimSpace(target.Scheme) == "" {
			target.Scheme = request.URL.Scheme
		}

		if strings.TrimSpace(pathPrefix) != "" && strings.TrimSpace(pathPrefix) != "/" {
			target.Path = singleJoiningSlash(pathPrefix, target.Path)
		} else if strings.HasPrefix(target.Path, forwardedURL.Path) {
			target.Path = strings.TrimPrefix(target.Path, forwardedURL.Path)
		}

		response.Header.Set(HeaderLocation, target.String())
	})
}
func (builder *reverseProxyBuilder) RewriteRequestBody(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder {
	return builder.RequestRewrite(func(request *http.Request) {
		if request.Body == nil {
			return
		}
		bodyBytes, _ := ioutil.ReadAll(request.Body)
		bodyString := string(bodyBytes)

		source, _ := url.Parse(request.RequestURI)

		originalHost := request.Header.Get(HeaderXForwardedHost)
		if strings.TrimSpace(originalHost) != "" {
			source.Host = originalHost
		}

		originalScheme := request.Header.Get(HeaderXForwardedProto)
		if strings.TrimSpace(originalScheme) != "" {
			source.Scheme = originalScheme
		}

		source.Path = singleJoiningSlash(pathPrefix, source.Path)

		// rewrite any matching urls in the body with the forwarded URL
		bodyString = strings.Replace(bodyString, source.String(), forwardedURL.String(), -1)
		bodyBytes = []byte(bodyString)
		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		request.ContentLength = int64(len(bodyBytes))
		request.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	})
}

func (builder *reverseProxyBuilder) RewriteResponseBody(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder {
	return builder.ResponseRewrite(func(response *http.Response) {
		if response.Body == nil {
			return
		}
		bodyBytes, _ := ioutil.ReadAll(response.Body)
		bodyString := string(bodyBytes)
		response.Body.Close()

		request := response.Request

		source, _ := url.Parse(request.RequestURI)
		originalHost := request.Header.Get(HeaderXForwardedHost)
		if strings.TrimSpace(originalHost) != "" {
			source.Host = originalHost
		}
		originalScheme := request.Header.Get(HeaderXForwardedProto)
		if strings.TrimSpace(originalScheme) != "" {
			source.Scheme = originalScheme
		} else {
			source.Scheme = forwardedURL.Scheme
		}
		source.Path = strings.TrimSuffix(pathPrefix, "/")

		bodyString = strings.Replace(bodyString, forwardedURL.String(), source.String(), -1)
		bodyBytes = []byte(bodyString)
		response.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		response.ContentLength = int64(len(bodyBytes))
		response.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	})
}

func (builder *reverseProxyBuilder) RewriteRequestCookies(forwardeURL *url.URL, pathPrefix string) ReverseProxyBuilder {
	return builder.RequestRewrite(func(request *http.Request) {
		cookies := []*http.Cookie{}
		for _, c := range request.Cookies() {
			cookies = append(cookies, c)

			if !strings.HasPrefix(c.Path, pathPrefix) {
				continue
			}

			// remove the path prefix of the frontend
			c.Path = strings.TrimPrefix(c.Path, pathPrefix)

			// add the path prefix of the back end
			c.Path = singleJoiningSlash(c.Path, forwardeURL.Path)
		}

		// remove all cookies
		request.Header.Del("Cookie")

		// add the modified cookies back
		for _, c := range cookies {
			request.AddCookie(c)
		}
	})
}

func (builder *reverseProxyBuilder) RewriteResponseCookies(forwardedURL *url.URL, pathPrefix string) ReverseProxyBuilder {
	builder.ResponseRewrite(func(response *http.Response) {
		cookies := []*http.Cookie{}
		for _, c := range response.Cookies() {

			cookies = append(cookies, c)

			if !strings.HasPrefix(c.Path, forwardedURL.Path) {
				continue
			}

			// remove the path prefix of the backend
			c.Path = strings.TrimPrefix(c.Path, forwardedURL.Path)

			// add the path prefix of the front end
			c.Path = singleJoiningSlash(c.Path, pathPrefix)
		}

		// remove all cookies, we will add them back one at a time
		response.Header.Del("Set-Cookie")

		// add the cookies back by serializing them to strings
		for _, cookie := range cookies {
			if v := cookie.String(); v != "" {
				response.Header.Add("Set-Cookie", v)
			}
		}
	})
	return builder
}

func (builder *reverseProxyBuilder) RequestRewrite(rewrite RequestRewrite) ReverseProxyBuilder {
	builder.requestRewrites = append(builder.requestRewrites, rewrite)
	return builder
}

func (builder *reverseProxyBuilder) AddRequestHeader(name string, value string) ReverseProxyBuilder {
	return builder.AddRequestHeaderIf(name, value, allRequests)
}

func (builder *reverseProxyBuilder) AddRequestHeaderIf(name string, value string, condition RequestCondition) ReverseProxyBuilder {
	return builder.RequestRewrite(func(request *http.Request) {
		if condition(request) {
			request.Header.Add(name, value)
		}
	})
}

func (builder *reverseProxyBuilder) SetRequestHeader(name string, value string) ReverseProxyBuilder {
	return builder.SetRequestHeaderIf(name, value, allRequests)
}

func (builder *reverseProxyBuilder) SetRequestHeaderIf(name string, value string, condition RequestCondition) ReverseProxyBuilder {
	return builder.RequestRewrite(func(request *http.Request) {
		if condition(request) {
			request.Header.Set(name, value)
		}
	})
}

func (builder *reverseProxyBuilder) CopyRequestHeader(source string, destination string) ReverseProxyBuilder {
	return builder.CopyRequestHeaderIf(source, destination, allRequests)
}

func (builder *reverseProxyBuilder) CopyRequestHeaderIf(source string, destination string, condition RequestCondition) ReverseProxyBuilder {
	return builder.RequestRewrite(func(request *http.Request) {
		if !condition(request) {
			return
		}
		value := request.Header.Get(source)
		request.Header.Set(destination, value)
	})
}

func (builder *reverseProxyBuilder) DeleteRequestHeader(name string, value string) ReverseProxyBuilder {
	return builder.DeleteRequestHeaderIf(name, allRequests)
}

func (builder *reverseProxyBuilder) DeleteRequestHeaderIf(name string, condition RequestCondition) ReverseProxyBuilder {
	builder.requestRewrites = append(builder.requestRewrites, func(request *http.Request) {
		if !condition(request) {
			return
		}
		request.Header.Del(name)
	})
	return builder
}

func (builder *reverseProxyBuilder) ReplaceRequestHeader(name string, match string, replace string) ReverseProxyBuilder {
	return builder.ReplaceRequestHeaderIf(name, match, replace, allRequests)
}

func (builder *reverseProxyBuilder) ReplaceRequestHeaderIf(name string, match string, replace string, condition RequestCondition) ReverseProxyBuilder {
	builder.requestRewrites = append(builder.requestRewrites, func(request *http.Request) {
		if !condition(request) {
			return
		}
		re := regexp.MustCompile(match)
		currentValue := request.Header.Get(name)
		newValue := re.ReplaceAllString(currentValue, replace)
		request.Header.Set(name, newValue)
	})
	return builder
}

func (builder *reverseProxyBuilder) ReplaceRequestHeaderValue(name string, match string, replace string) ReverseProxyBuilder {
	return builder.ReplaceRequestHeaderValueIf(name, match, replace, allRequests)
}

func (builder *reverseProxyBuilder) ReplaceRequestHeaderValueIf(name string, match string, replace string, condition RequestCondition) ReverseProxyBuilder {
	builder.requestRewrites = append(builder.requestRewrites, func(request *http.Request) {
		if !condition(request) {
			return
		}
		re := regexp.MustCompile(match)
		currentValue := request.Header.Get(name)
		segments := strings.Split(currentValue, ",")
		newSegments := []string{}
		for _, segment := range segments {
			newValue := re.ReplaceAllString(segment, replace)
			newSegments = append(newSegments, newValue)
		}
		request.Header.Set(name, strings.Join(newSegments, ","))
	})
	return builder
}

func (builder *reverseProxyBuilder) ReplaceRequestBody(match, replace string) ReverseProxyBuilder {
	return builder.ReplaceRequestBodyIf(match, replace, allRequests)
}

func (builder *reverseProxyBuilder) ReplaceRequestBodyIf(match, replace string, condition RequestCondition) ReverseProxyBuilder {
	return builder.RequestRewrite(func(request *http.Request) {
		if !condition(request) {
			return
		}

		regex := regexp.MustCompile(match)
		bodyBytes, _ := ioutil.ReadAll(request.Body)
		bodyString := string(bodyBytes)
		bodyString = regex.ReplaceAllString(bodyString, replace)
		bodyBytes = []byte(bodyString)
		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		request.ContentLength = int64(len(bodyBytes))
		request.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	})
}

func (builder *reverseProxyBuilder) ResponseRewrite(rewrite ResponseRewrite) ReverseProxyBuilder {
	builder.responseRewrites = append(builder.responseRewrites, rewrite)
	return builder
}

func (builder *reverseProxyBuilder) ReplaceResponseHeader(name, match, replace string) ReverseProxyBuilder {
	return builder.ReplaceResponseHeaderIf(name, match, replace, allResponses)
}

func (builder *reverseProxyBuilder) ReplaceResponseHeaderIf(name, match, replace string, condition ResponseCondition) ReverseProxyBuilder {
	return builder.ResponseRewrite(func(response *http.Response) {
		if !condition(response) {
			return
		}

	})
}

func (builder *reverseProxyBuilder) ReplaceResponseBody(match, replace string) ReverseProxyBuilder {
	return builder.ReplaceResponseBodyIf(match, replace, allResponses)
}

func (builder *reverseProxyBuilder) ReplaceResponseBodyIf(match, replace string, condition ResponseCondition) ReverseProxyBuilder {
	builder.ResponseRewrite(func(response *http.Response) {
		if !condition(response) {
			return
		}
		regex := regexp.MustCompile(match)
		bodyBytes, _ := ioutil.ReadAll(response.Body)
		bodyString := string(bodyBytes)
		bodyString = regex.ReplaceAllString(bodyString, replace)
		bodyBytes = []byte(bodyString)
		response.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	})
	return builder
}

// NewReverseProxyBuilder creates a reverse proxy builder that performs common rewrite functions with simple interfaces
func NewReverseProxyBuilder() ReverseProxyBuilder {
	return &reverseProxyBuilder{
		requestRewrites:  []RequestRewrite{},
		responseRewrites: []ResponseRewrite{},
	}
}
