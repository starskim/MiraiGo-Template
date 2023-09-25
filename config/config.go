package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Bot 账号配置
type BotConfig struct {
	Account  int64  `mapstructure:"account"`
	Password string `mapstructure:"password"`
}

// Sign 签名配置
type SignConfig struct {
	SignServer       string `mapstructure:"server"`
	SignServerBearer string `mapstructure:"server-bearer"`
	Key              string `mapstructure:"key"`
	IsBelow110       bool   `mapstructure:"is-below-110"`
	AutoRegister     bool   `mapstructure:"auto-register"`
	AutoRefreshToken bool   `mapstructure:"auto-refresh-token"`
	RefreshInterval  int64  `mapstructure:"refresh-interval"`
}

// Config 总配置文件
type Config struct {
	Bot  *BotConfig  `mapstructure:"bot"`
	Sign *SignConfig `mapstructure:"sign"`
}

type ViperConfig struct {
	*viper.Viper
}

// GlobalConfig 默认全局配置
var GlobalConfig = &ViperConfig{
	viper.New(),
}

// config
var (
	Bot               *BotConfig  // Bot配置
	Sign              *SignConfig // 签名配置
	SignServer        string      // 使用特定的服务器进行签名
	SignServerBearer  string      // 认证签名服务器的 Bearer Token
	Key               string      // 签名服务器密钥
	IsBelow110        bool        // 签名服务器版本是否低于1.1.0及以下
	HTTPTimeout       int         // download 超时时间
	SignServerTimeout int         // 签名服务器超时时间
)

func Base() {
	config := &Config{}
	err := GlobalConfig.Unmarshal(&config)
	if err != nil {
		logrus.Fatal("配置文件不合法!", err)
	}
	{
		Bot = config.Bot
		Sign = config.Sign
		SignServer = config.Sign.SignServer
		SignServerBearer = config.Sign.SignServerBearer
		Key = config.Sign.Key
		IsBelow110 = config.Sign.IsBelow110
	}
}

// Init 使用 ./application.yaml 初始化全局配置
func Init() {
	GlobalConfig.SetConfigName("application")
	GlobalConfig.SetConfigType("yaml")
	GlobalConfig.AddConfigPath(".")
	GlobalConfig.AddConfigPath("./config")

	err := GlobalConfig.ReadInConfig()
	if err != nil {
		logrus.WithField("config", "GlobalConfig").WithError(err).Fatal("unable to read global config")
	}
	Base()
}
