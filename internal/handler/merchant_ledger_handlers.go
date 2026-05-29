package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"mall-admin-api/internal/logic"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"
)

func merchantListLedgerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListLedgerReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantListLedger(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

func merchantGetLedgerSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetLedgerSummaryReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.MerchantGetLedgerSummary(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
