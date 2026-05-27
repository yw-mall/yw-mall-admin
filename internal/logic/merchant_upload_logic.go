package logic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"
)

const maxImageBytes = 5 * 1024 * 1024 // 5 MB

var allowedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// UploadProductImage 把图片放到 MinIO 的 product-images bucket，按 shop_id 分目录。
// 返回 URL（生产由 nginx 代理 /minio/，dev 直 :9000）+ key 给 FE 删除用。
func UploadProductImage(ctx context.Context, svcCtx *svc.ServiceContext, filename string, body io.Reader, size int64) (*types.UploadImageResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil || c.ShopId <= 0 {
		return nil, errors.New("not in a shop")
	}
	if svcCtx.MinIO == nil {
		return nil, errors.New("MinIO not configured")
	}
	if size <= 0 || size > maxImageBytes {
		return nil, fmt.Errorf("file size out of range (0, %d]", maxImageBytes)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedImageExts[ext] {
		return nil, errors.New("unsupported file type (allow jpg/jpeg/png/webp)")
	}
	key := fmt.Sprintf("product-images/shop-%d/%d%s", c.ShopId, time.Now().UnixNano(), ext)
	contentType := "image/" + strings.TrimPrefix(ext, ".")
	url, err := svcCtx.MinIO.PutFile(ctx, key, body, size, contentType)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	return &types.UploadImageResp{Url: url, Key: key}, nil
}
