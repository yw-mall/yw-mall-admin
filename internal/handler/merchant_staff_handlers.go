package handler

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"

	"mall-admin-api/internal/logic"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"
)

// extractBearerLocal duplicates middleware.extractBearer (which is unexported)
// — kept private to this handler file to avoid widening the middleware API
// surface just for the accept-invitation public route.
func extractBearerLocal(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return h
}

// ===== Staff CRUD =====

func listMerchantStaffHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.ListMerchantStaff(r.Context(), svcCtx)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

func updateMerchantStaffRoleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		var req types.UpdateStaffRoleReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.UpdateMerchantStaffRole(r.Context(), svcCtx, id, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

func disableMerchantStaffHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.DisableMerchantStaff(r.Context(), svcCtx, id)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// ===== Invitation =====

func createMerchantInvitationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateInvitationReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.CreateMerchantInvitation(r.Context(), svcCtx, &req)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

func listMerchantInvitationsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.ListMerchantInvitations(r.Context(), svcCtx)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

func revokeMerchantInvitationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseId(r)
		if err != nil || id <= 0 {
			writeErr(r, w, err)
			return
		}
		resp, err := logic.RevokeMerchantInvitation(r.Context(), svcCtx, id)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}

// acceptMerchantInvitationHandler is registered in the PUBLIC sub-route group
// because the caller's session.Role is "user" (c-side), not "merchant" yet —
// the MerchantAuth middleware would 403 them. We pull the bearer token
// ourselves and let logic.AcceptMerchantInvitation validate it via user-rpc.
func acceptMerchantInvitationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AcceptInvitationReq
		if err := httpx.Parse(r, &req); err != nil {
			writeErr(r, w, err)
			return
		}
		token := extractBearerLocal(r)
		resp, err := logic.AcceptMerchantInvitation(r.Context(), svcCtx, &req, token)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
