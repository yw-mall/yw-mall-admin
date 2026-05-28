package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-order-rpc/orderclient"
)

func refundProtoToInfo(r *orderclient.RefundRequest) *types.RefundInfo {
	if r == nil {
		return nil
	}
	items := make([]types.RefundItemDTO, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, types.RefundItemDTO{
			SkuId:    it.SkuId,
			SkuName:  it.SkuName,
			Quantity: it.Quantity,
			Amount:   it.Amount,
		})
	}
	return &types.RefundInfo{
		Id:                 r.Id,
		OrderId:            r.OrderId,
		OrderNo:            r.OrderNo,
		UserId:             r.UserId,
		ShopId:             r.ShopId,
		Amount:             r.Amount,
		Reason:             r.Reason,
		Evidence:           append([]string{}, r.Evidence...),
		Items:              items,
		Status:             r.Status,
		MerchantUserId:     r.MerchantUserId,
		MerchantRemark:     r.MerchantRemark,
		MerchantHandleTime: r.MerchantHandleTime,
		AdminId:            r.AdminId,
		AdminRemark:        r.AdminRemark,
		AdminHandleTime:    r.AdminHandleTime,
		AppealReason:       r.AppealReason,
		AppealTime:         r.AppealTime,
		RefundNo:           r.RefundNo,
		RefundCompleteTime: r.RefundCompleteTime,
		CreateTime:         r.CreateTime,
	}
}

// ListPendingArbitrations returns refund_requests in status=3 for admin review.
func ListPendingArbitrations(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListRefundsReq) (*types.ListRefundsResp, error) {
	resp, err := svcCtx.OrderRpc.ListPendingArbitrations(ctx, &orderclient.ListPendingArbitrationsReq{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*types.RefundInfo, 0, len(resp.Requests))
	for _, r := range resp.Requests {
		out = append(out, refundProtoToInfo(r))
	}
	return &types.ListRefundsResp{Total: resp.Total, Requests: out}, nil
}

func ArbitrateRefund(ctx context.Context, svcCtx *svc.ServiceContext, id int64, req *types.ArbitrateRefundReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.Uid <= 0 {
		return nil, errors.New("admin not authenticated")
	}
	if _, err := svcCtx.OrderRpc.AdminArbitrateRefund(ctx, &orderclient.AdminArbitrateRefundReq{
		RefundId: id,
		AdminId:  c.Uid,
		Action:   req.Action,
		Remark:   req.Remark,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

// ListShopRefunds returns refund_requests belonging to the authenticated
// merchant's shop. Status<0 means "all".
func ListShopRefunds(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListRefundsReq) (*types.ListRefundsResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("merchant shop unknown")
	}
	resp, err := svcCtx.OrderRpc.ListShopRefundRequests(ctx, &orderclient.ListShopRefundRequestsReq{
		ShopId:   c.ShopId,
		Status:   req.Status,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*types.RefundInfo, 0, len(resp.Requests))
	for _, r := range resp.Requests {
		out = append(out, refundProtoToInfo(r))
	}
	return &types.ListRefundsResp{Total: resp.Total, Requests: out}, nil
}

func MerchantHandleRefund(ctx context.Context, svcCtx *svc.ServiceContext, id int64, req *types.MerchantHandleRefundReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("merchant shop unknown")
	}
	if !hasPerm(c, "refund.handle") {
		return nil, errors.New("permission denied: refund.handle")
	}
	if _, err := svcCtx.OrderRpc.MerchantHandleRefund(ctx, &orderclient.MerchantHandleRefundReq{
		RefundId:       id,
		ShopId:         c.ShopId,
		MerchantUserId: c.Uid,
		Action:         req.Action,
		Remark:         req.Remark,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

// ===== M4 退款 3 类工作流 — detail + 退货验货 + 换货发货 =====

// MerchantGetRefundDetail 详情接口，校验退款单归属本店避免越权。
func MerchantGetRefundDetail(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.RefundInfo, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("merchant shop unknown")
	}
	r, err := svcCtx.OrderRpc.GetRefundRequest(ctx, &orderclient.GetRefundRequestReq{Id: id})
	if err != nil {
		return nil, err
	}
	if r.ShopId != c.ShopId {
		return nil, errors.New("refund not in this shop")
	}
	return refundProtoToInfo(r), nil
}

// MerchantInspectReturn 退货退款的「商家验货」阶段。
// 需 refund.inspect 权限（仓管角色可调）。
func MerchantInspectReturn(ctx context.Context, svcCtx *svc.ServiceContext, id int64, req *types.InspectReturnReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("merchant shop unknown")
	}
	if !hasPerm(c, "refund.inspect") {
		return nil, errors.New("permission denied: refund.inspect")
	}
	if _, err := svcCtx.OrderRpc.MerchantInspectReturn(ctx, &orderclient.MerchantInspectReturnReq{
		RefundId:       id,
		ShopId:         c.ShopId,
		MerchantUserId: c.Uid,
		Passed:         req.Passed,
		Remark:         req.Remark,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}

// MerchantShipExchange 换货工单的「商家寄换货」阶段。
// 需 order.ship 权限（与发货同款，仓管+店主可调）。
func MerchantShipExchange(ctx context.Context, svcCtx *svc.ServiceContext, id int64, req *types.ShipExchangeReq) (*types.OkResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("merchant shop unknown")
	}
	if !hasPerm(c, "order.ship") {
		return nil, errors.New("permission denied: order.ship")
	}
	if _, err := svcCtx.OrderRpc.MerchantShipExchange(ctx, &orderclient.MerchantShipExchangeReq{
		RefundId:       id,
		ShopId:         c.ShopId,
		MerchantUserId: c.Uid,
		TrackingNo:     req.TrackingNo,
		Carrier:        req.Carrier,
	}); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
