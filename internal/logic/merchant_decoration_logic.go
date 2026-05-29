package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-shop-rpc/shopservice"
)

func GetMerchantDecoration(ctx context.Context, svcCtx *svc.ServiceContext) (*types.ShopDecorationDTO, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	r, err := svcCtx.ShopRpc.GetShopDecoration(ctx, &shopservice.GetShopDecorationReq{ShopId: c.ShopId})
	if err != nil {
		return nil, err
	}
	return &types.ShopDecorationDTO{
		ShopId:       r.ShopId,
		Banners:      r.Banners,
		Announcement: r.Announcement,
		FeaturedPids: r.FeaturedPids,
		UpdateTime:   r.UpdateTime,
	}, nil
}

func UpdateMerchantDecoration(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdateDecorationReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if !hasPerm(c, "decoration.write") {
		return nil, errors.New("permission denied: decoration.write")
	}
	if _, err := svcCtx.ShopRpc.UpdateShopDecoration(ctx, &shopservice.UpdateShopDecorationReq{
		ShopId:       c.ShopId,
		Banners:      req.Banners,
		Announcement: req.Announcement,
		FeaturedPids: req.FeaturedPids,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
