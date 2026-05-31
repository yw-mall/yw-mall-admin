// 商家券模板管理 - admin-api 包装层
package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-promotion-rpc/promotionclient"
)

func MerchantCreateCouponTemplate(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreateCouponTemplateReq) (*types.CreateCouponTemplateResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	resp, err := svcCtx.PromotionRpc.CreateCouponTemplate(ctx, &promotionclient.CreateCouponTemplateReq{
		ShopId:        c.ShopId,
		Name:          req.Name,
		Type:          req.Type,
		Value:         req.Value,
		MinAmount:     req.MinAmount,
		MaxDiscount:   req.MaxDiscount,
		CategoryId:    req.CategoryId,
		TotalCount:    req.TotalCount,
		PerUserLimit:  req.PerUserLimit,
		ValidType:     req.ValidType,
		ValidDays:     req.ValidDays,
		ValidStart:    req.ValidStart,
		ValidEnd:      req.ValidEnd,
		ReceiveStart:  req.ReceiveStart,
		ReceiveEnd:    req.ReceiveEnd,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreateCouponTemplateResp{Id: resp.Id}, nil
}

func MerchantListCouponTemplates(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListCouponTemplatesReq) (*types.ListCouponTemplatesResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	resp, err := svcCtx.PromotionRpc.ListCouponTemplates(ctx, &promotionclient.ListCouponTemplatesReq{
		ShopId: c.ShopId, Status: req.Status, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*types.CouponTemplateInfo, 0, len(resp.Templates))
	for _, t := range resp.Templates {
		out = append(out, couponTemplateToInfo(t))
	}
	return &types.ListCouponTemplatesResp{Total: resp.Total, Templates: out}, nil
}

func MerchantChangeCouponTemplateStatus(ctx context.Context, svcCtx *svc.ServiceContext, id int64, status int32) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	_, _ = c, id
	if _, err := svcCtx.PromotionRpc.ChangeCouponTemplateStatus(ctx, &promotionclient.ChangeCouponTemplateStatusReq{
		Id: id, Status: status,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

func couponTemplateToInfo(t *promotionclient.CouponTemplate) *types.CouponTemplateInfo {
	return &types.CouponTemplateInfo{
		Id: t.Id, ActivityId: t.ActivityId, ShopId: t.ShopId, Name: t.Name, Type: t.Type,
		Value: t.Value, MinAmount: t.MinAmount, MaxDiscount: t.MaxDiscount, CategoryId: t.CategoryId,
		TotalCount: t.TotalCount, ReceivedCount: t.ReceivedCount, UsedCount: t.UsedCount,
		PerUserLimit: t.PerUserLimit, ValidType: t.ValidType, ValidDays: t.ValidDays,
		ValidStart: t.ValidStart, ValidEnd: t.ValidEnd,
		ReceiveStart: t.ReceiveStart, ReceiveEnd: t.ReceiveEnd,
		Status: t.Status, CreateTime: t.CreateTime, UpdateTime: t.UpdateTime,
	}
}
