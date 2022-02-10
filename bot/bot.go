package bot

import (
	"fmt"
	"github.com/Mrs4s/MiraiGo/binary"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/Sora233/MiraiGo-Template/utils"
	"github.com/sirupsen/logrus"
)

var reloginLock = new(sync.Mutex)

const sessionToken = "session.token"

// Bot 全局 Bot
type Bot struct {
	*client.QQClient

	start    bool
	isQRCode bool
}

func (bot *Bot) saveToken() {
	_ = ioutil.WriteFile(sessionToken, bot.GenToken(), 0o677)
}
func (bot *Bot) clearToken() {
	os.Remove(sessionToken)
}

func (bot *Bot) getToken() ([]byte, error) {
	return ioutil.ReadFile(sessionToken)
}

// ReLogin 掉线时可以尝试使用会话缓存重新登陆，只允许在OnDisconnected中调用
func (bot *Bot) ReLogin(e *client.ClientDisconnectedEvent) error {
	reloginLock.Lock()
	defer reloginLock.Unlock()
	if bot.Online.Load() {
		return nil
	}
	logger.Warnf("Bot已离线: %v", e.Message)
	logger.Warnf("尝试重连...")
	token, err := bot.getToken()
	if err == nil {
		err = bot.TokenLogin(token)
		if err == nil {
			bot.saveToken()
			return nil
		}
	}
	logger.Warnf("快速重连失败: %v", err)
	if bot.isQRCode {
		logger.Errorf("快速重连失败, 扫码登录无法恢复会话.")
		return fmt.Errorf("qrcode login relogin failed")
	}
	logger.Warnf("快速重连失败, 尝试普通登录. 这可能是因为其他端强行T下线导致的.")
	time.Sleep(time.Second)

	err = commonLogin()
	if err != nil {
		logger.Errorf("登录时发生致命错误: %v", err)
	} else {
		bot.saveToken()
	}
	return err
}

// Instance Bot 实例
var Instance *Bot

var logger = logrus.WithField("bot", "internal")

// Init 快速初始化
// 使用 config.GlobalConfig 初始化账号
// 使用 ./device.json 初始化设备信息
func Init() {
	deviceJson := utils.ReadFile("./device.json")
	if deviceJson == nil {
		logger.Fatal("无法读取 ./device.json")
	}
	err := client.SystemDeviceInfo.ReadJson(deviceJson)
	if err != nil {
		logger.Fatalf("读取device.json发生错误 - %v", err)
	}

	account := config.GlobalConfig.GetInt64("bot.account")
	password := config.GlobalConfig.GetString("bot.password")

	initBot(account, password)
}

// initBot 使用 account password 进行初始化账号
func initBot(account int64, password string) {
	if account == 0 {
		Instance = &Bot{
			QQClient: client.NewClientEmpty(),
			isQRCode: true,
		}
	} else {
		Instance = &Bot{
			QQClient: client.NewClient(account, password),
		}
	}
}

// UseDevice 使用 device 进行初始化设备信息
func UseDevice(device []byte) error {
	return client.SystemDeviceInfo.ReadJson(device)
}

// GenRandomDevice 生成随机设备信息
func GenRandomDevice() {
	client.GenRandomDevice()
	b, _ := utils.FileExist("./device.json")
	if b {
		logger.Warn("device.json exists, will not write device to file")
		return
	}
	err := ioutil.WriteFile("device.json", client.SystemDeviceInfo.ToJson(), os.FileMode(0755))
	if err != nil {
		logger.WithError(err).Errorf("unable to write device.json")
	}
}

// Login 登录
func Login() {
	logger.Info("开始尝试登录并同步消息...")
	logger.Infof("使用协议: %s", client.SystemDeviceInfo.Protocol)

	if ok, _ := utils.FileExist(sessionToken); ok {
		token, err := Instance.getToken()
		if err != nil {
			goto NormalLogin
		}
		if Instance.Uin != 0 {
			r := binary.NewReader(token)
			sessionUin := r.ReadInt64()
			if sessionUin != Instance.Uin {
				logger.Warnf("QQ号(%v)与会话缓存内的QQ号(%v)不符，将清除会话缓存", Instance.Uin, sessionUin)
				Instance.clearToken()
				goto NormalLogin
			}
		}
		if err = Instance.TokenLogin(token); err != nil {
			Instance.clearToken()
			logger.Warnf("恢复会话失败: %v , 尝试使用正常流程登录.", err)
			time.Sleep(time.Second)
			Instance.Disconnect()
			Instance.Release()
			Init()
		} else {
			Instance.saveToken()
			logger.Debug("恢复会话成功")
			return
		}
	}

NormalLogin:
	if Instance.Uin == 0 {
		logger.Info("未指定账号密码，请扫码登陆")
		err := qrcodeLogin()
		if err != nil {
			logger.Fatalf("login failed: %v", err)
		}
	} else {
		logger.Info("使用帐号密码登陆")
		err := commonLogin()
		if err != nil {
			logger.Fatalf("login failed: %v", err)
		}
	}
	Instance.saveToken()
}

// RefreshList 刷新联系人
func RefreshList() {
	logger.Info("start reload friends list")
	err := Instance.ReloadFriendList()
	if err != nil {
		logger.WithError(err).Error("unable to load friends list")
	}
	logger.Infof("load %d friends", len(Instance.FriendList))
	logger.Info("start reload groups list")
	err = Instance.ReloadGroupList()
	if err != nil {
		logger.WithError(err).Error("unable to load groups list")
	}
	logger.Infof("load %d groups", len(Instance.GroupList))
}

// StartService 启动服务
// 根据 Module 生命周期 此过程应在Login前调用
// 请勿重复调用
func StartService() {
	if Instance.start {
		return
	}

	Instance.start = true

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
		mi.Instance.Serve(Instance)
	}
	logger.Info("all modules serve functions registered")

	logger.Info("starting modules tasks ...")
	for _, mi := range modules {
		go mi.Instance.Start(Instance)
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
		mi.Instance.Stop(Instance, &wg)
	}
	wg.Wait()
	logger.Info("stopped")
	modules = make(map[string]ModuleInfo)
}
