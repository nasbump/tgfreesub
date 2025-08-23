package main

import (
	"bufio"
	"context"
	"embed"
	"fmt"
	"os"
	"strings"
	"subexport/cmd/httpsrv"
	"subexport/internal/logs"
	"subexport/internal/utils"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type SubChannelInfo struct {
	Name       string
	Title      string
	ChannelID  int64
	AccessHash int64
	Pts        int // 保存频道的 PTS（状态点）
}

//go:embed static/*
var embeddedStaticFiles embed.FS

func main() {
	appID := utils.XmArgValInt("appid", "https://core.telegram.org/api/obtaining_api_id", 0)
	appHash := utils.XmArgValString("apphash", "", "")
	// channelIDs := utils.XmArgValInt64s("channels", "channel ID", 0)
	phone := utils.XmArgValString("phone", "your login phone number", "")
	channelNames := utils.XmArgValStrings("names", "channel names", "")
	sessPath := utils.XmArgValString("session", "session file", "./session.json")
	rdsAddr := utils.XmArgValString("redis", "redis-server addr", "redis://127.0.0.1:6379/0")
	httpAddr := utils.XmArgValString("server", "http server listen addr", "127.0.0.1:2010")
	// modeListChannels := utils.XmArgValBool("list-channels", "list all channels")

	utils.XmLogsInit("./logs/subexport.log", 0, 50<<20, 1) // 设置日志级别为0(DEBUG)

	utils.XmUsageIfHasKeys("h", "help")
	utils.XmUsageIfHasNoKeys("appid", "apphash", "names", "phone")

	logs.Info().Int("appid", appID).Str("apphash", appHash).Strs("channel", channelNames).Send()
	httpsrv.SubsRedisInit(rdsAddr)
	go httpsrv.StartHttpSrv(embeddedStaticFiles, httpAddr)

	// 会话存储
	sessionStorage := &session.FileStorage{Path: sessPath}

	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	if err := client.Run(context.Background(), func(ctx context.Context) error {
		if err := login(ctx, client, phone); err != nil {
			return err
		}
		// 获取当前用户信息，拿到 self ID
		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("获取 Self 失败: %w", err)
		}
		logs.Info().Str("firstname", self.FirstName).Str("username", self.Username).
			Int64("id", self.ID).Int64("accesshash", self.AccessHash).
			Bool("bot", self.Bot).Bool("self", self.Self).
			Int("appid", appID).Msg("ready")

		api := client.API()

		cs := map[int64]SubChannelInfo{}
		for _, name := range channelNames {
			sci := resolveUsername(ctx, api, name)
			if sci.ChannelID > 0 {
				logs.Info().Str("name", name).Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Send()
				cs[sci.ChannelID] = sci
			}
		}

		// if modeListChannels {
		// 	listAllDialogs(ctx, api)
		// 	return nil
		// }

		// cs := queryPeerChannels(ctx, api, channelIDs)

		wg := sync.WaitGroup{}
		wg.Add(len(channelNames))
		for _, sci := range cs {
			go func() {
				// recvChannelMsg(ctx, api, &tgipc)
				recvChannelDiffMsg(ctx, api, &sci)
				wg.Done()
			}()
		}

		wg.Wait()
		return nil
	}); err != nil {
		logs.Panic(err).Send()
	}
}

func (sci *SubChannelInfo) showTgMessage(msg *tg.Message) {
	fromid := "FromID.nil"
	if msg.FromID != nil {
		fromid = msg.FromID.String()
	}
	// media := "Media.nil"
	// if msg.Media != nil {
	// 	media = msg.Media.String()
	// }

	dateStr := time.Unix(int64(msg.Date), 0).Format(time.DateTime)
	switch peer := msg.PeerID.(type) {
	case *tg.PeerChannel:
		logs.Info().Bool("ispost", msg.Post).Int("msgid", msg.ID).
			Str("FromID", fromid).
			// Str("Media", media).
			Str("content", msg.Message).
			Str("date", dateStr).
			Int64("channelid", peer.ChannelID).
			Msg(sci.Title)
		httpsrv.AddNewSubItem(sci.Name, sci.Title, dateStr, msg.Message, sci.ChannelID, int64(msg.ID))
	default:
		logs.Info().Bool("ispost", msg.Post).Int("msgid", msg.ID).
			Str("FromID", fromid).
			// Str("Media", media).
			Str("date", dateStr).
			Str("content", msg.Message).
			Msg(sci.Title)
	}

}

