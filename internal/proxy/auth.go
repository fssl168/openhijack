package proxy

import "strings"

type ProxyAuth struct {
	AuthKey string
}

func NewProxyAuth(key string) *ProxyAuth {
	return &ProxyAuth{AuthKey: key}
}

func (a *ProxyAuth) Verify(authHeader string) bool {
	if a.AuthKey == "" {
		return true
	}
	if authHeader == "" {
		return false
	}
	provided := strings.TrimPrefix(authHeader, "Bearer ")
	provided = strings.TrimPrefix(provided, "bearer ")
	return provided == a.AuthKey
}
