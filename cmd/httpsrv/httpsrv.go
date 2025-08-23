package httpsrv

import (
	"net/http"
	"path/filepath"
	"strings"
	"subexport/internal/logs"
)

func StartHttpSrv(addr string) error {
	// 设置静态文件服务
	staticDir := "."
	fs := http.FileServer(http.Dir(staticDir))

	// 处理根路径，重定向到index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 如果请求的是根路径，返回index.html
		logs.Debug().Str("req", r.URL.Path).Str("from", r.RemoteAddr).Send()
		if r.URL.Path == "/" {
			indexPath := filepath.Join(staticDir, "/static/index.html")
			http.ServeFile(w, r, indexPath)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/static/") {
			fs.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	// 单独处理API接口
	http.HandleFunc("/subs/list", HndSubsList)

	logs.Info().Str("addr", addr).Msg("HTTP server running on")
	// logs.Info().Str("static_dir", staticDir).Msg("静态文件目录")

	return http.ListenAndServe(addr, nil)
}
