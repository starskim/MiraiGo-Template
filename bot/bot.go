package bot

import (
	"fmt"
	"github.com/Mrs4s/MiraiGo/binary"
	"github.com/Mrs4s/MiraiGo/wrapper"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/Sora233/MiraiGo-Template/internal/requests"
	"github.com/Sora233/MiraiGo-Template/utils"
	"github.com/sirupsen/logrus"
	"gopkg.ilharper.com/x/isatty"
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
	_ = os.WriteFile(sessionToken, bot.GenToken(), 0o677)
}
func (bot *Bot) clearToken() {
	os.Remove(sessionToken)
}

func (bot *Bot) getToken() ([]byte, error) {
	return os.ReadFile(sessionToken)
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
	err := deviceInfo.ReadJson(deviceJson)
	if err != nil {
		logger.Fatalf("读取device.json发生错误 - %v", err)
	}

	initBot(config.Bot.Account, config.Bot.Password)

	signServer, err := getAvaliableSignServer() // 获取可用签名服务器
	if err != nil {
		logger.Warn(err)
	}
	if signServer != nil && len(signServer.URL) > 1 {
		logger.Infof("使用签名服务器：%v", signServer.URL)
		go signStartRefreshToken(config.Sign.RefreshInterval) // 定时刷新 token
		wrapper.DandelionEnergy = energy
		wrapper.FekitGetSign = sign
		if !config.IsBelow110 {
			if !config.Sign.AutoRegister {
				logger.Warn("自动注册实例已关闭，请配置 sign-server 端自动注册实例以保持正常签名")
			}
			if !config.Sign.AutoRefreshToken {
				logger.Info("自动刷新 token 已关闭，token 过期后获取签名时将不会立即尝试刷新获取新 token")
			}
		} else {
			logger.Warn("签名服务器版本 <= 1.1.0 ，无法使用刷新 token 等操作，建议使用 1.1.6 版本及以上签名服务器")
		}
	} else {
		logger.Warnf("警告: 未配置签名服务器或签名服务器不可用, 这可能会导致登录 45 错误码或发送消息被风控")
	}
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
	Instance.UseDevice(deviceInfo)
}

var deviceInfo = client.GenRandomDevice()

// UseDevice 使用 device 进行初始化设备信息
func UseDevice(device []byte) error {
	return deviceInfo.ReadJson(device)
}

// GenRandomDevice 生成随机设备信息
func GenRandomDevice() {
	client.GenRandomDevice()
	b, _ := utils.FileExist("./device.json")
	if b {
		logger.Warn("device.json exists, will not write device to file")
		return
	}
	err := ioutil.WriteFile("device.json", deviceInfo.ToJson(), os.FileMode(0755))
	if err != nil {
		logger.WithError(err).Errorf("unable to write device.json")
	}
}

var remoteVersions = map[int]string{
	1: "https://raw.githubusercontent.com/RomiChan/protocol-versions/master/android_phone.json",
	6: "https://raw.githubusercontent.com/RomiChan/protocol-versions/master/android_pad.json",
}

func getRemoteLatestProtocolVersion(protocolType int) ([]byte, error) {
	url, ok := remoteVersions[protocolType]
	if !ok {
		return nil, fmt.Errorf("remote version unavailable")
	}
	response, err := requests.Request{URL: url}.Bytes()
	if err != nil {
		return requests.Request{URL: "https://ghproxy.com/" + url}.Bytes()
	}
	return response, nil
}

func readIfTTY(de string) (str string) {
	if isatty.Isatty(os.Stdin.Fd()) {
		return readLine()
	}
	logger.Warnf("未检测到输入终端，自动选择%s.", de)
	return de
}

// Login 登录
func Login() {
	logger.Info("开始尝试登录并同步消息...")
	logger.Infof("使用协议: %s", deviceInfo.Protocol)
	Instance.UseDevice(deviceInfo)

	if Instance.isQRCode && Instance.Device().Protocol != 2 {
		logger.Warn("当前协议不支持二维码登录, 请配置账号密码登录.")
		os.Exit(0)
	}

	// 加载本地版本信息, 一般是在上次登录时保存的
	versionFile := fmt.Sprintf("versions_%d.json", int(Instance.Device().Protocol))
	if ok, _ := utils.FileExist(versionFile); ok {
		b, err := os.ReadFile(versionFile)
		if err == nil {
			_ = Instance.Device().Protocol.Version().UpdateFromJson(b)
		}
		logger.Infof("从文件 %s 读取协议版本 %v.", versionFile, Instance.Device().Protocol.Version())
	}

	var isTokenLogin bool
	if ok, _ := utils.FileExist(sessionToken); ok {
		token, err := Instance.getToken()
		if err == nil {
			if Instance.Uin != 0 {
				r := binary.NewReader(token)
				cu := r.ReadInt64()
				if cu != Instance.Uin {
					logger.Warnf("警告: 配置文件内的QQ号 (%v) 与缓存内的QQ号 (%v) 不相同", Instance.Uin, cu)
					logger.Warnf("1. 使用会话缓存继续.")
					logger.Warnf("2. 删除会话缓存并重启.")
					logger.Warnf("请选择:")
					text := readIfTTY("1")
					if text == "2" {
						_ = os.Remove("session.token")
						logger.Infof("缓存已删除.")
						os.Exit(0)
					}
				}
			}
			if err = Instance.TokenLogin(token); err != nil {
				_ = os.Remove("session.token")
				logger.Warnf("恢复会话失败: %v , 尝试使用正常流程登录.", err)
				time.Sleep(time.Second)
				Instance.Disconnect()
				Instance.Release()
				Init()
				Instance.UseDevice(deviceInfo)
			} else {
				isTokenLogin = true
			}
		}
	}

	if !isTokenLogin {
		logger.Infof("正在检查协议更新...")
		oldVersionName := Instance.Device().Protocol.Version().String()
		remoteVersion, err := getRemoteLatestProtocolVersion(int(Instance.Device().Protocol.Version().Protocol))
		if err == nil {
			if err = Instance.Device().Protocol.Version().UpdateFromJson(remoteVersion); err == nil {
				if Instance.Device().Protocol.Version().String() != oldVersionName {
					logger.Infof("已自动更新协议版本: %s -> %s", oldVersionName, Instance.Device().Protocol.Version().String())
				} else {
					logger.Infof("协议已经是最新版本")
				}
				_ = os.WriteFile(versionFile, remoteVersion, 0o644)
			}
		} else if err.Error() != "remote version unavailable" {
			logger.Warnf("检查协议更新失败: %v", err)
		}
		if !Instance.isQRCode {
			if err := commonLogin(); err != nil {
				log.Fatalf("登录时发生致命错误: %v", err)
			}
		} else {
			if err := qrcodeLogin(); err != nil {
				log.Fatalf("登录时发生致命错误: %v", err)
			}
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
