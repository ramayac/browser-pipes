package main

import (
	"crypto/sha256"
	"fmt"
	"net/url"
)

func parseURL(uri string) *url.URL {
	u, _ := url.Parse(uri)
	return u
}

func hashURL(uri string) string {
	h := sha256.New()
	h.Write([]byte(uri))
	return fmt.Sprintf("%x", h.Sum(nil))[:8]
}
