package logic

import (
	"context"
	"errors"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-product-rpc/productclient"
)

// MerchantBatchUpsertSkus 透传到 product-rpc BatchUpsertSkus。
// 需 product.write 权限 (warehouse + owner 可调).
func MerchantBatchUpsertSkus(ctx context.Context, svcCtx *svc.ServiceContext, productId int64, req *types.BatchUpsertSkusReq) (*types.BatchUpsertSkusResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if !hasPerm(c, "product.write") {
		return nil, errors.New("permission denied: product.write")
	}
	skus := make([]*productclient.SkuInput, 0, len(req.Skus))
	for _, s := range req.Skus {
		skus = append(skus, &productclient.SkuInput{
			Id:       s.Id,
			SkuCode:  s.SkuCode,
			SpecText: s.SpecText,
			SpecJson: s.SpecJson,
			Price:    s.Price,
			Stock:    s.Stock,
			Image:    s.Image,
			Status:   s.Status,
		})
	}
	r, err := svcCtx.ProductRpc.BatchUpsertSkus(ctx, &productclient.BatchUpsertSkusReq{
		ProductId: productId,
		ShopId:    c.ShopId,
		Skus:      skus,
	})
	if err != nil {
		return nil, err
	}
	out := make([]types.SkuItemDTO, 0, len(r.Items))
	for _, it := range r.Items {
		out = append(out, types.SkuItemDTO{
			Id:        it.Id,
			ProductId: it.ProductId,
			SkuCode:   it.SkuCode,
			SpecText:  it.SpecText,
			SpecJson:  it.SpecJson,
			Price:     it.Price,
			Stock:     it.Stock,
			Image:     it.Image,
			Status:    it.Status,
		})
	}
	return &types.BatchUpsertSkusResp{Items: out}, nil
}
