package httpsrv

import (
	"strconv"
	"strings"
	"subexport/internal/logs"
	"subexport/internal/redis"
	"time"
)

type SubItem struct {
	ChannelUrl  string `json:"url,omitempty" redis:"url,omitempty"`
	ChannelName string `json:"name,omitempty" redis:"name,omitempty"`
	PubDate     string `json:"date,omitempty" redis:"date,omitempty"`
	MsgContent  string `json:"content,omitempty" redis:"content,omitempty"`
	ChannelID   int64  `json:"chanid,omitempty" redis:"chanid,omitempty"`
	Msgid       int64  `json:"msgid,omitempty" redis:"msgid,omitempty"`
}

const (
	subsIndexKey            = "z_subs_index"
	subsItemKeyPrefix       = "h_subs_item_"
	socreStartOffset  int64 = 1755692698000000
)

var rds *redis.RdsClient

func SubsRedisInit(url string) error {
	var err error
	rds, err = redis.InitRedis(url)
	if err != nil {
		logs.Panic(err).Str("url", url).Msg("SubsRedisInit fail")
		return err
	}
	return nil
}

func addItem(rid string, item *SubItem) error {
	score := time.Now().UnixMicro() - socreStartOffset
	rKey := subsItemKeyPrefix + strconv.FormatInt(score, 10)

	item.MsgContent = strings.ReplaceAll(item.MsgContent, "\n", "</ p>")
	if err := rds.HashSetAll(rKey, item); err != nil {
		logs.Warn(err).Rid(rid).Str("rkey", rKey).Msg("HashSetAll fail")
		return err
	}

	return rds.ZsetAddMember(subsIndexKey, float64(score), score)
}

func getItemsTotal(_ string) int64 {
	return rds.ZsetCard(subsIndexKey)
}

func querySubItems(rid string, cursor, number int64) (int64, []SubItem) {
	if cursor == 0 {
		cursor = int64(^uint64(0) >> 1)
	}
	members := rds.ZsetRangeByScore(subsIndexKey, true, -1, cursor, number)
	if members == nil {
		return cursor, nil
	}

	items := []SubItem{}
	nxt := cursor

	for _, m := range members {
		rKey := subsItemKeyPrefix + m
		item := SubItem{}

		if err := rds.HashGetAll(rKey, &item); err != nil {
			logs.Warn(err).Rid(rid).Str("rkey", rKey).Msg("HashGetAll fail")
		} else {
			logs.Debug().Rid(rid).Str("chan", item.ChannelUrl).Int64("msgid", item.Msgid).Send()
			item.ChannelUrl = "t.me/" + item.ChannelUrl
			items = append(items, item)
		}
	}

	if len(members) > 0 {
		nxt, _ = strconv.ParseInt(members[len(members)-1], 10, 64)
	}
	return nxt, items
}
