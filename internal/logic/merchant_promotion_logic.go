// Phase 1 优惠活动管理 —— 商家后台 CRUD 透传到 promotion-rpc。
// 强制 c.ShopId 防越权，所有 logic 进入前先校验 merchant 已登录且属于某个 shop。
package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-promotion-rpc/promotionclient"
)

func MerchantCreatePromotion(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePromotionReq) (*types.CreatePromotionResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	targets := make([]*promotionclient.ActivityTarget, 0, len(req.Targets))
	for _, t := range req.Targets {
		targets = append(targets, &promotionclient.ActivityTarget{
			TargetType: t.TargetType,
			TargetId:   t.TargetId,
		})
	}
	actions := make([]*promotionclient.ActivityAction, 0, len(req.Actions))
	for _, a := range req.Actions {
		actions = append(actions, &promotionclient.ActivityAction{
			ActionType:     a.ActionType,
			ThresholdType:  a.ThresholdType,
			ThresholdValue: a.ThresholdValue,
			BenefitValue:   a.BenefitValue,
			MaxDiscount:    a.MaxDiscount,
			GiftSkuId:      a.GiftSkuId,
			StepOrder:      a.StepOrder,
		})
	}
	resp, err := svcCtx.PromotionRpc.CreateActivity(ctx, &promotionclient.CreateActivityReq{
		Type:          req.Type,
		Name:          req.Name,
		ShopId:        c.ShopId, // 强制本店
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		Priority:      req.Priority,
		Stackable:     req.Stackable,
		Description:   req.Description,
		CreateUserId:  c.Uid,
		Targets:       targets,
		Actions:       actions,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreatePromotionResp{Id: resp.Id}, nil
}

func MerchantUpdatePromotion(ctx context.Context, svcCtx *svc.ServiceContext, id int64, req *types.UpdatePromotionReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	// 防越权：先读一遍校验 shop_id
	got, err := svcCtx.PromotionRpc.GetActivity(ctx, &promotionclient.GetActivityReq{Id: id})
	if err != nil {
		return nil, err
	}
	if got.Activity.ShopId != c.ShopId {
		return nil, errors.New("activity does not belong to your shop")
	}

	targets := make([]*promotionclient.ActivityTarget, 0, len(req.Targets))
	for _, t := range req.Targets {
		targets = append(targets, &promotionclient.ActivityTarget{TargetType: t.TargetType, TargetId: t.TargetId})
	}
	actions := make([]*promotionclient.ActivityAction, 0, len(req.Actions))
	for _, a := range req.Actions {
		actions = append(actions, &promotionclient.ActivityAction{
			ActionType: a.ActionType, ThresholdType: a.ThresholdType,
			ThresholdValue: a.ThresholdValue, BenefitValue: a.BenefitValue,
			MaxDiscount: a.MaxDiscount, GiftSkuId: a.GiftSkuId, StepOrder: a.StepOrder,
		})
	}
	if _, err := svcCtx.PromotionRpc.UpdateActivity(ctx, &promotionclient.UpdateActivityReq{
		Id: id, Name: req.Name, StartTime: req.StartTime, EndTime: req.EndTime,
		Priority: req.Priority, Stackable: req.Stackable, Description: req.Description,
		Targets: targets, Actions: actions,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

func MerchantGetPromotion(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PromotionInfo, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	resp, err := svcCtx.PromotionRpc.GetActivity(ctx, &promotionclient.GetActivityReq{Id: id})
	if err != nil {
		return nil, err
	}
	if resp.Activity.ShopId != c.ShopId {
		return nil, errors.New("activity does not belong to your shop")
	}
	return promotionToInfo(resp.Activity), nil
}

func MerchantListPromotions(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPromotionsReq) (*types.ListPromotionsResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	resp, err := svcCtx.PromotionRpc.ListActivities(ctx, &promotionclient.ListActivitiesReq{
		ShopId: c.ShopId, // 强制本店
		Type:   req.Type,
		Status: req.Status,
		Page:   req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*types.PromotionInfo, 0, len(resp.Activities))
	for _, a := range resp.Activities {
		out = append(out, promotionToInfo(a))
	}
	return &types.ListPromotionsResp{Total: resp.Total, Promotions: out}, nil
}

func MerchantChangePromotionStatus(ctx context.Context, svcCtx *svc.ServiceContext, id int64, status int32) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	// 防越权
	got, err := svcCtx.PromotionRpc.GetActivity(ctx, &promotionclient.GetActivityReq{Id: id})
	if err != nil {
		return nil, err
	}
	if got.Activity.ShopId != c.ShopId {
		return nil, errors.New("activity does not belong to your shop")
	}
	if _, err := svcCtx.PromotionRpc.ChangeActivityStatus(ctx, &promotionclient.ChangeActivityStatusReq{
		Id: id, Status: status,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

func promotionToInfo(a *promotionclient.Activity) *types.PromotionInfo {
	targets := make([]types.PromotionTargetDTO, 0, len(a.Targets))
	for _, t := range a.Targets {
		targets = append(targets, types.PromotionTargetDTO{TargetType: t.TargetType, TargetId: t.TargetId})
	}
	actions := make([]types.PromotionActionDTO, 0, len(a.Actions))
	for _, ac := range a.Actions {
		actions = append(actions, types.PromotionActionDTO{
			ActionType: ac.ActionType, ThresholdType: ac.ThresholdType,
			ThresholdValue: ac.ThresholdValue, BenefitValue: ac.BenefitValue,
			MaxDiscount: ac.MaxDiscount, GiftSkuId: ac.GiftSkuId, StepOrder: ac.StepOrder,
		})
	}
	return &types.PromotionInfo{
		Id: a.Id, Type: a.Type, Name: a.Name, ShopId: a.ShopId, Status: a.Status,
		StartTime: a.StartTime, EndTime: a.EndTime, Priority: a.Priority, Stackable: a.Stackable,
		Description: a.Description, CreateUserId: a.CreateUserId,
		CreateTime: a.CreateTime, UpdateTime: a.UpdateTime,
		Targets: targets, Actions: actions,
	}
}
