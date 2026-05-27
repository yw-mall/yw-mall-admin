package svc

import (
	"mall-activity-rpc/activityclient"
	"mall-admin-api/internal/config"
	"mall-admin-api/internal/middleware"
	"mall-common/minioutil"
	"mall-logistics-rpc/logisticsclient"
	"mall-order-rpc/orderclient"
	"mall-payment-rpc/paymentclient"
	"mall-product-rpc/productclient"
	"mall-review-rpc/reviewclient"
	"mall-risk-rpc/riskclient"
	"mall-rule-rpc/ruleclient"
	"mall-shop-rpc/shopservice"
	"mall-user-rpc/userclient"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config       config.Config
	UserRpc      userclient.User
	ShopRpc      shopservice.ShopService
	ProductRpc   productclient.Product
	OrderRpc     orderclient.Order
	ReviewRpc    reviewclient.Review
	RiskRpc      riskclient.Risk
	RuleRpc      ruleclient.Rule
	PaymentRpc   paymentclient.Payment
	ActivityRpc  activityclient.Activity
	LogisticsRpc logisticsclient.Logistics

	AdminAuth    rest.Middleware
	MerchantAuth rest.Middleware
	OpLog        rest.Middleware

	// S4 security: Redis for failed-login lock + MFA challenge tokens.
	Redis *redis.Client

	// S4.8: direct DB connection to mall_user for the admin_op_log writer +
	// op-log query endpoint. We bypass mall-user-rpc here because the table is
	// owned by the gateway, not by the user-rpc data model.
	OpLogDB sqlx.SqlConn

	// M2: MinIO for merchant 商品图片上传。配置缺时为 nil，上传端点会返业务错。
	MinIO *minioutil.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	userRpc := userclient.NewUser(zrpc.MustNewClient(c.UserRpc))
	rdb := newRedisClient(c)

	// op-log DB: optional. When the DSN is empty we just write structured logs
	// (legacy behaviour). When set, the OpLog middleware fires-and-forgets an
	// INSERT into admin_op_log too.
	var opLogDB sqlx.SqlConn
	if c.OpLogDataSource != "" {
		opLogDB = sqlx.NewMysql(c.OpLogDataSource)
	}

	adminMw := middleware.NewSessionAuthMiddleware(userRpc, "admin")
	merchantMw := middleware.NewSessionAuthMiddleware(userRpc, "merchant")
	opLog := middleware.NewOpLogMiddleware(opLogDB)

	// M2: MinIO client（可选；endpoint 空时降级，上传端点会业务错）
	var minioCli *minioutil.Client
	if c.Minio.Endpoint != "" {
		cli, err := minioutil.New(minioutil.Config{
			Endpoint:   c.Minio.Endpoint,
			AccessKey:  c.Minio.AccessKey,
			SecretKey:  c.Minio.SecretKey,
			Bucket:     c.Minio.Bucket,
			PublicHost: c.Minio.PublicHost,
			UseSSL:     c.Minio.UseSSL,
		})
		if err != nil {
			logx.Errorf("admin-api MinIO init failed: %v", err)
		} else {
			minioCli = cli
		}
	}

	return &ServiceContext{
		Config:       c,
		UserRpc:      userRpc,
		ShopRpc:      shopservice.NewShopService(zrpc.MustNewClient(c.ShopRpc)),
		ProductRpc:   productclient.NewProduct(zrpc.MustNewClient(c.ProductRpc)),
		OrderRpc:     orderclient.NewOrder(zrpc.MustNewClient(c.OrderRpc)),
		ReviewRpc:    reviewclient.NewReview(zrpc.MustNewClient(c.ReviewRpc)),
		RiskRpc:      riskclient.NewRisk(zrpc.MustNewClient(c.RiskRpc)),
		RuleRpc:      ruleclient.NewRule(zrpc.MustNewClient(c.RuleRpc)),
		PaymentRpc:   paymentclient.NewPayment(zrpc.MustNewClient(c.PaymentRpc)),
		ActivityRpc:  activityclient.NewActivity(zrpc.MustNewClient(c.ActivityRpc)),
		LogisticsRpc: logisticsclient.NewLogistics(zrpc.MustNewClient(c.LogisticsRpc)),
		AdminAuth:    adminMw.Handle,
		MerchantAuth: merchantMw.Handle,
		OpLog:        opLog.Handle,
		Redis:        rdb,
		OpLogDB:      opLogDB,
		MinIO:        minioCli,
	}
}

func newRedisClient(c config.Config) *redis.Client {
	host := c.Redis.Host
	if host == "" {
		host = "127.0.0.1:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr:     host,
		Password: c.Redis.Pass,
		DB:       c.Redis.DB,
	})
}
