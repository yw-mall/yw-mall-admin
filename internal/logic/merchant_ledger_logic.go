package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-payment-rpc/paymentclient"
)

// MerchantListLedger 商家版收支流水：强制注入本店 shopId 防越权，
// 客户端传的 ShopId 被忽略（admin 版 ListLedger 允许跨店查，本接口
// 只允许查自己店的）。
func MerchantListLedger(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListLedgerReq) (*types.ListLedgerResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	resp, err := svcCtx.PaymentRpc.ListLedger(ctx, &paymentclient.ListLedgerReq{
		ShopId:    c.ShopId, // 强制本店
		Category:  req.Category,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	entries := make([]types.LedgerEntryDTO, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entries = append(entries, types.LedgerEntryDTO{
			Id:             e.Id,
			ShopId:         e.ShopId,
			Direction:      e.Direction,
			Category:       e.Category,
			Amount:         e.Amount,
			RunningBalance: e.RunningBalance,
			OrderId:        e.OrderId,
			RefundId:       e.RefundId,
			RefNo:          e.RefNo,
			Description:    e.Description,
			CreateTime:     e.CreateTime,
		})
	}
	return &types.ListLedgerResp{Total: resp.Total, Entries: entries}, nil
}

// MerchantGetLedgerSummary 同款防越权处理。
func MerchantGetLedgerSummary(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetLedgerSummaryReq) (*types.LedgerSummaryDTO, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	resp, err := svcCtx.PaymentRpc.GetShopLedgerSummary(ctx, &paymentclient.GetShopLedgerSummaryReq{
		ShopId:    c.ShopId, // 强制本店
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if err != nil {
		return nil, err
	}
	return &types.LedgerSummaryDTO{
		TotalIncome:     resp.TotalIncome,
		TotalRefund:     resp.TotalRefund,
		TotalCommission: resp.TotalCommission,
		TotalWithdrawal: resp.TotalWithdrawal,
	}, nil
}
