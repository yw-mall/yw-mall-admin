package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-shop-rpc/shopservice"
	"mall-user-rpc/userclient"
)

// hasPerm 检查 claims.Perms 是否含 perm 或 "*"（owner）
func hasPerm(c *middleware.Claims, perm string) bool {
	for _, p := range c.Perms {
		if p == "*" || p == perm {
			return true
		}
	}
	return false
}

// ===== Staff CRUD =====

func ListMerchantStaff(ctx context.Context, svcCtx *svc.ServiceContext) (*types.ListStaffResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	res, err := svcCtx.ShopRpc.ListShopStaff(ctx, &shopservice.ListShopStaffReq{ShopId: c.ShopId})
	if err != nil {
		return nil, err
	}
	out := make([]types.StaffItemDTO, 0, len(res.Items))
	for _, s := range res.Items {
		out = append(out, types.StaffItemDTO{
			Id: s.Id, UserId: s.UserId, Username: s.Username,
			Role: s.Role, Status: s.Status, JoinedAt: s.JoinedAt,
		})
	}
	return &types.ListStaffResp{Items: out}, nil
}

func UpdateMerchantStaffRole(ctx context.Context, svcCtx *svc.ServiceContext, staffId int64, req *types.UpdateStaffRoleReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if !hasPerm(c, "staff.write") {
		return nil, errors.New("permission denied: staff.write")
	}
	if _, err := svcCtx.ShopRpc.UpdateStaffRole(ctx, &shopservice.UpdateStaffRoleReq{
		ShopId: c.ShopId, StaffId: staffId, NewRole: req.NewRole,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

func DisableMerchantStaff(ctx context.Context, svcCtx *svc.ServiceContext, staffId int64) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if !hasPerm(c, "staff.write") {
		return nil, errors.New("permission denied: staff.write")
	}
	if _, err := svcCtx.ShopRpc.DisableStaff(ctx, &shopservice.DisableStaffReq{
		ShopId: c.ShopId, StaffId: staffId,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

// ===== Invitation =====

func CreateMerchantInvitation(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateInvitationReq) (*types.CreateInvitationResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if !hasPerm(c, "staff.write") {
		return nil, errors.New("permission denied: staff.write")
	}
	res, err := svcCtx.ShopRpc.CreateInvitation(ctx, &shopservice.CreateInvitationReq{
		ShopId: c.ShopId, InvitedByUid: c.Uid,
		TargetPhone: req.TargetPhone, TargetEmail: req.TargetEmail, Role: req.Role,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreateInvitationResp{InvitationCode: res.InvitationCode, ExpiresAt: res.ExpiresAt}, nil
}

func ListMerchantInvitations(ctx context.Context, svcCtx *svc.ServiceContext) (*types.ListInvitationsResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	res, err := svcCtx.ShopRpc.ListPendingInvitations(ctx, &shopservice.ListPendingInvitationsReq{ShopId: c.ShopId})
	if err != nil {
		return nil, err
	}
	out := make([]types.InvitationItemDTO, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, types.InvitationItemDTO{
			Id: it.Id, TargetPhone: it.TargetPhone, TargetEmail: it.TargetEmail,
			Role: it.Role, Status: it.Status, ExpiresAt: it.ExpiresAt, CreateTime: it.CreateTime,
		})
	}
	return &types.ListInvitationsResp{Items: out}, nil
}

func RevokeMerchantInvitation(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if !hasPerm(c, "staff.write") {
		return nil, errors.New("permission denied: staff.write")
	}
	if _, err := svcCtx.ShopRpc.RevokeInvitation(ctx, &shopservice.RevokeInvitationReq{
		ShopId: c.ShopId, InvitationId: id,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

// AcceptMerchantInvitation 接受邀请 — 调用方必须已有 c-user (role=user) session。
// 本端点放在 public 路由，自己用 ValidateSession 拿 uid + phone/email
// 而不依赖 MerchantAuth (那个要求 role=merchant)。
func AcceptMerchantInvitation(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AcceptInvitationReq, accessToken string) (*types.AcceptInvitationResp, error) {
	if accessToken == "" {
		return nil, errors.New("login required")
	}
	sess, err := svcCtx.UserRpc.ValidateSession(ctx, &userclient.ValidateSessionReq{AccessToken: accessToken})
	if err != nil || sess == nil || sess.Uid <= 0 {
		return nil, errors.New("invalid session")
	}
	// 允许 c-user (普通用户) 和 merchant (已是其他店员工) 接受邀请
	if sess.Role != "user" && sess.Role != "merchant" {
		return nil, errors.New("forbidden")
	}
	// shop-rpc AcceptInvitation 内部会查 user.phone/email 做防代领校验,
	// admin-api 不需要再多一次 RPC roundtrip.
	res, err := svcCtx.ShopRpc.AcceptInvitation(ctx, &shopservice.AcceptInvitationReq{
		InvitationCode: req.InvitationCode,
		AcceptorUid:    sess.Uid,
	})
	if err != nil {
		return nil, err
	}
	return &types.AcceptInvitationResp{ShopId: res.ShopId, Role: res.Role, ShopName: res.ShopName}, nil
}
