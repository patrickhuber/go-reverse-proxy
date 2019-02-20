package middleware

import (
	"crypto/tls"
	"net/http"
)

func NewTransport(skipTLSValidation bool) http.RoundTripper {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSValidation},
	}
}
