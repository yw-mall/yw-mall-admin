package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"mall-admin-api/internal/logic"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"
)

// merchantGetRefundDetailHandler GET /merchant/v1/refunds/:id
func merchantGetRefundDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantGetRefundDetail(r.Context(), svcCtx, id)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// merchantInspectReturnHandler POST /merchant/v1/refunds/:id/inspect
func merchantInspectReturnHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		var req types.InspectReturnReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantInspectReturn(r.Context(), svcCtx, id, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// merchantShipExchangeHandler POST /merchant/v1/refunds/:id/ship-exchange
func merchantShipExchangeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		var req types.ShipExchangeReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantShipExchange(r.Context(), svcCtx, id, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
