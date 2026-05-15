package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	UserRpc      zrpc.RpcClientConf
	ShopRpc      zrpc.RpcClientConf
	ProductRpc   zrpc.RpcClientConf
	OrderRpc     zrpc.RpcClientConf
	ReviewRpc    zrpc.RpcClientConf
	RiskRpc      zrpc.RpcClientConf
	RuleRpc      zrpc.RpcClientConf
	PaymentRpc   zrpc.RpcClientConf
	ActivityRpc  zrpc.RpcClientConf
	LogisticsRpc zrpc.RpcClientConf

	// S4 security: Redis for failed-login lock + MFA challenge tokens.
	Redis struct {
		Host string
		Pass string
		DB   int
	}

	// S4.8: DSN for the admin_op_log table (lives in mall_user database).
	OpLogDataSource string
}
