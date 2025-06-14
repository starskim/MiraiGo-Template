package main

import (
	"os"
	"os/signal"

	"github.com/Sora233/MiraiGo-Template/bot"
	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/Sora233/MiraiGo-Template/utils"

	_ "github.com/Sora233/MiraiGo-Template/modules/logging"
)

func init() {
	utils.WriteLogToFS()
	config.Init()
}

func main() {
	// 快速初始化
	bot.Init(&utils.ProtocolLogger{})

	// 初始化 Modules
	bot.StartService()

	// 登录
	bot.Login()

	defer bot.Dumpsig()
	defer bot.QQClient.Release()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	<-ch
	bot.Stop()
}
