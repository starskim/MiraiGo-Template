module github.com/Sora233/MiraiGo-Template

go 1.15

require (
	github.com/Logiase/MiraiGo-Template v0.0.0-20210228150851-29092d4d5486
	github.com/Mrs4s/MiraiGo v0.0.0-20210611062429-4f967b0a6264
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lestrrat-go/strftime v1.0.4 // indirect
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sirupsen/logrus v1.8.0
	github.com/spf13/viper v1.7.1
	github.com/yinghau76/go-ascii-art v0.0.0-20190517192627-e7f465a30189
)

replace github.com/Logiase/MiraiGo-Template => ./

replace github.com/Logiase/MiraiGo-Template/bot => ./bot

replace github.com/Logiase/MiraiGo-Template/modules => ./modules

replace github.com/Logiase/MiraiGo-Template/config => ./config

replace github.com/Logiase/MiraiGo-Template/utils => ./utils
