package bot

import (
	"bytes"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/tuotoo/qrcode"

	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/Sora233/MiraiGo-Template/utils"

	"github.com/LagrangeDev/LagrangeGo/client"
	"github.com/LagrangeDev/LagrangeGo/client/auth"
	"github.com/LagrangeDev/LagrangeGo/client/packets/wtlogin/qrcodestate"
	"github.com/sirupsen/logrus"
)

// Bot 全局 Bot
type Bot struct {
	*client.QQClient

	start atomic.Bool
}

var logger = logrus.WithField("bot", "internal")

// Bot 实例
var QQClient *Bot

func Init(logger *utils.ProtocolLogger) {
	appInfo := auth.AppList["linux"]["3.2.15-30366"]
	qqClientInstance := client.NewClient(config.GlobalConfig.GetUint32("bot.account"), config.GlobalConfig.GetString("bot.password"))
	qqClientInstance.SetLogger(logger)
	qqClientInstance.UseVersion(appInfo)
	qqClientInstance.AddSignServer(config.GlobalConfig.GetString("signServer"))
	qqClientInstance.UseDevice(auth.NewDeviceInfo(config.GlobalConfig.GetInt("bot.account")))

	data, err := os.ReadFile("sig.bin")
	if err != nil {
		logrus.Warnln("read sig error:", err)
	} else {
		sig, err := auth.UnmarshalSigInfo(data, true)
		if err != nil {
			logrus.Warnln("load sig error:", err)
		} else {
			qqClientInstance.UseSig(sig)
		}
	}
	QQClient = &Bot{QQClient: qqClientInstance}
}

// Login 登录
func Login() error {
	//自动登录
	sig := QQClient.Sig()
	if sig != nil {
		err := QQClient.FastLogin()
		if err != nil {
			logrus.Errorln("fast login err:", err)
			logrus.Println("change to qrcode mode.")
		} else {
			return nil
		}
	}

	//获取二维码
	png, _, err := QQClient.FetchQRCodeDefault()
	if err != nil {
		logrus.Errorln("login err:", err)
		return err
	}

	// 打印二维码内容
	qrMatrix, err := qrcode.Decode(bytes.NewReader(png))
	if err != nil {
		logrus.Warnln("二维码内容识别失败，可能无法终端打印:", err)
	} else {
		logrus.Infoln("请使用手机扫码登录：")
		config := qrterminal.Config{
			Level:     qrterminal.M,
			Writer:    os.Stdout,
			BlackChar: qrterminal.WHITE,
			WhiteChar: qrterminal.BLACK,
			QuietZone: 1,
		}
		qrterminal.GenerateWithConfig(qrMatrix.Content, config)
	}

	//轮询登录状态
	var retCode qrcodestate.State
	for {
		retCode, err = QQClient.GetQRCodeResult()
		if err != nil {
			logrus.Errorln(err)
			return err
		}
		// 等待扫码
		if retCode.Waitable() {
			time.Sleep(3 * time.Second)
			continue
		}
		if !retCode.Success() {
			return errors.New(retCode.Name())
		}
		break
	}
	_, err = QQClient.QRCodeLogin()
	if err != nil {
		logrus.Errorln("login err:", err)
		return err
	}
	return nil
}

// 保存sign
func Dumpsig() {
	data, err := QQClient.Sig().Marshal()
	if err != nil {
		logrus.Errorln("marshal sig.bin err:", err)
		return
	}
	err = os.WriteFile("sig.bin", data, 0644)
	if err != nil {
		logrus.Errorln("write sig.bin err:", err)
		return
	}
	logrus.Infoln("sig saved into sig.bin")
}

// StartService 启动服务
// 根据 Module 生命周期 此过程应在Login前调用
// 请勿重复调用
func StartService() {
	if !QQClient.start.CompareAndSwap(false, true) {
		return
	}

	logger.Infof("initializing modules ...")
	for _, mi := range modules {
		mi.Instance.Init()
	}
	for _, mi := range modules {
		mi.Instance.PostInit()
	}
	logger.Info("all modules initialized")

	logger.Info("registering modules serve functions ...")
	for _, mi := range modules {
		mi.Instance.Serve(QQClient)
	}
	logger.Info("all modules serve functions registered")

	logger.Info("starting modules tasks ...")
	for _, mi := range modules {
		go mi.Instance.Start(QQClient)
	}
	logger.Info("tasks running")
}

// Stop 停止所有服务
// 调用此函数并不会使Bot离线
func Stop() {
	logger.Warn("stopping ...")
	wg := sync.WaitGroup{}
	for _, mi := range modules {
		wg.Add(1)
		mi.Instance.Stop(QQClient, &wg)
	}
	wg.Wait()
	logger.Info("stopped")
	modules = make(map[string]ModuleInfo)
}
