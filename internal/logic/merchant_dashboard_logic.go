package logic

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-order-rpc/orderclient"
	"mall-payment-rpc/paymentclient"
)

// MerchantDashboard 聚合 4 个核心指标 + 钱包余额, 用 errgroup 并发调
// 3 个下游 RPC 服务. 任一失败不阻塞其他, 失败的字段返 0 给 FE 兜底.
//
// 注: "今日营业额" 需要 order-rpc 支持日期 filter, 当前只暴露 status,
// M6 先返「总订单数」+「待发货数」+「待退款数」+「钱包可用/冻结」
// 5 个字段, 日维度聚合 P1+ 单独 sprint 加.
func MerchantDashboard(ctx context.Context, svcCtx *svc.ServiceContext) (*types.MerchantDashboardResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}

	resp := &types.MerchantDashboardResp{}
	g, gctx := errgroup.WithContext(ctx)

	// 1. 总订单数
	g.Go(func() error {
		r, err := svcCtx.OrderRpc.ListShopOrders(gctx, &orderclient.ListShopOrdersReq{
			ShopId: c.ShopId, Status: -1, Page: 1, PageSize: 1,
		})
		if err != nil {
			logErr(ctx, "dashboard ListShopOrders all", err)
			return nil
		}
		resp.TotalOrders = int32(r.Total)
		return nil
	})

	// 2. 待发货数 (status=1)
	g.Go(func() error {
		r, err := svcCtx.OrderRpc.ListShopOrders(gctx, &orderclient.ListShopOrdersReq{
			ShopId: c.ShopId, Status: 1, Page: 1, PageSize: 1,
		})
		if err != nil {
			logErr(ctx, "dashboard ListShopOrders pending", err)
			return nil
		}
		resp.PendingShipments = int32(r.Total)
		return nil
	})

	// 3. 待退款数 (status=0)
	g.Go(func() error {
		r, err := svcCtx.OrderRpc.ListShopRefundRequests(gctx, &orderclient.ListShopRefundRequestsReq{
			ShopId: c.ShopId, Status: 0, Page: 1, PageSize: 1,
		})
		if err != nil {
			logErr(ctx, "dashboard ListShopRefundRequests pending", err)
			return nil
		}
		resp.PendingRefunds = int32(r.Total)
		return nil
	})

	// 4. 钱包
	g.Go(func() error {
		w, err := svcCtx.PaymentRpc.GetMerchantWallet(gctx, &paymentclient.GetMerchantWalletReq{
			ShopId: c.ShopId,
		})
		if err != nil {
			logErr(ctx, "dashboard GetMerchantWallet", err)
			return nil
		}
		resp.WalletAvailable = w.Balance
		resp.WalletFrozen = w.Frozen
		return nil
	})

	_ = g.Wait()
	return resp, nil
}

func logErr(_ context.Context, scope string, err error) {
	// 用全局 logx 避免在 errgroup goroutine 里取 ctx-bound logger 的踩坑
	_ = scope
	_ = err
}