func login(ctx context.Context, client *telegram.Client, phone string) error {
	// 尝试从会话恢复
	status, err := client.Auth().Status(ctx)
	if err != nil {
		logs.Warn(err).Msg("client.Auth.Status fail")
		return err
	}

	if status.Authorized {
		return nil
	}

	// 发送验证码请求
	sentCode, err := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
	if err != nil {
		logs.Error(err).Str("phone", phone).Msg("client.SendCode fail")
		return err
	}

	// 从标准输入读取验证码
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("请输入收到的验证码: ")
	code, err := reader.ReadString('\n')
	if err != nil {
		logs.Error(err).Msg("读取验证码失败")
		return err
	}
	code = strings.TrimSpace(code)

	// 类型断言获取PhoneCodeHash
	codeSent, ok := sentCode.(*tg.AuthSentCode)
	if !ok {
		logs.Error(nil).Msg("invalid tg.AuthSentCode")
		return err
	}

	// 验证登录
	if _, err := client.Auth().SignIn(ctx, phone, code, codeSent.PhoneCodeHash); err != nil {
		logs.Error(err).Str("phone", phone).Str("code", code).Msg("client.SignIn fail")
		return err
	}
	return nil
}

func listAllDialogs(ctx context.Context, api *tg.Client) map[int64]SubChannelInfo {
	cs := map[int64]SubChannelInfo{}

	limit := 100
	offsetID := 0
	offsetDate := 0
	var offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}

	// 拉取对话列表，找到频道
	for {
		dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      limit,
			OffsetID:   offsetID,
			OffsetDate: offsetDate,
			OffsetPeer: offsetPeer,
		})
		if err != nil {
			logs.Trace().AnErr("err", err).Int("offset", offsetID).Int("channels.cnt", len(cs)).Msg("MessagesGetDialogs fail")
			break
		}

		if d, ok := dialogs.(*tg.MessagesDialogs); ok {
			for _, ch := range d.Chats {
				if c, ok := ch.(*tg.Channel); ok {
					if strings.HasSuffix(c.Title, " Messages") {
						continue
					}
					cs[c.ID] = SubChannelInfo{
						Title:      c.Title,
						ChannelID:  c.ID,
						AccessHash: c.AccessHash,
					}
					logs.Info().Int64("id", c.ID).Int64("hash", c.AccessHash).Str("title", c.Title).Int("channels.cnt", len(cs)).Msg("found channel")
				}
			}
		} else {
			logs.Trace().Int("offset", offsetID).Str("dialogs.type", dialogs.TypeName()).Int("channels.cnt", len(cs)).Msg("skip unknown type")
		}

		dialogsResp, ok := dialogs.(*tg.MessagesDialogsSlice)
		if !ok {
			logs.Trace().Int("offset", offsetID).Str("dialogs.type", dialogs.TypeName()).Int("channels.cnt", len(cs)).Msg("unknown type")
			break
		}
		cnt := len(dialogsResp.Dialogs)
		last := dialogsResp.Dialogs[cnt-1]
		if lt, ok := last.(*tg.Dialog); ok {
			offsetID = lt.TopMessage
		}

		if cnt < limit { // 拉取完成
			logs.Info().Int("offset", offsetID).Int("channels.cnt", len(cs)).Int("channels.cnt", len(cs)).Msg("list-channels done")
			break
		}
	}
	return cs
}

func queryPeerChannels(ctx context.Context, api *tg.Client, channelIDs []int64) map[int64]SubChannelInfo {
	cs := map[int64]SubChannelInfo{}

	for _, cid := range channelIDs {
		cs[cid] = SubChannelInfo{
			ChannelID: cid,
		}
	}

	all := listAllDialogs(ctx, api)
	for _, sci := range all {
		if _, ok := cs[sci.ChannelID]; ok {
			logs.Info().Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Msg("sub channel")
			cs[sci.ChannelID] = sci
		} else {
			logs.Trace().Int64("id", sci.ChannelID).Int64("hash", sci.AccessHash).Str("title", sci.Title).Msg("skip channel")
		}
	}
	return cs
}

