package bot

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/Mrs4s/MiraiGo/utils"
	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func energy(uin uint64, id string, _ string, salt []byte) ([]byte, error) {
	signServer := config.SignServer
	if !strings.HasSuffix(signServer, "/") {
		signServer += "/"
	}
	query := fmt.Sprintf("?data=%v&salt=%v&uin=%v&android_id=%v&guid=%v",
		id, hex.EncodeToString(salt), uin, utils.B2S(deviceInfo.AndroidId), hex.EncodeToString(deviceInfo.Guid))
	if config.IsBelow110 {
		query = fmt.Sprintf("?data=%v&salt=%v", id, hex.EncodeToString(salt))
	}
	resp, err := http.Get(signServer + "custom_energy" + query)
	signServerBearer := config.SignServerBearer
	if signServerBearer != "" {
		resp.Header.Set("Authorization", "Bearer "+signServerBearer)
	}
	if err != nil {
		logger.Warnf("获取T544 sign时出现错误: %v server: %v", err, signServer)
		return nil, err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnf("获取T544 sign时出现错误: %v server: %v", err, signServer)
		return nil, err
	}
	data, err := hex.DecodeString(gjson.GetBytes(response, "data").String())
	if err != nil {
		logger.Warnf("获取T544 sign时出现错误: %v", err)
		return nil, err
	}
	if len(data) == 0 {
		logger.Warnf("获取T544 sign时出现错误: %v", "data is empty")
		return nil, errors.New("data is empty")
	}
	return data, nil
}

// signSubmit 提交的操作类型
func signSubmit(uin string, cmd string, callbackID int64, buffer []byte, t string) {
	signServer := config.SignServer
	if !strings.HasSuffix(signServer, "/") {
		signServer += "/"
	}
	buffStr := hex.EncodeToString(buffer)
	logger.Infof("submit %v: uin=%v, cmd=%v, callbackID=%v, buffer-end=%v", t, uin, cmd, callbackID,
		buffStr[len(buffStr)-10:])
	_, err := http.Get(signServer + "submit" + fmt.Sprintf("?uin=%v&cmd=%v&callback_id=%v&buffer=%v",
		uin, cmd, callbackID, buffStr))
	if err != nil {
		logger.Warnf("提交 callback 时出现错误: %v server: %v", err, signServer)
	}
}

// signCallback request token 和签名的回调
func signCallback(uin string, results []gjson.Result, t string) {
	for _, result := range results {
		cmd := result.Get("cmd").String()
		callbackID := result.Get("callbackId").Int()
		body, _ := hex.DecodeString(result.Get("body").String())
		ret, err := Instance.SendSsoPacket(cmd, body)
		if err != nil {
			logger.Warnf("callback error: %v", err)
		}
		signSubmit(uin, cmd, callbackID, ret, t)
	}
}

func signRequset(seq uint64, uin string, cmd string, qua string, buff []byte) (sign []byte, extra []byte, token []byte, err error) {
	signServer := config.SignServer
	if !strings.HasSuffix(signServer, "/") {
		signServer += "/"
	}
	req, err := http.NewRequest(http.MethodPost, signServer+"sign", bytes.NewReader([]byte(fmt.Sprintf("uin=%v&qua=%s&cmd=%s&seq=%v&buffer=%v&android_id=%v&guid=%v",
		uin, qua, cmd, seq, hex.EncodeToString(buff), utils.B2S(deviceInfo.AndroidId), hex.EncodeToString(deviceInfo.Guid)))))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	signServerBearer := config.SignServerBearer
	if signServerBearer != "" {
		req.Header.Set("Authorization", "Bearer "+signServerBearer)
	}
	req.Body = io.NopCloser(bytes.NewReader([]byte(fmt.Sprintf("uin=%v&qua=%s&cmd=%s&seq=%v&buffer=%v&android_id=%v&guid=%v",
		uin, qua, cmd, seq, hex.EncodeToString(buff), utils.B2S(deviceInfo.AndroidId), hex.EncodeToString(deviceInfo.Guid)))))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, err
	}
	sign, _ = hex.DecodeString(gjson.GetBytes(response, "data.sign").String())
	extra, _ = hex.DecodeString(gjson.GetBytes(response, "data.extra").String())
	token, _ = hex.DecodeString(gjson.GetBytes(response, "data.token").String())
	if !config.IsBelow110 {
		go signCallback(uin, gjson.GetBytes(response, "data.requestCallback").Array(), "sign")
	}
	return sign, extra, token, nil
}

var registerLock sync.Mutex

func signRegister(uin int64, androidID, guid []byte, qimei36, key string) {
	if config.IsBelow110 {
		logger.Warn("签名服务器版本低于1.1.0, 跳过实例注册")
		return
	}
	signServer := config.SignServer
	if !strings.HasSuffix(signServer, "/") {
		signServer += "/"
	}
	req, err := http.Get(signServer + "register" + fmt.Sprintf("?uin=%v&android_id=%v&guid=%v&qimei36=%v&key=%s",
		uin, utils.B2S(androidID), hex.EncodeToString(guid), qimei36, key))
	if err != nil {
		logger.Warnf("注册QQ实例时出现错误: %v server: %v", err, signServer)
		return
	}
	defer req.Body.Close()
	resp, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Warnf("注册QQ实例时出现错误: %v server: %v", err, signServer)
		return
	}
	msg := gjson.GetBytes(resp, "msg")
	if gjson.GetBytes(resp, "code").Int() != 0 {
		logger.Warnf("注册QQ实例时出现错误: %v server: %v", msg, signServer)
		return
	}
	logger.Infof("注册QQ实例 %v 成功: %v", uin, msg)
}

