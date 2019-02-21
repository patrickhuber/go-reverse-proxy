package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/patrickhuber/go-reverse-proxy/proxies"

	"github.com/urfave/cli"
)

const (
	DefaultPort = "8080"
)

func main() {
	app := cli.App{
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "port, p",
				EnvVar: "PORT",
				Value:  DefaultPort,
			},
			cli.StringFlag{
				Name:   "forwarded-url, f",
				EnvVar: "FORWARDED_URL",
			},
			cli.StringFlag{
				Name:   "path-prefix, x",
				EnvVar: "PATH_PREFIX",
			},
			cli.BoolFlag{
				Name:   "skip-ssl-validation, k",
				EnvVar: "SKIP_SSL_VALIDATION",
			},
			cli.StringFlag{
				Name:   "x-forwarded-host-header",
				EnvVar: "X_FORWARDED_HOST_HEADER",
			},
			cli.StringFlag{
				Name:   "x-forwarded-path-header",
				EnvVar: "X_FORWARDED_PATH_HEADER",
			},
		},
		Action: func(c *cli.Context) error {
			port := c.String("port")
			forwardedURL := c.String("forwarded-url")
			skipTLSValidation := c.Bool("skip-ssl-validation")
			xForwardedHostHeader := c.String("x-forwarded-host-header")
			xForwardedPathHeader := c.String("x-forwarded-path-header")
			pathPrefix := c.String("path-prefix")

			if strings.TrimSpace(forwardedURL) == "" {
				return fmt.Errorf("forwarded-url argument is required")
			}

			url, err := url.Parse(forwardedURL)
			if err != nil {
				return err
			}

			reverseProxy := proxies.NewReverseProxyBuilder().
				RewriteHost(url, pathPrefix).
				CopyRequestHeaderIf(xForwardedHostHeader, "X-Forwarded-Host", func(r *http.Request) bool {
					return strings.TrimSpace(xForwardedHostHeader) != ""
				}).
				CopyRequestHeaderIf(xForwardedPathHeader, "X-Forwarded-Path", func(r *http.Request) bool {
					return strings.TrimSpace(xForwardedPathHeader) != ""
				}).
				RewriteRequestBody(url, pathPrefix).
				RewriteRedirect(url, pathPrefix).
				RewriteResponseBody(url, pathPrefix).
				RewriteResponseCookies(url, pathPrefix).
				ToReverseProxy(&http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSValidation},
				})

			return http.ListenAndServe(":"+port, reverseProxy)
		},
	}

	// set the output so cf doesn't throw errors during logging
	log.SetOutput(os.Stdout)

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	os.Exit(0)
}
