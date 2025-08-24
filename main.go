package main

import (
	"embed"
	"subexport/cmd/httpsrv"
	"subexport/cmd/store"
	"subexport/cmd/tg"
	"subexport/internal/utils"
)

//go:embed static/*
var embeddedStaticFiles embed.FS

func main() {
	ts := &tg.TgSuber{}

	ts.AppID = utils.XmArgValInt("appid", "https://core.telegram.org/api/obtaining_api_id", 0)
	ts.AppHash = utils.XmArgValString("apphash", "", "")
	ts.Phone = utils.XmArgValString("phone", "your login phone number", "")
	ts.ChannelNames = utils.XmArgValStrings("names", "channel names", "")
	ts.SessionPath = utils.XmArgValString("session", "session file", "./session.json")
	ts.GetHistoryCnt = utils.XmArgValInt("history", "get history msg count", 0)
	rdsAddr := utils.XmArgValString("redis", "redis-server addr", "redis://127.0.0.1:6379/0")
	httpAddr := utils.XmArgValString("server", "http server listen addr", "127.0.0.1:2010")
	// modeListChannels := utils.XmArgValBool("list-channels", "list all channels")

	utils.XmLogsInit("./logs/subexport.log", 0, 50<<20, 1) // 设置日志级别为0(DEBUG)

	utils.XmUsageIfHasKeys("h", "help")
	utils.XmUsageIfHasNoKeys("appid", "apphash", "names", "phone")

	store.StoreInit(rdsAddr)

	go httpsrv.StartHttpSrv(embeddedStaticFiles, httpAddr)
	go ts.Start()

	select {}
}
