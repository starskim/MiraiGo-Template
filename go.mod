module github.com/Sora233/MiraiGo-Template

go 1.15

require (
	github.com/Baozisoftware/qrcode-terminal-go v0.0.0-20170407111555-c0650d8dff0f
	github.com/Logiase/MiraiGo-Template v0.0.0-20210228150851-29092d4d5486
	github.com/Mrs4s/MiraiGo v0.0.0-20210810070836-6614d2383adb
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lestrrat-go/strftime v1.0.5 // indirect
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sirupsen/logrus v1.8.0
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e // indirect
	github.com/spf13/viper v1.7.1
	github.com/tuotoo/qrcode v0.0.0-20190222102259-ac9c44189bf2
	github.com/willf/bitset v1.2.0 // indirect
	github.com/yinghau76/go-ascii-art v0.0.0-20190517192627-e7f465a30189
)

replace github.com/Logiase/MiraiGo-Template => ./

replace github.com/Logiase/MiraiGo-Template/bot => ./bot

replace github.com/Logiase/MiraiGo-Template/modules => ./modules

replace github.com/Logiase/MiraiGo-Template/config => ./config

replace github.com/Logiase/MiraiGo-Template/utils => ./utils

replace github.com/willf/bitset v1.2.0 => github.com/bits-and-blooms/bitset v1.2.0
