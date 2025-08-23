package httpsrv

import "net/http"

func StartHttpSrv(addr string) error {

	// http.HandleFunc("/", HndSubsList)
	http.HandleFunc("/subs/list", HndSubsList)

	return http.ListenAndServe(addr, nil)
}