func signRefreshToken(uin string) error {
	signServer := config.SignServer
	if !strings.HasSuffix(signServer, "/") {
		signServer += "/"
	}
	logger.Info("正在刷新 token")
	req, err := http.Get(signServer + "request_token" + fmt.Sprintf("?uin=%v", uin))
	if err != nil {
		return err
	}
	defer req.Body.Close()
	resp, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	msg := gjson.GetBytes(resp, "msg")
	if gjson.GetBytes(resp, "code").Int() != 0 {
		return errors.New(msg.String())
	}
	go signCallback(uin, gjson.GetBytes(resp, "data").Array(), "request token")
	return nil
}

var missTokenCount = uint64(0)

func sign(seq uint64, uin string, cmd string, qua string, buff []byte) (sign []byte, extra []byte, token []byte, err error) {
	i := 0
	for {
		sign, extra, token, err = signRequset(seq, uin, cmd, qua, buff)
		if err != nil {
			logger.Warnf("获取sso sign时出现错误: %v server: %v", err, config.SignServer)
		}
		if i > 0 {
			break
		}
		i++
		if (!config.IsBelow110) && config.Sign.AutoRegister && err == nil && len(sign) == 0 {
			if registerLock.TryLock() { // 避免并发时多处同时销毁并重新注册
				logger.Warn("获取签名为空，实例可能丢失，正在尝试重新注册")
				defer registerLock.Unlock()
				err := signServerDestroy(uin)
				if err != nil {
					logger.Warnln(err)
					return nil, nil, nil, err
				}
				signRegister(config.Bot.Account, deviceInfo.AndroidId, deviceInfo.Guid, deviceInfo.QImei36, config.Key)
			}
			continue
		}
		if (!config.IsBelow110) && config.Sign.AutoRefreshToken && len(token) == 0 {
			logger.Warnf("token 已过期, 总丢失 token 次数为 %v", atomic.AddUint64(&missTokenCount, 1))
			if registerLock.TryLock() {
				defer registerLock.Unlock()
				if err := signRefreshToken(uin); err != nil {
					logger.Warnf("刷新 token 出现错误: %v server: %v", err, config.SignServer)
				} else {
					logger.Info("刷新 token 成功")
				}
			}
			continue
		}
		break
	}
	return sign, extra, token, err
}

func signServerDestroy(uin string) error {
	signServer := config.SignServer
	if !strings.HasSuffix(signServer, "/") {
		signServer += "/"
	}
	signVersion, err := signVersion()
	if err != nil {
		return errors.Wrapf(err, "获取签名服务版本出现错误, server: %v", signServer)
	}
	if "v"+signVersion > "v1.1.6" {
		return errors.Errorf("当前签名服务器版本 %v 低于 1.1.6，无法使用 destroy 接口", signVersion)
	}

	req, err := http.Get(signServer + "destroy" + fmt.Sprintf("?uin=%v&key=%v", uin, config.Key))
	if err != nil {
		return errors.Wrapf(err, "destroy 实例出现错误, server: %v", signServer)
	}
	defer req.Body.Close()
	resp, err := io.ReadAll(req.Body)
	if err != nil || gjson.GetBytes(resp, "code").Int() != 0 {
		return errors.Wrapf(err, "destroy 实例出现错误, server: %v", signServer)
	}
	return nil
}

func signVersion() (version string, err error) {
	signServer := config.SignServer
	req, err := http.Get(signServer)
	if err != nil {
		return "", err
	}
	defer req.Body.Close()
	resp, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	if gjson.GetBytes(resp, "code").Int() == 0 {
		return gjson.GetBytes(resp, "data.version").String(), nil
	}
	return "", errors.New("empty version")
}

// 定时刷新 token, interval 为间隔时间（分钟）
func signStartRefreshToken(interval int64) {
	if interval <= 0 {
		logger.Warn("定时刷新 token 已关闭")
		return
	}
	logger.Infof("每 %v 分钟将刷新一次签名 token", interval)
	if interval < 10 {
		logger.Warnf("间隔时间 %v 分钟较短，推荐 30~40 分钟", interval)
	}
	if interval > 60 {
		logger.Warn("间隔时间不能超过 60 分钟，已自动设置为 60 分钟")
		interval = 60
	}
	t := time.NewTicker(time.Duration(interval) * time.Minute)
	defer t.Stop()
	for range t.C {
		err := signRefreshToken(strconv.FormatInt(config.Bot.Account, 10))
		if err != nil {
			logger.Warnf("刷新 token 出现错误: %v server: %v", err, config.SignServer)
		}
	}
}

func signWaitServer() bool {
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()
	i := 0
	for range t.C {
		if i > 3 {
			return false
		}
		i++
		u, err := url.Parse(config.SignServer)
		if err != nil {
			logger.Warnf("连接到签名服务器出现错误: %v", err)
			continue
		}
		r := utils.RunTCPPingLoop(u.Host, 4)
		if r.PacketsLoss > 0 {
			logger.Warnf("连接到签名服务器出现错误: 丢包%d/%d 时延%dms", r.PacketsLoss, r.PacketsSent, r.AvgTimeMill)
			continue
		}
		break
	}
	logger.Infof("连接至签名服务器: %s", config.SignServer)
	return true
}
