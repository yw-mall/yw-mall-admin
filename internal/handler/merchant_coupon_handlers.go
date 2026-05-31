package handler

import (
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"

	"mall-admin-api/internal/logic"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"
)

// createMerchantCouponTemplateHandler POST /merchant/v1/coupon-templates
func createMerchantCouponTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateCouponTemplateReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantCreateCouponTemplate(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// listMerchantCouponTemplatesHandler GET /merchant/v1/coupon-templates
func listMerchantCouponTemplatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListCouponTemplatesReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantListCouponTemplates(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// changeMerchantCouponTemplateStatusHandler PUT /merchant/v1/coupon-templates/:id/status
func changeMerchantCouponTemplateStatusHandler(svcCtx *svc.ServiceContext, status int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
		resp, err := logic.MerchantChangeCouponTemplateStatus(r.Context(), svcCtx, id, status)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
