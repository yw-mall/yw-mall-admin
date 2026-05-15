package logic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"mall-admin-api/internal/middleware"
	"mall-admin-api/internal/svc"
	"mall-admin-api/internal/types"

	"mall-user-rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

// -----------------------------------------------------------------------------
// S4.2 failed-login lockout (Redis counters) + IP whitelist check
// -----------------------------------------------------------------------------

const (
	lockMaxAttempts = 5
	lockWindow      = 30 * time.Minute
)

// failKey returns the per-username and per-IP counter keys. Two separate
// namespaces ('user' vs 'admin') so that locking out alice on the c-side
// doesn't lock out an admin called alice on the admin side.
func failKey(scope, key string) string { return "login_fail:" + scope + ":" + key }

// CheckLoginLock returns nil when the (username, ip) pair is below the
// lockout threshold; otherwise an error including remaining seconds.
func CheckLoginLock(ctx context.Context, svcCtx *svc.ServiceContext, scope, username, ip string) error {
	usr, err := svcCtx.Redis.Get(ctx, failKey(scope, username)).Int64()
	if err != nil && err.Error() != "redis: nil" {
		// Soft-fail: don't lock people out because Redis blipped.
		return nil
	}
	ipCount, err := svcCtx.Redis.Get(ctx, failKey(scope, ip)).Int64()
	if err != nil && err.Error() != "redis: nil" {
		return nil
	}
	if usr >= lockMaxAttempts || ipCount >= lockMaxAttempts {
		// Compute remaining TTL based on whichever counter is over.
		ttl := svcCtx.Redis.TTL(ctx, failKey(scope, username)).Val()
		if ttl <= 0 {
			ttl = svcCtx.Redis.TTL(ctx, failKey(scope, ip)).Val()
		}
		remaining := int64(ttl.Seconds())
		if remaining <= 0 {
			remaining = int64(lockWindow.Seconds())
		}
		return fmt.Errorf("账号已锁定，请 %d 秒后重试", remaining)
	}
	return nil
}

// MarkLoginFail INCRs both counters and EXPIREs them.
func MarkLoginFail(ctx context.Context, svcCtx *svc.ServiceContext, scope, username, ip string) {
	pipe := svcCtx.Redis.Pipeline()
	if username != "" {
		pipe.Incr(ctx, failKey(scope, username))
		pipe.Expire(ctx, failKey(scope, username), lockWindow)
	}
	if ip != "" {
		pipe.Incr(ctx, failKey(scope, ip))
		pipe.Expire(ctx, failKey(scope, ip), lockWindow)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		logx.WithContext(ctx).Errorf("MarkLoginFail: %v", err)
	}
}

// ClearLoginFail wipes both counters on a successful login.
func ClearLoginFail(ctx context.Context, svcCtx *svc.ServiceContext, scope, username, ip string) {
	keys := []string{}
	if username != "" {
		keys = append(keys, failKey(scope, username))
	}
	if ip != "" {
		keys = append(keys, failKey(scope, ip))
	}
	if len(keys) > 0 {
		_ = svcCtx.Redis.Del(ctx, keys...).Err()
	}
}

