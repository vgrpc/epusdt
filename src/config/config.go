package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"time"
)

var (
	AppDebug            bool
	MysqlDns            string
	RuntimePath         string
	LogSavePath         string
	StaticPath          string
	TgBotToken          string
	TgProxy             string
	TgManage            int64
	UsdtRate            float64
	TronNet             string
	UsdtContractAddress string
	OklinkKey           string
	QueryChannel        string
)

func Init() {
	viper.AddConfigPath("./")
	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	gwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	AppDebug = viper.GetBool("app_debug")
	StaticPath = viper.GetString("static_path")
	RuntimePath = fmt.Sprintf(
		"%s%s",
		gwd,
		viper.GetString("runtime_root_path"))
	LogSavePath = fmt.Sprintf(
		"%s%s",
		RuntimePath,
		viper.GetString("log_save_path"))
	MysqlDns = fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		viper.GetString("mysql_user"),
		viper.GetString("mysql_passwd"),
		fmt.Sprintf(
			"%s:%s",
			viper.GetString("mysql_host"),
			viper.GetString("mysql_port")),
		viper.GetString("mysql_database"))
	TgBotToken = viper.GetString("tg_bot_token")
	TgProxy = viper.GetString("tg_proxy")
	TgManage = viper.GetInt64("tg_manage")
	TronNet = viper.GetString("tron_net")
	UsdtContractAddress = viper.GetString("usdt_contract_address")
	OklinkKey = viper.GetString("oklink_key")
	QueryChannel = viper.GetString("query_channel")
}

func GetAppVersion() string {
	return "0.0.12"
}

func GetAppName() string {
	appName := viper.GetString("app_name")
	if appName == "" {
		return "epusdt"
	}
	return appName
}

func GetAppUri() string {
	return viper.GetString("app_uri")
}

func GetApiAuthToken() string {
	return viper.GetString("api_auth_token")
}

func GetUsdtRate() float64 {
	forcedUsdtRate := viper.GetFloat64("forced_usdt_rate")
	if forcedUsdtRate > 0 {
		return forcedUsdtRate
	}
	if UsdtRate <= 0 {
		return 6.4
	}
	return UsdtRate
}

func GetOrderExpirationTime() int {
	timer := viper.GetInt("order_expiration_time")
	if timer <= 0 {
		return 10
	}
	return timer
}

func GetOrderExpirationTimeDuration() time.Duration {
	timer := GetOrderExpirationTime()
	return time.Minute * time.Duration(timer)
}

func GetTronApiUri() string {
	switch TronNet {
	case "MAIN":
		return "https://apilist.tronscanapi.com/api/transfer/trc20"
	case "SHASTA":
		return "https://shastapi.tronscan.org/api/transfer/trc20"
	case "NILE":
		return "https://nileapi.tronscan.org/api/transfer/trc20"
	default:
		return "https://shastapi.tronscan.org/api/transfer/trc20"
	}
}
