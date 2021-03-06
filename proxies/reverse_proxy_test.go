package proxies_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/patrickhuber/go-reverse-proxy/proxies"

	"github.com/gorilla/mux"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReverseProxy", func() {
	type PathUnion struct {
		Front string
		Back  string
	}
	DescribeTable("redirects",
		func(paths *PathUnion) {
			fmt.Println()
			fmt.Printf("front path: '%s', back path: '%s'", paths.Front, paths.Back)
			fmt.Println()
			router := mux.NewRouter()
			router.HandleFunc(paths.Back+"/ok", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			router.HandleFunc(paths.Back+"/redirect", func(w http.ResponseWriter, r *http.Request) {
				redirectURL := &url.URL{
					Host:   r.Host,
					Path:   paths.Back + "/ok",
					Scheme: "http",
				}
				http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
			})
			router.HandleFunc(paths.Back+"/redirect/query/decoded", func(w http.ResponseWriter, r *http.Request) {
				queryStringURL := &url.URL{
					Host:   r.Host,
					Path:   paths.Back + "/ok",
					Scheme: "http",
				}
				redirectURL := &url.URL{
					Host:     "www.google.com",
					Scheme:   "http",
					RawQuery: fmt.Sprintf("q=%s", queryStringURL.String()),
				}

				http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
			})
			router.HandleFunc(paths.Back+"/redirect/query/encoded", func(w http.ResponseWriter, r *http.Request) {
				queryStringURL := &url.URL{
					Host:   r.Host,
					Path:   paths.Back + "/ok",
					Scheme: "http",
				}
				redirectURL := &url.URL{
					Host:     "www.google.com",
					Scheme:   "http",
					RawQuery: fmt.Sprintf("q=%s", url.QueryEscape(queryStringURL.String())),
				}

				http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
			})

			backend := httptest.NewServer(router)

			backendURL, err := url.Parse(backend.URL)
			backendURL.Path = paths.Back + "/" + backendURL.Path
			Expect(err).To(BeNil())

			reverseProxy := proxies.NewReverseProxyBuilder().
				AddRequestHeader("X-Forwarded-Proto", "http").
				RewriteHost(backendURL, paths.Front).
				RewriteRedirect(backendURL, paths.Front).
				ToReverseProxy(&http.Transport{})

			frontend := httptest.NewServer(reverseProxy)
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			// sanity check to call the ok endpoint
			res, err := http.Get(frontend.URL + paths.Front + "/ok")
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusOK))

			// just a standard redirect, make sure the core URL is rewritten
			res, err = client.Get(frontend.URL + paths.Front + "/redirect")
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusTemporaryRedirect))
			Expect(res.Header.Get("Location")).To(Equal(frontend.URL + paths.Front + "/ok"))

			// redirect with embedded URL in the query string URL decoded
			res, err = client.Get(frontend.URL + paths.Front + "/redirect/query/decoded")
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusTemporaryRedirect))
			actual := res.Header.Get("Location")
			expected := fmt.Sprintf("http://www.google.com?q=%s", frontend.URL+paths.Front+"/ok")
			Expect(actual).To(Equal(expected))

			// redirect with embedded URL in the query string URL encoded
			res, err = client.Get(frontend.URL + paths.Front + "/redirect/query/encoded")
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusTemporaryRedirect))
			actual = res.Header.Get("Location")
			expected = fmt.Sprintf("http://www.google.com?q=%s", url.QueryEscape(frontend.URL+paths.Front+"/ok"))
			Expect(actual).To(Equal(expected))
		},
		Entry("no source path no target path", &PathUnion{Front: "", Back: ""}),
		Entry("source path target no path", &PathUnion{Front: "/one", Back: ""}),
		Entry("no source path target path", &PathUnion{Front: "", Back: "/two"}),
		Entry("target and source have path", &PathUnion{Front: "/one", Back: "/two"}))
	Context("passthrough", func() {
		var (
			backend  *httptest.Server
			frontend *httptest.Server
		)
		BeforeEach(func() {
			router := mux.NewRouter()
			router.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			router.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				for name, value := range r.Header {
					fmt.Fprintf(w, "%s=%s", name, value)
					fmt.Fprintln(w)
				}
			})
			router.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				redirectURL := fmt.Sprintf("http://%s/ok", r.Host)
				http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			})
			router.HandleFunc("/redirect-encoded", func(w http.ResponseWriter, r *http.Request) {
				queryStringRedirectURL := &url.URL{
					Host:   r.Host,
					Scheme: r.URL.Scheme,
					Path:   "ok",
				}
				if strings.TrimSpace(queryStringRedirectURL.Scheme) == "" {
					queryStringRedirectURL.Scheme = "http"
				}
				redirectURIEncoded := url.QueryEscape(queryStringRedirectURL.String())
				redirectURL := fmt.Sprintf("http://%s/ok?redirect_uri=%s", r.Host, redirectURIEncoded)
				http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			})
			router.HandleFunc("/redirect-decoded", func(w http.ResponseWriter, r *http.Request) {
				queryStringRedirectURL := &url.URL{
					Host:   r.Host,
					Scheme: r.URL.Scheme,
					Path:   "ok",
				}
				if strings.TrimSpace(queryStringRedirectURL.Scheme) == "" {
					queryStringRedirectURL.Scheme = "http"
				}
				redirectURL := fmt.Sprintf("http://%s/ok?redirect_uri=%s", r.Host, queryStringRedirectURL.String())
				http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			})
			router.HandleFunc("/is-match", func(w http.ResponseWriter, r *http.Request) {
				bodyBytes, _ := ioutil.ReadAll(r.Body)
				bodyString := string(bodyBytes)
				if !strings.Contains(bodyString, r.Host) {
					w.WriteHeader(http.StatusBadRequest)
				}
			})
			router.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
				url, _ := url.Parse(r.RequestURI)
				url.Host = r.Host
				url.Scheme = "http"
				w.Write([]byte(url.String()))
			})

			backend = httptest.NewServer(router)

			backendURL, err := url.Parse(backend.URL)
			Expect(err).To(BeNil())

			reverseProxy := proxies.NewReverseProxyBuilder().
				CopyRequestHeader("X-Original-Host", "X-Forwarded-Host").
				CopyRequestHeader("X-Original-Path", "X-Forwarded-Path").
				RewriteHost(backendURL, "/").
				RewriteRequestBody(backendURL, "/").
				RewriteRedirect(backendURL, "/").
				RewriteResponseBody(backendURL, "/").
				ToReverseProxy(&http.Transport{})

			frontend = httptest.NewServer(reverseProxy)
		})
		AfterEach(func() {
			backend.Close()
			frontend.Close()
		})
		It("can add x forwarded headers", func() {
			res, err := http.Get(frontend.URL + "/headers")
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusOK))

			frontendURL, err := url.Parse(frontend.URL)
			Expect(err).To(BeNil())

			body, err := ioutil.ReadAll(res.Body)
			Expect(err).To(BeNil())

			bodyString := string(body)
			searchString := fmt.Sprintf("X-Forwarded-Host=[%s]", frontendURL.Host)
			Expect(strings.Contains(bodyString, searchString)).To(BeTrue(), "cannot find x-forwarded-host header in response")

			searchString = "X-Forwarded-Path=[/headers]"
			Expect(strings.Contains(bodyString, searchString)).To(BeTrue(), "cannot find x-forwarded-path header in response")

		})

		It("can rewrite request body", func() {
			body := fmt.Sprintf("%s", frontend.URL+"/is-match")
			res, err := http.Post(frontend.URL+"/is-match", "text/plain", bytes.NewBufferString(body))
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusOK))
		})
		It("can rewrite response body", func() {
			res, err := http.Get(frontend.URL + "/info")
			Expect(err).To(BeNil())
			defer res.Body.Close()

			Expect(res.StatusCode).To(Equal(http.StatusOK))
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).To(BeNil())
			Expect(string(body)).To(Equal(frontend.URL + "/info"))
		})

		It("can copy x forwarded host header from other header", func() {

			client := &http.Client{}
			req, err := http.NewRequest("GET", frontend.URL+"/headers", nil)
			Expect(err).To(BeNil())

			req.Header.Set("X-Original-Host", "www.example.com")
			res, err := client.Do(req)
			Expect(err).To(BeNil())
			Expect(res.Body).ToNot(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusOK))

			bodyBytes, err := ioutil.ReadAll(res.Body)
			Expect(err).To(BeNil())

			bodyString := string(bodyBytes)
			regex, err := regexp.Compile("(?i)x-forwarded-host\\s*=\\s*\\[([^]]+)\\]")
			Expect(err).To(BeNil())

			Expect(regex.MatchString(bodyString)).To(BeTrue())
		})

		It("can copy x forwarded path header from other header", func() {
			client := &http.Client{}
			req, err := http.NewRequest("GET", frontend.URL+"/ok", nil)
			Expect(err).To(BeNil())

			req.Header.Set("X-Original-Path", "/headers")
			res, err := client.Do(req)
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusOK))
			Expect(res.Body).ToNot(BeNil())

			bodyBytes, err := ioutil.ReadAll(res.Body)
			Expect(err).To(BeNil())

			bodyString := string(bodyBytes)

			regex, err := regexp.Compile("(?i)x-forwarded-path\\s*=\\s*\\[([^]]+)\\]")
			Expect(err).To(BeNil())

			Expect(regex.MatchString(bodyString)).To(BeTrue())
		})
	})
	Context("new path", func() {
		var (
			frontSidePath string
			backSidePath  string
			backend       *httptest.Server
			frontend      *httptest.Server
		)
		AfterEach(func() {
			router := mux.NewRouter()
			router.HandleFunc(backSidePath+"/ok", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			backend = httptest.NewServer(router)
			defer backend.Close()

			backendURL, err := url.Parse(backend.URL)
			Expect(err).To(BeNil())

			backendURL.Path = backSidePath
			reverseProxy := proxies.NewReverseProxyBuilder().
				RewriteHost(backendURL, frontSidePath).
				ToReverseProxy(&http.Transport{})

			frontend = httptest.NewServer(reverseProxy)
			defer frontend.Close()

			res, err := http.Get(frontend.URL + frontSidePath + "/ok")
			Expect(err).To(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusOK))
		})
		It("can rewrite source with subpath to target without subpath", func() {
			frontSidePath = "/test"
			backSidePath = ""
		})
		It("can rewrite source without subpath to target with subpath", func() {
			frontSidePath = ""
			backSidePath = "/test"
		})
		It("can rewrite source to different subpath than taget", func() {
			frontSidePath = "/one"
			backSidePath = "/two"
		})
	})
	Context("cookies", func() {
		var (
			frontSidePath string
			backSidePath  string
			backend       *httptest.Server
			frontend      *httptest.Server
		)
		It("can rewrite response cookie path when server drops cookie at root", func() {

			frontSidePath = "/test"
			backSidePath = ""
		})
		AfterEach(func() {
			router := mux.NewRouter()
			router.HandleFunc(backSidePath+"/set-cookies", func(w http.ResponseWriter, r *http.Request) {
				expire := time.Now().AddDate(0, 0, 1)
				cookie := http.Cookie{
					Name:    "cookie",
					Value:   "value",
					Expires: expire,
				}
				http.SetCookie(w, &cookie)
				w.WriteHeader(http.StatusOK)
			})
			router.HandleFunc(backSidePath+"/cookies", func(w http.ResponseWriter, r *http.Request) {
				for _, c := range r.Cookies() {
					fmt.Fprintf(w, "%v", c)
					fmt.Fprintln(w)
				}
				w.WriteHeader(http.StatusOK)
			})

			backend = httptest.NewServer(router)
			defer backend.Close()

			backendURL, err := url.Parse(backend.URL)
			Expect(err).To(BeNil())

			backendURL.Path = backSidePath
			reverseProxy := proxies.NewReverseProxyBuilder().
				RewriteHost(backendURL, frontSidePath).
				RewriteRequestCookies(backendURL, frontSidePath).
				RewriteResponseCookies(backendURL, frontSidePath).
				ToReverseProxy(&http.Transport{})

			frontend = httptest.NewServer(reverseProxy)
			defer frontend.Close()

			cookieJar, err := cookiejar.New(nil)
			Expect(err).To(BeNil())

			client := &http.Client{
				Jar: cookieJar,
			}

			resp, err := client.Get(frontend.URL + frontSidePath + "/set-cookies")
			Expect(err).To(BeNil())

			Expect(len(resp.Cookies())).To(Equal(1))

			cookie := resp.Cookies()[0]
			Expect(cookie.Path).To(Equal(frontSidePath))

			resp, err = client.Get(frontend.URL + frontSidePath + "/cookies")
			Expect(err).To(BeNil())
			Expect(resp.Body).ToNot(BeNil())

			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).To(BeNil())

			bodyString := string(bodyBytes)
			Expect(bodyString).To(ContainSubstring("value"))
		})
	})
})