// func recvChannelMsg(ctx context.Context, api *tg.Client, peer *tg.InputPeerChannel) {
// 	var lastID int // 记录最后一条消息 ID
// 	ticker := time.NewTicker(10 * time.Second)
// 	defer ticker.Stop()

// 	// 建立上下文（必须要调一次，不然不会推送消息）
// 	_, err := api.ChannelsGetFullChannel(ctx, &tg.InputChannel{
// 		ChannelID:  peer.ChannelID,
// 		AccessHash: peer.AccessHash,
// 	})
// 	if err != nil {
// 		logs.Error(err).Msg("ChannelsGetFullChannel fail")
// 		return
// 	}

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
// 				Peer:  peer,
// 				Limit: 5,
// 			})
// 			if err != nil {
// 				logs.Warn(err).Msg("MessagesGetHistory fail")
// 				continue
// 			}

// 			msgs, ok := history.(*tg.MessagesChannelMessages)
// 			if !ok {
// 				logs.Debug().Msgf("recv non-channel-msg:%+v", msgs)
// 				continue
// 			}

// 			logs.Debug().Int("msgs.Messages.size", len(msgs.Messages))
// 			for _, m := range msgs.Messages {
// 				if msg, ok := m.(*tg.Message); ok {
// 					if msg.ID > lastID {
// 						showTgMessage(msg)
// 						if msg.ID > lastID {
// 							lastID = msg.ID
// 						}
// 					}
// 				} else {
// 					logs.Debug().Msgf("recv non-tg-msg:%+v", msg)
// 				}
// 			}
// 		}
// 	}
// }

func recvChannelDiffMsg(ctx context.Context, api *tg.Client, sci *SubChannelInfo) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	peer := &tg.InputPeerChannel{
		ChannelID:  sci.ChannelID,
		AccessHash: sci.AccessHash,
	}

	// 建立上下文（必须要调一次，不然不会推送消息）
	full, err := api.ChannelsGetFullChannel(ctx, &tg.InputChannel{
		ChannelID:  peer.ChannelID,
		AccessHash: peer.AccessHash,
	})
	if err != nil {
		logs.Error(err).Msg("ChannelsGetFullChannel fail")
		return
	}

	chatFull, ok := full.FullChat.(*tg.ChannelFull)
	if !ok {
		logs.Error(err).Msg("FullChat fail")
		return
	}

	pts := chatFull.Pts

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			diff, err := api.UpdatesGetChannelDifference(ctx, &tg.UpdatesGetChannelDifferenceRequest{
				Channel: &tg.InputChannel{
					ChannelID:  peer.ChannelID,
					AccessHash: peer.AccessHash,
				},
				Filter: &tg.ChannelMessagesFilterEmpty{},
				Pts:    pts,
				Limit:  50,
			})
			if err != nil {
				logs.Warn(err).Int64("channel.id", peer.ChannelID).Msg("UpdatesGetChannelDifference fail")
				continue
			}

			switch upd := diff.(type) {
			case *tg.UpdatesChannelDifference:
				for _, m := range upd.NewMessages {
					if msg, ok := m.(*tg.Message); ok {
						sci.showTgMessage(msg)
					}
				}
				// 更新 PTS
				pts = upd.Pts
			case *tg.UpdatesChannelDifferenceEmpty:
				// 没有新消息，更新 PTS
				pts = upd.Pts
			}
		}
	}
}

func resolveUsername(ctx context.Context, api *tg.Client, username string) SubChannelInfo {
	res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		logs.Warn(err).Str("channel.name", username).Msg("resolve fail")
		return SubChannelInfo{}
	}

	switch peer := res.Peer.(type) {
	case *tg.PeerChannel:
		tgc := res.Chats[0].(*tg.Channel)
		return SubChannelInfo{
			Name:       username,
			ChannelID:  peer.ChannelID,
			AccessHash: tgc.AccessHash,
			Title:      tgc.Title,
		}

	default:
		logs.Warn(err).Str("channel.name", username).Str("peer.type", peer.TypeName()).Msg("unnown peer")
		return SubChannelInfo{}
	}
}
