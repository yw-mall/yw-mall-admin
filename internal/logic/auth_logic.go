package logic

import (
	"context"
	"errors"
	"strings"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-shop-rpc/shopservice"
	"mall-user-rpc/userclient"
)

// AdminLogin authenticates an admin and mints a Redis-backed opaque session.
func AdminLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LoginReq) (*types.LoginResp, error) {
	resp, err := svcCtx.UserRpc.AdminLogin(ctx, &userclient.AdminLoginReq{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	perms := splitPerms(resp.Permissions)
	sess, err := svcCtx.UserRpc.CreateSession(ctx, &userclient.CreateSessionReq{
		Uid:      resp.Id,
		Username: resp.Username,
		Role:     "admin",
		ShopId:   0,
		Perms:    perms,
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

// MerchantLogin reuses the regular user login then attaches shop_id from the
// shop service before minting an opaque session bound to role=merchant.
func MerchantLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LoginReq) (*types.LoginResp, error) {
	loginResp, err := svcCtx.UserRpc.Login(ctx, &userclient.LoginReq{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	shop, err := svcCtx.ShopRpc.GetShopByOwnerId(ctx, &shopservice.GetShopByOwnerIdReq{OwnerUserId: loginResp.Id})
	if err != nil {
		return nil, errors.New("merchant has no active shop; apply for one first")
	}
	sess, err := svcCtx.UserRpc.CreateSession(ctx, &userclient.CreateSessionReq{
		Uid:      loginResp.Id,
		Username: req.Username,
		Role:     "merchant",
		ShopId:   shop.Id,
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
		ShopId:       shop.Id,
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

func splitPerms(s string) []string {
	if s == "" {
		return nil
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
