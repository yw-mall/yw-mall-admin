package middleware

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const (
	ctxKeyClaims ctxKey = iota
)

// Claims is the per-request identity for both admin and merchant tokens.
// Filled by SessionAuthMiddleware from the SessionInfo returned by
// mall-user-rpc.ValidateSession, then consumed by oplog + handler logic.
type Claims struct {
	Uid    int64    `json:"uid"`
	Role   string   `json:"role"`    // "admin" | "merchant"
	ShopId int64    `json:"shop_id"` // 0 for admin
	Perms  []string `json:"perms"`
}

// extractBearer pulls the Bearer token from the Authorization header.
func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return h
}

// ClaimsFromContext returns the claims stored by the auth middleware.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ctxKeyClaims).(*Claims)
	return c, ok
}

// withClaims stores claims in the request context.
func withClaims(r *http.Request, c *Claims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyClaims, c))
}

// AccessTokenFromContext returns the bearer token used for the current request,
// so logout handlers can revoke without re-reading the header.
func AccessTokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyAccessToken).(string); ok {
		return v
	}
	return ""
}

type ctxAccessTokenKey int

const ctxKeyAccessToken ctxAccessTokenKey = 0

func withAccessToken(r *http.Request, tok string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyAccessToken, tok))
}
