package bot

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/martian/log"
	"github.com/tuotoo/qrcode"
	asc2art "github.com/yinghau76/go-ascii-art"
	"image"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Logiase/MiraiGo-Template/config"
	"github.com/Logiase/MiraiGo-Template/utils"
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/sirupsen/logrus"
)

// Bot 全局 Bot
type Bot struct {
	*client.QQClient

	start bool
}

// Instance Bot 实例
var Instance *Bot

var logger = logrus.WithField("bot", "internal")

// Init 快速初始化
// 使用 config.GlobalConfig 初始化账号
// 使用 ./device.json 初始化设备信息
func Init() {
	Instance = &Bot{
		client.NewClient(
			config.GlobalConfig.GetInt64("bot.account"),
			config.GlobalConfig.GetString("bot.password"),
		),
		false,
	}
	Instance.AllowSlider = true
	deviceJson := utils.ReadFile("./device.json")
	if deviceJson == nil {
		logger.Fatal("can not read ./device.json")
	}
	err := client.SystemDeviceInfo.ReadJson(deviceJson)
	if err != nil {
		logger.WithError(err).Fatal("read device.json error")
	}
}

// InitBot 使用 account password 进行初始化账号
func InitBot(account int64, password string) {
	Instance = &Bot{
		client.NewClient(account, password),
		false,
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

func qrcodeLogin() error {
	rsp, err := Instance.FetchQRCode()
	if err != nil {
		return err
	}
	fi, err := qrcode.Decode(bytes.NewReader(rsp.ImageData))
	if err != nil {
		return err
	}
	_ = ioutil.WriteFile("qrcode.png", rsp.ImageData, 0o644)
	defer func() { _ = os.Remove("qrcode.png") }()
	log.Infof("请使用手机QQ扫描二维码 (qrcode.png) : ")
	time.Sleep(time.Second)
	qrcodeTerminal.New().Get(fi.Content).Print()
	s, err := Instance.QueryQRCodeStatus(rsp.Sig)
	if err != nil {
		return err
	}
	prevState := s.State
	for {
		time.Sleep(time.Second)
		s, _ = Instance.QueryQRCodeStatus(rsp.Sig)
		if s == nil {
			continue
		}
		if prevState == s.State {
			continue
		}
		prevState = s.State
		switch s.State {
		case client.QRCodeCanceled:
			fmt.Println("扫码被用户取消.")
			os.Exit(1)
		case client.QRCodeTimeout:
			fmt.Println("二维码过期")
			os.Exit(1)
		case client.QRCodeWaitingForConfirm:
			log.Infof("扫码成功, 请在手机端确认登录.")
		case client.QRCodeConfirmed:
			res, err := Instance.QRCodeLogin(s.LoginInfo)
			if err != nil {
				return err
			}
			return login(res)
		case client.QRCodeImageFetch, client.QRCodeWaitingForScan:
			// ignore
		}
	}
}

// Login 登录
func Login() {
	resp, err := Instance.Login()
	if err != nil {
		logger.Fatal("login failed: %v", err)
	}

	err = login(resp)
	logger.Fatal("login failed: %v", err)

	if err != nil {
		logger.Fatal("login failed: %v", err)
	} else {
		logger.Infof("bot login: %s", Instance.Nickname)
	}
}

func login(resp *client.LoginResponse) error {
	console := bufio.NewReader(os.Stdin)
	var err error

	for {
		if err != nil {
			return err
		}
		if resp.Success {
			return nil
		}

		var text string
		switch resp.Error {

		case client.NeedCaptcha:
			img, _, _ := image.Decode(bytes.NewReader(resp.CaptchaImage))
			fmt.Println(asc2art.New("image", img).Art)
			fmt.Print("please input captcha: ")
			text, _ := console.ReadString('\n')
			resp, err = Instance.SubmitCaptcha(strings.ReplaceAll(text, "\n", ""), resp.CaptchaSign)
			continue

		case client.UnsafeDeviceError:
			fmt.Printf("device lock -> %v\n", resp.VerifyUrl)
			os.Exit(4)

		case client.SMSNeededError:
			fmt.Println("device lock enabled, Need SMS Code")
			fmt.Printf("Send SMS to %s ? (yes)", resp.SMSPhone)
			t, _ := console.ReadString('\n')
			t = strings.TrimSpace(t)
			if t != "yes" {
				os.Exit(2)
			}
			if !Instance.RequestSMS() {
				logger.Warnf("unable to request SMS Code")
				os.Exit(2)
			}
			logger.Warn("please input SMS Code: ")
			text, _ = console.ReadString('\n')
			resp, err = Instance.SubmitSMS(strings.ReplaceAll(strings.ReplaceAll(text, "\n", ""), "\r", ""))
			continue

		case client.TooManySMSRequestError:
			fmt.Printf("too many SMS request, please try later.\n")
			os.Exit(6)

		case client.SMSOrVerifyNeededError:
			fmt.Println("device lock enabled, choose way to verify:")
			fmt.Println("1. Send SMS Code to ", resp.SMSPhone)
			fmt.Println("2. Scan QR Code")
			fmt.Print("input (1,2):")
			text, _ = console.ReadString('\n')
			text = strings.TrimSpace(text)
			switch text {
			case "1":
				if !Instance.RequestSMS() {
					logger.Warnf("unable to request SMS Code")
					os.Exit(2)
				}
				logger.Warn("please input SMS Code: ")
				text, _ = console.ReadString('\n')
				resp, err = Instance.SubmitSMS(strings.ReplaceAll(strings.ReplaceAll(text, "\n", ""), "\r", ""))
				continue
			case "2":
				fmt.Printf("device lock -> %v\n", resp.VerifyUrl)
				os.Exit(2)
			default:
				fmt.Println("invalid input")
				os.Exit(2)
			}

		case client.SliderNeededError:
			// code below copyright by https://github.com/Mrs4s/go-cqhttp
			fmt.Println("登录需要滑条验证码. ")
			fmt.Println("请参考文档 -> https://docs.go-cqhttp.org/faq/slider.html <- 进行处理")
			fmt.Println("1. 自行抓包并获取 Ticket 输入.")
			fmt.Println("2. 使用手机QQ扫描二维码登入. (推荐)")
			text, _ = console.ReadString('\n')
			if strings.Contains(text, "1") {
				fmt.Printf("\n请用浏览器打开 -> %v <- 并获取Ticket.\n", resp.VerifyUrl)
				fmt.Printf("请输入Ticket： (Enter 提交)")
				text, _ := console.ReadString('\n')
				resp, err = Instance.SubmitTicket(strings.ReplaceAll(text, "\n", ""))
				continue
			}
			Instance.Disconnect()
			Instance.QQClient = client.NewClientEmpty()
			return qrcodeLogin()
		case client.OtherLoginError, client.UnknownLoginError:
			logger.Fatalf("login failed: %v", resp.ErrorMessage)
			os.Exit(3)
		}
	}
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
