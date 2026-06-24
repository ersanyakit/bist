package useragent

import "math/rand"

func RandomUserAgent() string {
	if len(UserAgents) == 0 {
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	}
	return UserAgents[rand.Intn(len(UserAgents))]
}
