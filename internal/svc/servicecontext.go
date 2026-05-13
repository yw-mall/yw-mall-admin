package svc

import (
	"mall-activity-rpc/activityclient"
	"mall-admin-api/internal/config"
	"mall-admin-api/internal/middleware"
	"mall-logistics-rpc/logisticsclient"
	"mall-order-rpc/orderclient"
	"mall-payment-rpc/paymentclient"
	"mall-product-rpc/productclient"
	"mall-review-rpc/reviewclient"
	"mall-risk-rpc/riskclient"
	"mall-rule-rpc/ruleclient"
	"mall-shop-rpc/shopservice"
	"mall-user-rpc/userclient"

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
}

func NewServiceContext(c config.Config) *ServiceContext {
	userRpc := userclient.NewUser(zrpc.MustNewClient(c.UserRpc))
	adminMw := middleware.NewSessionAuthMiddleware(userRpc, "admin")
	merchantMw := middleware.NewSessionAuthMiddleware(userRpc, "merchant")
	opLog := middleware.NewOpLogMiddleware()

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
	}
}
