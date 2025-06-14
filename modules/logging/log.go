package logging

import (
	"sync"

	"github.com/LagrangeDev/LagrangeGo/client"
	"github.com/LagrangeDev/LagrangeGo/client/event"
	"github.com/LagrangeDev/LagrangeGo/message"

	"github.com/Sora233/MiraiGo-Template/bot"
	"github.com/Sora233/MiraiGo-Template/utils"
)

func init() {
	instance = &logging{}
	bot.RegisterModule(instance)
}

type logging struct {
}

func (m *logging) MiraiGoModule() bot.ModuleInfo {
	return bot.ModuleInfo{
		ID:       "internal.logging",
		Instance: instance,
	}
}

func (m *logging) Init() {
	// 初始化过程
	// 在此处可以进行 Module 的初始化配置
	// 如配置读取
}

func (m *logging) PostInit() {
	// 第二次初始化
	// 再次过程中可以进行跨Module的动作
	// 如通用数据库等等
}

func (m *logging) Serve(b *bot.Bot) {
	// 注册服务函数部分
	registerLog(b)
}

func (m *logging) Start(b *bot.Bot) {
	// 此函数会新开携程进行调用
	// ```go
	// 		go exampleModule.Start()
	// ```

	// 可以利用此部分进行后台操作
	// 如http服务器等等
}

func (m *logging) Stop(b *bot.Bot, wg *sync.WaitGroup) {
	// 别忘了解锁
	defer wg.Done()
	// 结束部分
	// 一般调用此函数时，程序接收到 os.Interrupt 信号
	// 即将退出
	// 在此处应该释放相应的资源或者对状态进行保存
}

var instance *logging

var logger = utils.GetModuleLogger("internal.logging")

func logGroupMessage(msg *message.GroupMessage) {
	logger.
		WithField("from", "GroupMessage").
		WithField("MessageID", msg.ID).
		WithField("MessageIID", msg.InternalID).
		WithField("GroupCode", msg.GroupUin).
		WithField("SenderUin", msg.Sender.Uin).
		WithField("SenderUID", msg.Sender.UID).
		Info(msg.ToString())
}

func logPrivateMessage(msg *message.PrivateMessage) {
	logger.
		WithField("from", "PrivateMessage").
		WithField("MessageID", msg.ID).
		WithField("MessageIID", msg.InternalID).
		WithField("SenderUin", msg.Sender.Uin).
		WithField("SenderUID", msg.Sender.UID).
		WithField("Target", msg.Target).
		Info(msg.ToString())
}

func logFriendMessageRecallEvent(event *event.FriendRecall) {
	logger.
		WithField("from", "FriendsMessageRecall").
		WithField("FromUin", event.FromUin).
		WithField("Sequence", event.Sequence).
		WithField("FromUID", event.FromUID).
		Info("friend message recall")
}

func logGroupMessageRecallEvent(event *event.GroupRecall) {
	logger.
		WithField("from", "GroupMessageRecall").
		WithField("OperatorUID", event.OperatorUID).
		WithField("OperatorUin", event.OperatorUin).
		WithField("GroupCode", event.GroupUin).
		Info("group message recall")
}

func logGroupMuteEvent(event *event.GroupMute) {
	logger.
		WithField("from", "GroupMute").
		WithField("GroupCode", event.GroupUin).
		WithField("GroupUserUin", event.GroupEvent.UserUin).
		WithField("GroupUserUID", event.GroupEvent.UserUID).
		WithField("OperatorUID", event.OperatorUID).
		WithField("OperatorUin", event.OperatorUin).
		WithField("MuteTime", event.Duration).
		Info("group mute")
}

func logDisconnect(event *client.DisconnectedEvent) {
	logger.
		WithField("from", "Disconnected").
		WithField("reason", event.Message).
		Warn("bot disconnected")
}

func registerLog(b *bot.Bot) {
	b.GroupRecallEvent.Subscribe(func(qqClient *client.QQClient, event *event.GroupRecall) {
		logGroupMessageRecallEvent(event)
	})

	b.GroupMessageEvent.Subscribe(func(qqClient *client.QQClient, groupMessage *message.GroupMessage) {
		logGroupMessage(groupMessage)
	})

	b.GroupMuteEvent.Subscribe(func(qqClient *client.QQClient, event *event.GroupMute) {
		logGroupMuteEvent(event)
	})

	b.PrivateMessageEvent.Subscribe(func(qqClient *client.QQClient, privateMessage *message.PrivateMessage) {
		logPrivateMessage(privateMessage)
	})

	b.FriendRecallEvent.Subscribe(func(qqClient *client.QQClient, event *event.FriendRecall) {
		logFriendMessageRecallEvent(event)
	})

	b.DisconnectedEvent.Subscribe(func(qqClient *client.QQClient, event *client.DisconnectedEvent) {
		logDisconnect(event)
	})
}
