package httpsrv

import (
	"subexport/internal/logs"

	"github.com/oklog/ulid/v2"
)

func AddNewSubItem(url, name, date, content string, chanid, msgid int64) error {
	item := &SubItem{
		ChannelUrl:  url,
		ChannelName: name,
		PubDate:     date,
		MsgContent:  content,
		ChannelID:   chanid,
		Msgid:       msgid,
	}

	rid := ulid.Make().String()
	if err := addItem(rid, item); err != nil {
		logs.Warn(err).Rid(rid).Int64("msgid", msgid).Str("channel", url).Msg("add item fail")
		return err
	}
	logs.Info().Rid(rid).Int64("msgid", msgid).Str("channel", url).Msg("add item succ")
	return nil
}
