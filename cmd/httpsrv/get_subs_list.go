package httpsrv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"subexport/cmd/store"
	"subexport/internal/logs"

	"github.com/oklog/ulid/v2"
)

type SubsListReq struct {
	offset int64
	number int64
}
type SubsListResp struct {
	Rtn   int    `json:"rtn"`
	Msg   string `json:"msg,omitempty"`
	Total int64  `json:"total"`
	// Number int       `json:"number"`
	Offset int64           `json:"offset"`
	Itmes  []store.SubItem `json:"items,omitempty"`
}

// GET /subs/list?offset=0&number=10
// offset=0 表示查询最新消息
// 接口支持分页查询，响应中的offset为下一页的起始位置
// 当响应中的offset < 0时表示已经查询完成
func HndSubsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		// logs.Warn(nil).Str("method", r.Method).Msg("unsupport")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()

	req := SubsListReq{
		offset: 0,
		number: 20,
	}

	q := r.URL.Query()
	offsetStr := q.Get("offset")
	numberStr := q.Get("number")

	if offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &req.offset)
	}
	if numberStr != "" {
		fmt.Sscanf(numberStr, "%d", &req.number)
	}

	logs.Info().Rid(rid).Int64("offset", req.offset).Int64("number", req.number).Str(r.Method, r.URL.Path).Send()

	resp := &SubsListResp{
		Rtn:    0,
		Msg:    "succ",
		Offset: -1,
		Total:  store.GetItemsTotal(rid),
	}

	nxt, items := store.QuerySubItems(rid, req.offset, req.number)
	if items != nil {
		resp.Offset = nxt
		resp.Itmes = items
	}

	replyJson(w, rid, resp)
}

func replyJson(w http.ResponseWriter, _ string, j any) error {
	// logs.Debug().Rid(rid).Msgf("resp:%+v", j)
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(j)
}
