package main

import (
	"net/url"
)

func parseURL(uri string) *url.URL {
	u, _ := url.Parse(uri)
	return u
}