// ClientIP extracts the best-effort client IP. X-Forwarded-For wins (we trust
// the local nginx/CDN), then RemoteAddr (host:port → strip port).
func ClientIP(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-For"); h != "" {
		// First entry in a comma-separated chain is the original client.
		if i := strings.Index(h, ","); i > 0 {
			return strings.TrimSpace(h[:i])
		}
		return strings.TrimSpace(h)
	}
	if h := r.Header.Get("X-Real-IP"); h != "" {
		return strings.TrimSpace(h)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// -----------------------------------------------------------------------------
// S4.1 MFA challenge token (Redis-backed handoff between /login and /login/mfa)
// -----------------------------------------------------------------------------

const mfaChallengeTTL = 5 * time.Minute

type mfaChallengePayload struct {
	AdminId  int64    `json:"adminId"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Perms    []string `json:"perms"`
	Ip       string   `json:"ip"`
}

func mfaChallengeKey(token string) string { return "mfa_challenge:" + token }

func newChallengeToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// stashChallenge writes the post-password-but-pre-MFA state under a random
// token and returns the token. The FE sends this back with the MFA code.
func stashChallenge(ctx context.Context, svcCtx *svc.ServiceContext, p mfaChallengePayload) (string, error) {
	tok, err := newChallengeToken()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if err := svcCtx.Redis.Set(ctx, mfaChallengeKey(tok), data, mfaChallengeTTL).Err(); err != nil {
		return "", err
	}
	return tok, nil
}

func popChallenge(ctx context.Context, svcCtx *svc.ServiceContext, tok string) (*mfaChallengePayload, error) {
	if tok == "" {
		return nil, errors.New("challengeToken required")
	}
	raw, err := svcCtx.Redis.Get(ctx, mfaChallengeKey(tok)).Bytes()
	if err != nil {
		return nil, errors.New("challenge token expired or invalid")
	}
	_ = svcCtx.Redis.Del(ctx, mfaChallengeKey(tok)).Err() // single-use
	var p mfaChallengePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// -----------------------------------------------------------------------------
// Mock SMS for the MFA backup channel
// -----------------------------------------------------------------------------

func smsKey(adminId int64) string { return fmt.Sprintf("sms:admin_mfa:%d", adminId) }

// SmsSend generates a 6-digit code, stores it in Redis (TTL 5 min), and
// "sends" it via logx so test/dev can grab it from logs.
func SmsSend(ctx context.Context, svcCtx *svc.ServiceContext, adminId int64) error {
	code, err := randomDigitCode(6)
	if err != nil {
		return err
	}
	if err := svcCtx.Redis.Set(ctx, smsKey(adminId), code, 5*time.Minute).Err(); err != nil {
		return err
	}
	logx.WithContext(ctx).Infof("[mock-sms] admin=%d code=%s", adminId, code)
	return nil
}

// SmsVerify checks the code, deletes the entry on success.
func SmsVerify(ctx context.Context, svcCtx *svc.ServiceContext, adminId int64, code string) bool {
	stored, err := svcCtx.Redis.Get(ctx, smsKey(adminId)).Result()
	if err != nil || stored == "" {
		return false
	}
	if stored != code {
		return false
	}
	_ = svcCtx.Redis.Del(ctx, smsKey(adminId)).Err()
	return true
}

func randomDigitCode(n int) (string, error) {
	const digits = "0123456789"
	out := make([]byte, n)
	max := big.NewInt(int64(len(digits)))
	for i := 0; i < n; i++ {
		x, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = digits[x.Int64()]
	}
	return string(out), nil
}

// -----------------------------------------------------------------------------
// IP whitelist check (S4.2)
// -----------------------------------------------------------------------------

// EnforceIpWhitelist returns nil when the admin has no whitelist rows OR ip
// matches at least one CIDR. Empty whitelist == no restriction (avoids
// locking everyone out on first deploy).
func EnforceIpWhitelist(ctx context.Context, svcCtx *svc.ServiceContext, adminId int64, ip string) error {
	rows, err := svcCtx.UserRpc.ListAdminIpWhitelist(ctx, &userclient.ListAdminIpWhitelistReq{AdminId: adminId})
	if err != nil {
		// Don't block login on a transient RPC blip.
		logx.WithContext(ctx).Errorf("EnforceIpWhitelist: list failed: %v", err)
		return nil
	}
	if len(rows.Items) == 0 {
		return nil
	}
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return errors.New("IP 不在白名单")
	}
	for _, r := range rows.Items {
		_, ipNet, err := net.ParseCIDR(r.Cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(parsed) {
			return nil
		}
	}
	return errors.New("IP 不在白名单")
}

// -----------------------------------------------------------------------------
// MFA login second-stage handler
// -----------------------------------------------------------------------------

func MfaLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MfaLoginReq) (*types.LoginResp, error) {
	p, err := popChallenge(ctx, svcCtx, req.ChallengeToken)
	if err != nil {
		return nil, err
	}
	// Try TOTP / backup first via user-rpc.
	if _, err := svcCtx.UserRpc.VerifyAdminMfa(ctx, &userclient.VerifyAdminMfaReq{
		AdminId: p.AdminId, Code: req.Code,
	}); err != nil {
		// Fallback to mock SMS.
		if !SmsVerify(ctx, svcCtx, p.AdminId, req.Code) {
			return nil, errors.New("MFA 验证码不正确")
		}
	}
	return mintAdminSession(ctx, svcCtx, p)
}

func MfaSmsSend(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MfaSmsSendReq) (*types.MfaSmsSendResp, error) {
	// Re-stash the challenge so /sms-verify still finds the payload (popChallenge
	// is single-shot for /login/mfa, but the SMS path needs a separate verify).
	raw, err := svcCtx.Redis.Get(ctx, mfaChallengeKey(req.ChallengeToken)).Bytes()
	if err != nil {
		return nil, errors.New("challenge token expired or invalid")
	}
	var p mfaChallengePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if err := SmsSend(ctx, svcCtx, p.AdminId); err != nil {
		return nil, err
	}
	return &types.MfaSmsSendResp{Ok: true}, nil
}

// mintAdminSession is the shared post-MFA path: create the session with the
// permissions snapshot we stashed before the MFA challenge.
func mintAdminSession(ctx context.Context, svcCtx *svc.ServiceContext, p *mfaChallengePayload) (*types.LoginResp, error) {
	sess, err := svcCtx.UserRpc.CreateSession(ctx, &userclient.CreateSessionReq{
		Uid:      p.AdminId,
		Username: p.Username,
		Role:     "admin",
		Perms:    p.Perms,
		Ip:       p.Ip,
	})
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{
		Token:        sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		ExpiresIn:    sess.ExpiresIn,
		CsrfToken:    sess.CsrfToken,
		Id:           p.AdminId,
		Username:     p.Username,
		Role:         "admin",
		Permissions:  p.Perms,
	}, nil
}

// -----------------------------------------------------------------------------
// IP whitelist CRUD (admin self-management)
// -----------------------------------------------------------------------------

func ListIpWhitelist(ctx context.Context, svcCtx *svc.ServiceContext) (*types.ListIpWhitelistResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	res, err := svcCtx.UserRpc.ListAdminIpWhitelist(ctx, &userclient.ListAdminIpWhitelistReq{AdminId: c.Uid})
	if err != nil {
		return nil, err
	}
	out := make([]types.IpWhitelistEntry, 0, len(res.Items))
	for _, r := range res.Items {
		out = append(out, types.IpWhitelistEntry{
			Id: r.Id, AdminId: r.AdminId, Cidr: r.Cidr, Note: r.Note, CreateTime: r.CreateTime,
		})
	}
	return &types.ListIpWhitelistResp{Items: out}, nil
}

func AddIpWhitelist(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AddIpWhitelistReq) (*types.AddIpWhitelistResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	res, err := svcCtx.UserRpc.AddAdminIpWhitelist(ctx, &userclient.AddAdminIpWhitelistReq{
		AdminId: c.Uid, Cidr: req.Cidr, Note: req.Note,
	})
	if err != nil {
		return nil, err
	}
	return &types.AddIpWhitelistResp{Id: res.Id}, nil
}

func DeleteIpWhitelist(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.LogoutResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	if _, err := svcCtx.UserRpc.DeleteAdminIpWhitelist(ctx, &userclient.DeleteAdminIpWhitelistReq{
		Id: id, AdminId: c.Uid,
	}); err != nil {
		return nil, err
	}
	return &types.LogoutResp{Ok: true}, nil
}

// -----------------------------------------------------------------------------
// MFA self-management
// -----------------------------------------------------------------------------

func MfaStatus(ctx context.Context, svcCtx *svc.ServiceContext) (*types.MfaStatusResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	res, err := svcCtx.UserRpc.GetAdminMfaStatus(ctx, &userclient.GetAdminMfaStatusReq{AdminId: c.Uid})
	if err != nil {
		return nil, err
	}
	return &types.MfaStatusResp{Enabled: res.Enabled, LastUsedAt: res.LastUsedAt}, nil
}

func MfaEnable(ctx context.Context, svcCtx *svc.ServiceContext) (*types.MfaEnableResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	res, err := svcCtx.UserRpc.EnableAdminMfa(ctx, &userclient.EnableAdminMfaReq{AdminId: c.Uid})
	if err != nil {
		return nil, err
	}
	return &types.MfaEnableResp{
		TotpSecret:  res.TotpSecret,
		QrUrl:       res.QrUrl,
		BackupCodes: res.BackupCodes,
	}, nil
}

func MfaConfirm(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MfaConfirmReq) (*types.LogoutResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	if _, err := svcCtx.UserRpc.ConfirmAdminMfa(ctx, &userclient.ConfirmAdminMfaReq{
		AdminId: c.Uid, Code: req.Code,
	}); err != nil {
		return nil, err
	}
	return &types.LogoutResp{Ok: true}, nil
}

func MfaDisable(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MfaDisableReq) (*types.LogoutResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	if _, err := svcCtx.UserRpc.DisableAdminMfa(ctx, &userclient.DisableAdminMfaReq{
		AdminId: c.Uid, Code: req.Code,
	}); err != nil {
		return nil, err
	}
	return &types.LogoutResp{Ok: true}, nil
}

// -----------------------------------------------------------------------------
// Change password (admin self)
// -----------------------------------------------------------------------------

func ChangePassword(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChangePasswordReq) (*types.LogoutResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	if _, err := svcCtx.UserRpc.ChangePassword(ctx, &userclient.ChangePasswordReq{
		SubjectType: 2, // admin
		SubjectId:   c.Uid,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}); err != nil {
		return nil, err
	}
	return &types.LogoutResp{Ok: true}, nil
}

// -----------------------------------------------------------------------------
// KYC review (admin)
// -----------------------------------------------------------------------------

func ListPendingKyc(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPendingKycReq) (*types.ListPendingKycResp, error) {
	res, err := svcCtx.UserRpc.ListPendingKyc(ctx, &userclient.ListPendingKycReq{
		Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]types.KycPendingItemDTO, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, types.KycPendingItemDTO{
			UserId:         it.UserId,
			Username:       it.Username,
			RealName:       it.RealName,
			IdCardNo:       it.IdCardNo,
			IdCardFrontUrl: it.IdCardFrontUrl,
			IdCardBackUrl:  it.IdCardBackUrl,
			FaceVideoUrl:   it.FaceVideoUrl,
			SubmitTime:     it.SubmitTime,
			Status:         it.Status,
		})
	}
	return &types.ListPendingKycResp{Items: out, Total: res.Total}, nil
}

func AuditKyc(ctx context.Context, svcCtx *svc.ServiceContext, userId int64, req *types.AuditKycReq) (*types.LogoutResp, error) {
	c, _ := middleware.ClaimsFromContext(ctx)
	if c == nil {
		return nil, errors.New("unauthenticated")
	}
	if _, err := svcCtx.UserRpc.AdminAuditKyc(ctx, &userclient.AdminAuditKycReq{
		UserId: userId, Pass: req.Pass, Reason: req.Reason, AuditAdminId: c.Uid,
	}); err != nil {
		return nil, err
	}
	return &types.LogoutResp{Ok: true}, nil
}

// -----------------------------------------------------------------------------
// OpLog query
// -----------------------------------------------------------------------------

func QueryOpLog(ctx context.Context, svcCtx *svc.ServiceContext, req *types.OpLogQueryReq) (*types.OpLogQueryResp, error) {
	if svcCtx.OpLogDB == nil {
		return &types.OpLogQueryResp{Total: 0, Items: nil}, nil
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	conds := []string{"1=1"}
	args := []any{}
	if req.ActorId > 0 {
		conds = append(conds, "actor_id=?")
		args = append(args, req.ActorId)
	}
	if req.ActorRole != "" {
		conds = append(conds, "actor_role=?")
		args = append(args, req.ActorRole)
	}
	if req.Method != "" {
		conds = append(conds, "method=?")
		args = append(args, strings.ToUpper(req.Method))
	}
	if req.Path != "" {
		conds = append(conds, "path LIKE ?")
		args = append(args, "%"+req.Path+"%")
	}
	if req.StatusMin > 0 {
		conds = append(conds, "status_code>=?")
		args = append(args, req.StatusMin)
	}
	if req.StatusMax > 0 {
		conds = append(conds, "status_code<=?")
		args = append(args, req.StatusMax)
	}
	if req.Since > 0 {
		conds = append(conds, "create_time>=?")
		args = append(args, req.Since)
	}
	if req.Until > 0 {
		conds = append(conds, "create_time<=?")
		args = append(args, req.Until)
	}
	where := strings.Join(conds, " AND ")

	var total int64
	if err := svcCtx.OpLogDB.QueryRowCtx(ctx, &total,
		"SELECT COUNT(*) FROM admin_op_log WHERE "+where, args...); err != nil {
		return nil, err
	}

	type row struct {
		Id          int64  `db:"id"`
		ActorId     int64  `db:"actor_id"`
		ActorRole   string `db:"actor_role"`
		Method      string `db:"method"`
		Path        string `db:"path"`
		RequestBody string `db:"request_body"`
		StatusCode  int32  `db:"status_code"`
		Ip          string `db:"ip"`
		CreateTime  int64  `db:"create_time"`
	}
	var rows []*row
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	if err := svcCtx.OpLogDB.QueryRowsCtx(ctx, &rows,
		"SELECT id, actor_id, actor_role, method, path, COALESCE(request_body,'') AS request_body, status_code, ip, create_time FROM admin_op_log WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?",
		queryArgs...); err != nil {
		return nil, err
	}
	items := make([]types.OpLogEntryDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.OpLogEntryDTO{
			Id:          r.Id,
			ActorId:     r.ActorId,
			ActorRole:   r.ActorRole,
			Method:      r.Method,
			Path:        r.Path,
			RequestBody: r.RequestBody,
			StatusCode:  r.StatusCode,
			Ip:          r.Ip,
			CreateTime:  r.CreateTime,
		})
	}
	return &types.OpLogQueryResp{Total: total, Items: items}, nil
}
