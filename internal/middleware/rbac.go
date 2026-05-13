package middleware

import (
	"net/http"

	"mall-user-rpc/userclient"
	userpb "mall-user-rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// SessionAuthMiddleware enforces opaque-token session auth via mall-user-rpc
// and the per-route required role. It replaces the legacy JWT middleware:
// the bearer token is now a Redis-backed opaque blob, validated by an RPC
// roundtrip on every request (with mall-user-rpc handling sliding-window TTL).
type SessionAuthMiddleware struct {
	userRpc      userclient.User
	requiredRole string
}

func NewSessionAuthMiddleware(userRpc userclient.User, requiredRole string) *SessionAuthMiddleware {
	return &SessionAuthMiddleware{userRpc: userRpc, requiredRole: requiredRole}
}

func (m *SessionAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := extractBearer(r)
		if tok == "" {
			httpx.WriteJson(w, http.StatusUnauthorized, map[string]any{"code": 1003, "msg": "missing token"})
			return
		}
		sess, err := m.userRpc.ValidateSession(r.Context(), &userpb.ValidateSessionReq{AccessToken: tok})
		if err != nil || sess == nil || sess.Uid <= 0 {
			if err != nil {
				logx.WithContext(r.Context()).Errorf("ValidateSession failed: %v", err)
			}
			httpx.WriteJson(w, http.StatusUnauthorized, map[string]any{"code": 1003, "msg": "invalid token"})
			return
		}
		if sess.Role != m.requiredRole {
			httpx.WriteJson(w, http.StatusForbidden, map[string]any{"code": 1003, "msg": "forbidden"})
			return
		}
		claims := &Claims{
			Uid:    sess.Uid,
			Role:   sess.Role,
			ShopId: sess.ShopId,
			Perms:  sess.Perms,
		}
		next(w, withAccessToken(withClaims(r, claims), tok))
	}
}
