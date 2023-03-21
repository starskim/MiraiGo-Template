package bot

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Mrs4s/MiraiGo/utils"
	"github.com/Mrs4s/MiraiGo/wrapper"
	"github.com/gocq/qrcode"
	"github.com/guonaihong/gout"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"os"
	"strings"
	"time"
)

var console = bufio.NewReader(os.Stdin)

func init() {
	wrapper.DandelionEnergy = energy
}

func energy(uin uint64, id string, salt []byte) ([]byte, error) {
	// temporary solution
	signServer := "https://captcha.go-cqhttp.org/sdk/dandelion/energy"

	var response []byte
	err := gout.POST(signServer).
		SetHeader(gout.H{"Content-Type": "application/x-www-form-urlencoded"}).
		SetBody([]byte(fmt.Sprintf("uin=%v&id=%s&salt=%s", uin, id, hex.EncodeToString(salt)))).
		BindBody(&response).Do()

	if err != nil {
		logger.Errorf("获取T544时出现问题: %v", err)
		return nil, err
	}
	sign, err := hex.DecodeString(gjson.GetBytes(response, "result").String())
	if err != nil {
		logger.Errorf("获取T544时出现问题: %v", err)
		return nil, err
	}
	return sign, nil
}

func fetchCaptcha(id string) string {
	var b []byte
	err := gout.GET("https://captcha.go-cqhttp.org/captcha/ticket?id=" + id).BindBody(&b).Do()
	//g, err := download.Request{URL: "https://captcha.go-cqhttp.org/captcha/ticket?id=" + id}.JSON()
	if err != nil {
		logger.Debugf("获取 Ticket 时出现错误: %v", err)
		return ""
	}
	if gt := gjson.GetBytes(b, "ticket"); gt.Exists() {
		return gt.String()
	}
	return ""
}

func readLine() (str string) {
	str, _ = console.ReadString('\n')
	str = strings.TrimSpace(str)
	return
}

func readLineTimeout(t time.Duration, de string) (str string) {
	r := make(chan string)
	go func() {
		select {
		case r <- readLine():
		case <-time.After(t):
		}
	}()
	str = de
	select {
	case str = <-r:
	case <-time.After(t):
	}
	return
}

// ErrSMSRequestError SMS请求出错
var ErrSMSRequestError = errors.New("sms request error")

func commonLogin() error {
	res, err := Instance.Login()
	if err != nil {
		return err
	}
	return loginResponseProcessor(res)
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
	_ = os.WriteFile("qrcode.png", rsp.ImageData, 0o644)
	defer func() { _ = os.Remove("qrcode.png") }()
	if Instance.Uin != 0 {
		logger.Infof("请使用账号 %v 登录手机QQ扫描二维码 (qrcode.png) : ", Instance.Uin)
	} else {
		logger.Infof("请使用手机QQ扫描二维码 (qrcode.png) : ")
	}
	time.Sleep(time.Second)
	qrcodeTerminal.New2(qrcodeTerminal.ConsoleColors.BrightBlack, qrcodeTerminal.ConsoleColors.BrightWhite, qrcodeTerminal.QRCodeRecoveryLevels.Low).Get(fi.Content).Print()
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
			logger.Fatalf("扫码被用户取消.")
		case client.QRCodeTimeout:
			logger.Fatalf("二维码过期")
		case client.QRCodeWaitingForConfirm:
			logger.Infof("扫码成功, 请在手机端确认登录.")
		case client.QRCodeConfirmed:
			res, err := Instance.QRCodeLogin(s.LoginInfo)
			if err != nil {
				return err
			}
			return loginResponseProcessor(res)
		case client.QRCodeImageFetch, client.QRCodeWaitingForScan:
			// ignore
		}
	}
}

func getTicket(u string) string {
	logger.Warnf("请选择提交滑块ticket方式:")
	logger.Warnf("1. 自动提交")
	logger.Warnf("2. 手动抓取提交")
	logger.Warn("请输入(1 - 2)：")
	text := readLine()
	id := utils.RandomString(8)
	auto := !strings.Contains(text, "2")
	if auto {
		u = strings.ReplaceAll(u, "https://ssl.captcha.qq.com/template/wireless_mqq_captcha.html?", fmt.Sprintf("https://captcha.go-cqhttp.org/captcha?id=%v&", id))
	}
	logger.Warnf("请前往该地址验证 -> %v ", u)
	if !auto {
		logger.Warn("请输入ticket： (Enter 提交)")
		return readLine()
	}

	for count := 120; count > 0; count-- {
		str := fetchCaptcha(id)
		if str != "" {
			return str
		}
		time.Sleep(time.Second)
	}
	logger.Warnf("验证超时")
	return ""
}

func loginResponseProcessor(res *client.LoginResponse) error {
	var err error
	for {
		if err != nil {
			return err
		}
		if res.Success {
			return nil
		}
		var text string
		switch res.Error {
		case client.SliderNeededError:
			logger.Warnf("登录需要滑条验证码, 请验证后重试.")
			ticket := getTicket(res.VerifyUrl)
			if ticket == "" {
				logger.Infof("按 Enter 继续....")
				readLine()
				os.Exit(0)
			}
			res, err = Instance.SubmitTicket(ticket)
			continue
		case client.NeedCaptcha:
			logger.Warnf("登录需要验证码.")
			_ = os.WriteFile("captcha.jpg", res.CaptchaImage, 0o644)
			logger.Warnf("请输入验证码 (captcha.jpg)： (Enter 提交)")
			text = readLine()
			os.Remove("captcha.jpg")
			res, err = Instance.SubmitCaptcha(text, res.CaptchaSign)
			continue
		case client.SMSNeededError:
			logger.Warnf("账号已开启设备锁, 按 Enter 向手机 %v 发送短信验证码.", res.SMSPhone)
			readLine()
			if !Instance.RequestSMS() {
				logger.Warnf("发送验证码失败，可能是请求过于频繁.")
				return errors.WithStack(ErrSMSRequestError)
			}
			logger.Warn("请输入短信验证码： (Enter 提交)")
			text = readLine()
			res, err = Instance.SubmitSMS(text)
			continue
		case client.SMSOrVerifyNeededError:
			logger.Warnf("账号已开启设备锁，请选择验证方式:")
			logger.Warnf("1. 向手机 %v 发送短信验证码", res.SMSPhone)
			logger.Warnf("2. 使用手机QQ扫码验证.")
			logger.Warn("请输入(1 - 2)：")
			text = readIfTTY("2")
			if strings.Contains(text, "1") {
				if !Instance.RequestSMS() {
					logger.Warnf("发送验证码失败，可能是请求过于频繁.")
					return errors.WithStack(ErrSMSRequestError)
				}
				logger.Warn("请输入短信验证码： (Enter 提交)")
				text = readLine()
				res, err = Instance.SubmitSMS(text)
				continue
			}
			fallthrough
		case client.UnsafeDeviceError:
			logger.Warnf("账号已开启设备锁，请前往 -> %v <- 验证后重启Bot.", res.VerifyUrl)
			logger.Infof("按 Enter 或等待 5s 后继续....")
			readLineTimeout(time.Second*5, "")
			os.Exit(0)
		case client.OtherLoginError, client.UnknownLoginError, client.TooManySMSRequestError:
			msg := res.ErrorMessage
			logger.Warnf("登录失败: %v Code: %v", msg, res.Code)
			if res.Code == 235 {
				logger.Warnf("请删除 device.json 后重试.")
			}
			logger.Infof("按 Enter 继续....")
			readLine()
			os.Exit(0)
		}
	}
}
