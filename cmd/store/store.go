package store

import (
	"fmt"
	"strings"
	"tgfreesub/internal/logs"
	"tgfreesub/internal/redis"
)

type SubItem struct {
	ChannelUrl  string `json:"url,omitempty" redis:"url,omitempty"`
	ChannelName string `json:"name,omitempty" redis:"name,omitempty"`
	PubDate     int64  `json:"date,omitempty" redis:"date,omitempty"`
	MsgContent  string `json:"content,omitempty" redis:"content,omitempty"`
	ChannelID   int64  `json:"chanid,omitempty" redis:"chanid,omitempty"`
	Msgid       int64  `json:"msgid,omitempty" redis:"msgid,omitempty"`
	// Score       int64  `json:"-,omitempty" redis:"score,omitempty"`
}

const (
	subsIndexKey            = "z_subs_index_v3"
	subsItemKeyPrefix       = "h_subs_item_"
	socreStartOffset  int64 = 1755692698000000
)

var rds *redis.RdsClient

func StoreInit(url string) error {
	var err error
	rds, err = redis.InitRedis(url)
	if err != nil {
		logs.Panic(err).Str("url", url).Msg("SubsRedisInit fail")
		return err
	}
	return nil
}

func (item *SubItem) calcScore() int64 {
	// 相对于date -d '2024-1-1 0:0:0' +%s 做偏移
	return (((item.PubDate - 1704038400) << 31) | item.Msgid)
}

func AddItem(rid string, item *SubItem) error {
	// score := time.Now().UnixMicro() - socreStartOffset
	score := item.calcScore()
	member := fmt.Sprintf("%s_%d", item.ChannelUrl, item.Msgid)
	rKey := subsItemKeyPrefix + member

	if rds.ZsetIsMember(subsIndexKey, member) {
		logs.Trace().Rid(rid).Str("subsIndexKey", subsIndexKey).Str("member", member).Msg("had recored")
		return nil
	}

	item.MsgContent = strings.ReplaceAll(item.MsgContent, "\n", "</ p>")
	if err := rds.HashSetAll(rKey, item); err != nil {
		logs.Warn(err).Rid(rid).Str("rkey", rKey).Msg("HashSetAll fail")
		return err
	}

	logs.Info().Rid(rid).Str("subsIndexKey", subsIndexKey).Str("member", member).Int64("score", score).Msg("add new record")
	return rds.ZsetAddMember(subsIndexKey, float64(score), member)
}

func GetItemsTotal(_ string) int64 {
	return rds.ZsetCard(subsIndexKey)
}

func QuerySubItems(rid string, cursor, number int64) (int64, []SubItem) {
	if cursor == 0 {
		cursor = int64(^uint64(0) >> 1)
	}
	members := rds.ZsetRangeByScore(subsIndexKey, true, -1, cursor, number)
	if members == nil {
		return cursor, nil
	}

	items := []SubItem{}

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

	var nxt int64 = -1
	if len(items) > 0 {
		nxt = items[len(items)-1].calcScore()
	}
	return nxt, items
}
