package httpsrv

import (
	"errors"
	"strings"
	"subexport/cmd/store"
	"subexport/internal/logs"

	"github.com/oklog/ulid/v2"
)

var errItemFiltered = errors.New("item filtered")

func AddNewSubItem(url, name string, date int64, content string, chanid, msgid int64) error {
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
	logs.Info().Rid(rid).Int64("msgid", msgid).Str("channel", url).Msg("add item succ")
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
