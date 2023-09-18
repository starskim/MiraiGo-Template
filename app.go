package main

import (
	"os"
	"os/signal"

	"github.com/starskim/MiraiGo-Template/bot"
	"github.com/starskim/MiraiGo-Template/config"
	"github.com/starskim/MiraiGo-Template/utils"

	_ "github.com/starskim/MiraiGo-Template/modules/logging"
)

func init() {
	utils.WriteLogToFS()
	config.Init()
}

func main() {
	// 快速初始化
	bot.Init()

	// 初始化 Modules
	bot.StartService()

	// 登录
	bot.Login()

	// 刷新好友列表，群列表
	bot.RefreshList()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	<-ch
	bot.Stop()
}
