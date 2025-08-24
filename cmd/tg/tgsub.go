package tg

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"tgfreesub/cmd/httpsrv"
	"tgfreesub/internal/logs"
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

type TgSuber struct {
	AppID         int
	AppHash       string
	Phone         string
	ChannelNames  []string
	SessionPath   string
	GetHistoryCnt int

	client *telegram.Client
}

func (ts *TgSuber) Start() {
	logs.Info().Int("appid", ts.AppID).Str("apphash", ts.AppHash).Strs("channel", ts.ChannelNames).Send()
	// 会话存储
	sessionStorage := &session.FileStorage{Path: ts.SessionPath}

	ts.client = telegram.NewClient(ts.AppID, ts.AppHash, telegram.Options{
		SessionStorage: sessionStorage,
	})
	if err := ts.client.Run(context.Background(), func(ctx context.Context) error {
		return ts.handle(ctx)
	}); err != nil {
		logs.Panic(err).Send()
	}
}

func (ts *TgSuber) handle(ctx context.Context) error {
	if err := ts.login(ctx); err != nil {
		return err
	}
	// 获取当前用户信息，拿到 self ID
	self, err := ts.client.Self(ctx)
	if err != nil {
		return fmt.Errorf("获取 用户信息 失败: %w", err)
	}

	logs.Info().Str("firstname", self.FirstName).Str("username", self.Username).
		Int64("id", self.ID).Int64("accesshash", self.AccessHash).
		Bool("bot", self.Bot).Str("phone", ts.Phone).
		Int("appid", ts.AppID).Msg("ready")

	cs := map[int64]SubChannelInfo{}
	for _, name := range ts.ChannelNames {
		sci := ts.resolveUsername(ctx, name)
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
	wg.Add(len(ts.ChannelNames))
	for _, sci := range cs {
		go func() {
			if ts.GetHistoryCnt > 0 {
				ts.recvChannelHistoryMsg(ctx, &sci)
			}
			ts.recvChannelDiffMsg(ctx, &sci)
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
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
		httpsrv.AddNewSubItem(sci.Name, sci.Title, int64(msg.Date), msg.Message, sci.ChannelID, int64(msg.ID))
	default:
		logs.Info().Bool("ispost", msg.Post).Int("msgid", msg.ID).
			Str("FromID", fromid).
			// Str("Media", media).
			Str("date", dateStr).
			Str("content", msg.Message).
			Msg(sci.Title)
	}

}

func (ts *TgSuber) login(ctx context.Context) error {
	tsca := ts.client.Auth()
	// 尝试从会话恢复
	status, err := tsca.Status(ctx)
	if err != nil {
		logs.Warn(err).Msg("client.Auth.Status fail")
		return err
	}

	if status.Authorized {
		return nil
	}

	// 发送验证码请求
	sentCode, err := tsca.SendCode(ctx, ts.Phone, auth.SendCodeOptions{})
	if err != nil {
		logs.Error(err).Msg("client.SendCode fail")
		return err
	}
	// 类型断言获取PhoneCodeHash
	codeSent, ok := sentCode.(*tg.AuthSentCode)
	if !ok {
		logs.Error(nil).Msg("invalid tg.AuthSentCode")
		return err
	}

	for {
		// 从标准输入读取验证码
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("请输入收到的验证码: ")
		code, err := reader.ReadString('\n')
		if err != nil {
			logs.Error(err).Msg("读取验证码失败")
			continue
		}
		code = strings.TrimSpace(code)

		// 验证登录
		if _, err := tsca.SignIn(ctx, ts.Phone, code, codeSent.PhoneCodeHash); err != nil {
			logs.Error(err).Str("code", code).Msg("登陆失败")
			continue
		}
		break
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

func (ts *TgSuber) recvChannelHistoryMsg(ctx context.Context, sci *SubChannelInfo) {
	api := ts.client.API()

	peer := &tg.InputPeerChannel{
		ChannelID:  sci.ChannelID,
		AccessHash: sci.AccessHash,
	}

	// 建立上下文（必须要调一次，不然不会推送消息）
	_, err := api.ChannelsGetFullChannel(ctx, &tg.InputChannel{
		ChannelID:  peer.ChannelID,
		AccessHash: peer.AccessHash,
	})
	if err != nil {
		logs.Error(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("ChannelsGetFullChannel fail")
		return
	}

	history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: ts.GetHistoryCnt,
	})
	if err != nil {
		logs.Warn(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("MessagesGetHistory fail")
	}

	msgs, ok := history.(*tg.MessagesChannelMessages)
	if !ok {
		logs.Debug().Str("channel", sci.Name).Str("title", sci.Title).Msgf("recv non-channel-msg:%+v", msgs)
		return
	}

	logs.Debug().Int("msgs.Messages.size", len(msgs.Messages))
	for _, m := range msgs.Messages {
		if msg, ok := m.(*tg.Message); ok {
			sci.showTgMessage(msg)
		}
	}
}

func (ts *TgSuber) recvChannelDiffMsg(ctx context.Context, sci *SubChannelInfo) {
	api := ts.client.API()

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
		logs.Error(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("ChannelsGetFullChannel fail")
		return
	}

	chatFull, ok := full.FullChat.(*tg.ChannelFull)
	if !ok {
		logs.Error(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("FullChat fail")
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
				logs.Warn(err).Str("channel", sci.Name).Str("title", sci.Title).Msg("UpdatesGetChannelDifference fail")
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

func (ts *TgSuber) resolveUsername(ctx context.Context, username string) SubChannelInfo {
	api := ts.client.API()
	sci := SubChannelInfo{}

	res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		logs.Warn(err).Str("channel.name", username).Msg("resolve fail")
		return sci
	}

	switch peer := res.Peer.(type) {
	case *tg.PeerChannel:
		tgc := res.Chats[0].(*tg.Channel)
		sci.Name = username
		sci.ChannelID = peer.ChannelID
		sci.AccessHash = tgc.AccessHash
		sci.Title = tgc.Title
	default:
		logs.Warn(err).Str("channel.name", username).Str("peer.type", peer.TypeName()).Msg("unnown peer")
	}
	return sci
}
