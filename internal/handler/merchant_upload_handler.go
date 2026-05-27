package handler

import (
	"net/http"

	"mall-admin-api/internal/logic"
	"mall-admin-api/internal/svc"
)

// uploadProductImageHandler 接受 multipart/form-data 的 file 字段，
// 限 6MB form size (5MB 文件 + 表单开销)。校验后上传到 MinIO 返 URL+key。
func uploadProductImageHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(6 << 20); err != nil {
			writeErr(r, w, err)
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			writeErr(r, w, err)
			return
		}
		defer file.Close()
		resp, err := logic.UploadProductImage(r.Context(), svcCtx, hdr.Filename, file, hdr.Size)
		if err != nil {
			writeErr(r, w, err)
			return
		}
		writeOk(r, w, resp)
	}
}
