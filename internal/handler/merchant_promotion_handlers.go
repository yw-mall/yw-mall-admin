// Phase 1 优惠活动 HTTP 入口 —— /merchant/v1/promotions/*
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

// createMerchantPromotionHandler POST /merchant/v1/promotions
func createMerchantPromotionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreatePromotionReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantCreatePromotion(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// listMerchantPromotionsHandler GET /merchant/v1/promotions
func listMerchantPromotionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListPromotionsReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantListPromotions(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// getMerchantPromotionHandler GET /merchant/v1/promotions/:id
func getMerchantPromotionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
		resp, err := logic.MerchantGetPromotion(r.Context(), svcCtx, id)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// updateMerchantPromotionHandler PUT /merchant/v1/promotions/:id
func updateMerchantPromotionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
		var req types.UpdatePromotionReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantUpdatePromotion(r.Context(), svcCtx, id, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// changeMerchantPromotionStatusHandler POST /merchant/v1/promotions/:id/online | /offline
//
// 状态机说明（在 promotion-rpc/ChangeActivityStatus）：
//   0 草稿 -> 1 待开始（自动检测时间窗，已开始则跳 2 进行中）
//   1/2 -> 4 已下线
func changeMerchantPromotionStatusHandler(svcCtx *svc.ServiceContext, status int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
		resp, err := logic.MerchantChangePromotionStatus(r.Context(), svcCtx, id, status)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
