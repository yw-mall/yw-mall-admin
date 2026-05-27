package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-shop-rpc/shopservice"
	"mall-user-rpc/userclient"
)

// requestIPKey lets the handler stash the client IP into ctx so the auth logic
// can pick it up without taking *http.Request as a parameter.
type requestIPKey struct{}

// WithIP wraps ctx with the supplied client IP.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, requestIPKey{}, ip)
}

// IPFromCtx returns the IP previously stashed via WithIP, or "" if absent.
func IPFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(requestIPKey{}).(string); ok {
		return v
	}
	return ""
}

// AdminLogin authenticates an admin and mints a Redis-backed opaque session.
//
// Sprint 4 layered checks (in order):
//   1. failed-login lockout (Redis counters; 30-min window)
//   2. password verify via mall-user-rpc
//   3. IP whitelist (empty = no restriction; mismatch = 401)
//   4. password expiry → password_expired:true (FE forces change-password)
//   5. MFA enabled → mfa_required:true + challengeToken (FE goes to /login/mfa)
//   6. otherwise → CreateSession + return full LoginResp
func AdminLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LoginReq) (*types.LoginResp, error) {
	ip := IPFromCtx(ctx)
	username := strings.TrimSpace(req.Username)

	// 1. lockout
	if err := CheckLoginLock(ctx, svcCtx, "admin", username, ip); err != nil {
		return nil, err
	}

	// 2. password
	resp, err := svcCtx.UserRpc.AdminLogin(ctx, &userclient.AdminLoginReq{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		MarkLoginFail(ctx, svcCtx, "admin", username, ip)
		return nil, err
	}
	perms := splitPerms(resp.Permissions)

	// 3. IP whitelist
	if err := EnforceIpWhitelist(ctx, svcCtx, resp.Id, ip); err != nil {
		// Don't INCR fail counter here — the password was correct, and we don't
		// want a wrong network → permanent lockout.
		return nil, err
	}

	// success → clear lockout counters
	ClearLoginFail(ctx, svcCtx, "admin", username, ip)

	// 4. password expiry: gateway returns expired flag without minting a session
	if resp.PasswordExpired {
		return &types.LoginResp{
			Id:              resp.Id,
			Username:        resp.Username,
			Role:            "admin",
			Permissions:     perms,
			PasswordExpired: true,
		}, nil
	}

	// 5. MFA required: stash perms+identity, return challenge token
	if resp.MfaRequired {
		tok, err := stashChallenge(ctx, svcCtx, mfaChallengePayload{
			AdminId:  resp.Id,
			Username: resp.Username,
			Role:     "admin",
			Perms:    perms,
			Ip:       ip,
		})
		if err != nil {
			return nil, err
		}
		return &types.LoginResp{
			Id:             resp.Id,
			Username:       resp.Username,
			Role:           "admin",
			Permissions:    perms,
			MfaRequired:    true,
			ChallengeToken: tok,
		}, nil
	}

	// 6. happy path
	sess, err := svcCtx.UserRpc.CreateSession(ctx, &userclient.CreateSessionReq{
		Uid:      resp.Id,
		Username: resp.Username,
		Role:     "admin",
		ShopId:   0,
		Perms:    perms,
		Ip:       ip,
	})
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{
		Token:        sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		ExpiresIn:    sess.ExpiresIn,
		CsrfToken:    sess.CsrfToken,
		Id:           resp.Id,
		Username:     resp.Username,
		Role:         "admin",
		Permissions:  perms,
	}, nil
}

// MerchantLogin reuses the regular user login then looks the caller up in the
// merchant_staff table (M1 — adds staff role + perms; previously only the
// shop owner could log in via GetShopByOwnerId). Mints an opaque session bound
// to role="merchant" with shopId + perms injected so SessionAuthMiddleware
// can read perms straight out of session.Perms without an extra RPC roundtrip.
func MerchantLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LoginReq) (*types.LoginResp, error) {
	loginResp, err := svcCtx.UserRpc.Login(ctx, &userclient.LoginReq{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	sr, err := svcCtx.ShopRpc.GetStaffByUserId(ctx, &shopservice.GetStaffByUserIdReq{
		UserId: loginResp.Id,
	})
	if err != nil {
		return nil, errors.New("staff lookup failed")
	}
	if !sr.Found {
		return nil, errors.New("user is not a staff of any shop; apply for one or accept an invitation first")
	}
	sess, err := svcCtx.UserRpc.CreateSession(ctx, &userclient.CreateSessionReq{
		Uid:      loginResp.Id,
		Username: req.Username,
		Role:     "merchant",
		ShopId:   sr.Staff.ShopId,
		Perms:    sr.Perms,
	})
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{
		Token:        sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		ExpiresIn:    sess.ExpiresIn,
		CsrfToken:    sess.CsrfToken,
		Id:           loginResp.Id,
		Username:     req.Username,
		Role:         "merchant",
		ShopId:       sr.Staff.ShopId,
		StaffRole:    sr.Staff.Role,
		Permissions:  sr.Perms,
	}, nil
}

// AdminRefresh / MerchantRefresh rotate the access/refresh pair for the
// matching role. The middleware can't run here — the access token is
// already expired — so we trust the refresh token and verify the role on
// the session returned by mall-user-rpc.
func AdminRefresh(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RefreshReq) (*types.RefreshResp, error) {
	return refreshSession(ctx, svcCtx, req, "admin")
}

func MerchantRefresh(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RefreshReq) (*types.RefreshResp, error) {
	return refreshSession(ctx, svcCtx, req, "merchant")
}

func refreshSession(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RefreshReq, requiredRole string) (*types.RefreshResp, error) {
	if req.RefreshToken == "" {
		return nil, errors.New("missing refresh token")
	}
	sess, err := svcCtx.UserRpc.RefreshSession(ctx, &userclient.RefreshSessionReq{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		return nil, err
	}
	if sess.Role != requiredRole {
		return nil, errors.New("role mismatch")
	}
	return &types.RefreshResp{
		Token:        sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		ExpiresIn:    sess.ExpiresIn,
		CsrfToken:    sess.CsrfToken,
	}, nil
}

// AdminLogout / MerchantLogout revoke the bearer token of the current request.
// Idempotent on the server side, so a double-click on logout is fine.
func AdminLogout(ctx context.Context, svcCtx *svc.ServiceContext) (*types.LogoutResp, error) {
	return destroySessionFromCtx(ctx, svcCtx)
}

func MerchantLogout(ctx context.Context, svcCtx *svc.ServiceContext) (*types.LogoutResp, error) {
	return destroySessionFromCtx(ctx, svcCtx)
}

func destroySessionFromCtx(ctx context.Context, svcCtx *svc.ServiceContext) (*types.LogoutResp, error) {
	tok := middleware.AccessTokenFromContext(ctx)
	if tok != "" {
		_, _ = svcCtx.UserRpc.DestroySession(ctx, &userclient.DestroySessionReq{AccessToken: tok})
	}
	return &types.LogoutResp{Ok: true}, nil
}

func CreateAdmin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateAdminReq) (*types.CreateAdminResp, error) {
	resp, err := svcCtx.UserRpc.CreateAdmin(ctx, &userclient.CreateAdminReq{
		Username:    req.Username,
		Password:    req.Password,
		Email:       req.Email,
		Role:        req.Role,
		Permissions: req.Permissions,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreateAdminResp{Id: resp.Id}, nil
}

func ListAdmins(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListAdminsReq) (*types.ListAdminsResp, error) {
	resp, err := svcCtx.UserRpc.ListAdmins(ctx, &userclient.ListAdminsReq{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*types.AdminInfo, 0, len(resp.Admins))
	for _, a := range resp.Admins {
		out = append(out, &types.AdminInfo{
			Id:          a.Id,
			Username:    a.Username,
			Email:       a.Email,
			Role:        a.Role,
			Permissions: a.Permissions,
			Status:      a.Status,
			CreateTime:  a.CreateTime,
		})
	}
	return &types.ListAdminsResp{Total: resp.Total, Admins: out}, nil
}

// splitPerms parses the admin's permissions column. The DB schema stores it
// as a JSON string array ('["*"]' or '["product.read","order.write"]'). Older
// rows or CLI seeding tools may write plain CSV ("a,b,c"), so we accept both:
//
//   - leading '[' → JSON array decode
//   - otherwise   → comma-split fallback
func splitPerms(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "[") {
		var out []string
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			cleaned := make([]string, 0, len(out))
			for _, p := range out {
				p = strings.TrimSpace(p)
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			return cleaned
		}
		// fall through to CSV path on JSON parse error
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
