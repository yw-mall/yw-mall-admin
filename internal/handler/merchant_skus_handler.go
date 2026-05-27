package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"mall-admin-api/internal/logic"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"
)

// merchantBatchUpsertSkusHandler POST /merchant/v1/products/:id/skus
// 全量上送 SKU 数组，product-rpc 端在单事务里 upsert + 聚合回写 product 行。
func merchantBatchUpsertSkusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		var req types.BatchUpsertSkusReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantBatchUpsertSkus(r.Context(), svcCtx, id, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
