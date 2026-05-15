package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// OpLogMiddleware records admin/merchant write operations:
//   1. always: structured logx (back-compat)
//   2. when DB is non-nil: fire-and-forget INSERT INTO admin_op_log
//
// Inserts run on a fresh background context so the request handler can return
// before the DB roundtrip finishes — the audit row must not block the user.
type OpLogMiddleware struct {
	db sqlx.SqlConn
}

// NewOpLogMiddleware accepts an optional sqlx.SqlConn. Pass nil to skip DB
// writes (legacy log-only behaviour).
func NewOpLogMiddleware(db sqlx.SqlConn) *OpLogMiddleware { return &OpLogMiddleware{db: db} }

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (m *OpLogMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := strings.ToUpper(r.Method)
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			next(w, r)
			return
		}
		var snippet []byte
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			snippet = b
			r.Body = io.NopCloser(bytes.NewBuffer(b))
		}
		if len(snippet) > 4096 {
			snippet = snippet[:4096]
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next(rec, r)
		var actorId int64
		actorRole := ""
		if c, ok := ClaimsFromContext(r.Context()); ok {
			actorId = c.Uid
			actorRole = c.Role
		}
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		logx.WithContext(r.Context()).Infof(
			"[oplog] actor=%d role=%s %s %s status=%d ip=%s elapsed=%s body=%s",
			actorId, actorRole, method, r.URL.Path, rec.status, ip, time.Since(start), string(snippet))

		// S4.8 fire-and-forget DB write. Skip when no db is configured.
		if m.db != nil {
			db := m.db
			path := r.URL.Path
			body := string(snippet)
			status := int32(rec.status)
			now := time.Now().Unix()
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if _, err := db.ExecCtx(ctx, `
					INSERT INTO admin_op_log
					  (actor_id, actor_role, method, path, request_body, status_code, ip, create_time)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					actorId, actorRole, method, path, body, status, ip, now); err != nil {
					logx.Errorf("[oplog] insert failed: %v", err)
				}
			}()
		}
	}
}
