package main

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"tgfreesub/cmd/httpsrv"
	"tgfreesub/cmd/store"
	"tgfreesub/cmd/tg"
	"tgfreesub/internal/logs"
	"tgfreesub/internal/utils"
	"time"

	"github.com/oklog/ulid/v2"
)

//go:embed static/*
var embeddedStaticFiles embed.FS

var errItemFiltered = errors.New("item filtered")

func main() {
	appid := utils.XmArgValInt("appid", "https://core.telegram.org/api/obtaining_api_id", 0)
	appHash := utils.XmArgValString("apphash", "", "")
	phone := utils.XmArgValString("phone", "your login phone number", "")
	names := utils.XmArgValStrings("names", "channel names", "")
	sessionPath := utils.XmArgValString("session", "session file", "./session.json")
	getHistoryCnt := utils.XmArgValInt("history", "get history msg count", 0)
	rdsAddr := utils.XmArgValString("redis", "redis-server addr", "redis://127.0.0.1:6379/0")
	httpAddr := utils.XmArgValString("server", "http server listen addr", "127.0.0.1:2010")
	socks5 := utils.XmArgValString("proxy", "proxy url: socks5://127.0.0.1:1080", "")

	utils.XmLogsInit("./logs/tgfreesub.log", 0, 50<<20, 1) // 设置日志级别为0(DEBUG)

	utils.XmUsageIfHasKeys("h", "help")
	utils.XmUsageIfHasNoKeys("appid", "apphash", "names", "phone")

	store.StoreInit(rdsAddr)

	go httpsrv.StartHttpSrv(embeddedStaticFiles, httpAddr)

	ts := tg.NewTG(appid, appHash, phone).
		WithHistoryMsgCnt(getHistoryCnt).
		WithSocks5Proxy(socks5).
		WithSession(sessionPath, inputLoginCode)

	ts.WithMsgHandle(tg.TgNote, func(msgid int, tgmsg *tg.TgMsg) error {
		sci := tgmsg.From
		dateStr := time.Unix(tgmsg.Date, 0).Format(time.DateTime)
		logs.Info().Int("msgid", msgid).Str("content", tgmsg.Text).
			Str("date", dateStr).Str("channel", sci.Name).
			Msg(sci.Title)
		return addNewSubItem(sci.Name, sci.Title, tgmsg.Date, tgmsg.Text, sci.ChannelID, int64(msgid))
	})

	ts.WithMsgHandle(tg.TgPhoto, func(msgid int, tgmsg *tg.TgMsg) error {
		// 有些消息同时包含了照片，所以也要处理照片类型的消息
		if tgmsg.Text == "" {
			return nil
		}
		sci := tgmsg.From
		dateStr := time.Unix(tgmsg.Date, 0).Format(time.DateTime)
		logs.Info().Int("msgid", msgid).Str("content", tgmsg.Text).
			Str("date", dateStr).Str("channel", sci.Name).
			Msg(sci.Title)
		return addNewSubItem(sci.Name, sci.Title, tgmsg.Date, tgmsg.Text, sci.ChannelID, int64(msgid))
	})

	ts.Run(names)
}

func inputLoginCode() string {
	for {
		// 从标准输入读取验证码
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("请输入收到的验证码: ")
		code, err := reader.ReadString('\n')
		if err != nil {
			// logs.Error(err).Msg("读取验证码失败")
			continue
		}
		code = strings.TrimSpace(code)
		if code != "" {
			return code
		}
	}
}

func addNewSubItem(url, name string, date int64, content string, chanid, msgid int64) error {
	item := &store.SubItem{
		ChannelUrl:  url,
		ChannelName: name,
		PubDate:     date,
		MsgContent:  content,
		ChannelID:   chanid,
		Msgid:       msgid,
	}

	if itemFilter(item) {
		return errItemFiltered
	}

	rid := ulid.Make().String()
	if err := store.AddItem(rid, item); err != nil {
		logs.Warn(err).Rid(rid).Int64("msgid", msgid).Str("channel", url).Msg("add item fail")
		return err
	}
	logs.Debug().Rid(rid).Int64("msgid", msgid).Str("channel", url).Msg("add item succ")
	return nil
}

func itemFilter(item *store.SubItem) bool {
	if strings.Contains(item.MsgContent, "机场") ||
		strings.Contains(item.MsgContent, "订阅") ||
		strings.Contains(item.MsgContent, "节点") {
		return false
	}
	return true
}
