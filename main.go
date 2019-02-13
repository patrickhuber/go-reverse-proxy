package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
)

const (
	DefaultPort = "8080"
)

func main() {
	var (
		forwardedURL         string
		requestBodyFind      string
		requestBodyReplace   string
		processRequestBodies bool
		port                 string
	)

	if port = os.Getenv("PORT"); len(port) == 0 {
		port = DefaultPort
	}

	if forwardedURL = os.Getenv("FORWARDED_URL"); len(forwardedURL) == 0 {
		log.Fatal("missing required FORWARDED_URL environment variable")
		os.Exit(1)
	}

	url, err := url.Parse(forwardedURL)
	if err != nil {
		log.Fatal("FORWARDED_URL environment variable must be a valid url")
		log.Fatal(err)
		os.Exit(1)
	}

	processRequestBodies = true
	if requestBodyFind = os.Getenv("REQUEST_BODY_FIND"); len(requestBodyFind) == 0 {
		processRequestBodies = false
	}
	if requestBodyReplace = os.Getenv("REQUEST_BODY_REPLACE"); len(requestBodyReplace) == 0 && processRequestBodies {
		log.Fatal("REQUEST_BODY_REPLACE is required if REQUEST_BODY_FIND is set")
		os.Exit(1)
	}

	director := func(req *http.Request) {
		req.URL = url
		req.Host = url.Host

		// Read the content
		if req.Body == nil {
			return
		}

		if !processRequestBodies {
			return
		}

		bodyBytes, _ := ioutil.ReadAll(req.Body)

		// Modify the body bytes
		bodyString := string(bodyBytes)
		re := regexp.MustCompile(requestBodyFind)
		bodyString = re.ReplaceAllString(bodyString, requestBodyReplace)
		bodyBytes = []byte(bodyString)

		// Restore the io.ReadCloser to its original state
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	proxy := &httputil.ReverseProxy{Director: director}

	log.Fatal(http.ListenAndServe(":"+port, proxy))
}
