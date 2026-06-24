package kap

import (
	"net/http"

	"hissebot/pkg/useragent"
)

func setKAPRequestHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", useragent.RandomUserAgent())
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
	}
}
