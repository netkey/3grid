package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"io"
	"net/http"
)

type HTTP_worker struct {
	Id       int
	Ipdb     *IP.IP_db
	Rtdb     *RT.Route_db
	Handlers map[string]interface{}
}

func (h *HTTP_worker) Init() {
	h.Handlers = make(map[string]interface{})
	h.Handlers["/dns"] = h.Dns
}

func (h *HTTP_worker) Dns(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello, http!\n")
}

func Working(listen string, port string, num int, ipdb *IP.IP_db, rtdb *RT.Route_db) {

	worker := HTTP_worker{}
	worker.Id = num
	worker.Ipdb = ipdb
	worker.Rtdb = rtdb
	worker.Init()

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HTTP working: %s", pan)
		}
	}()

	for location, handler := range worker.Handlers {
		http.HandleFunc(location, handler.(func(http.ResponseWriter, *http.Request)))
	}

	if err := http.ListenAndServe(listen+":"+port, nil); err != nil {
		G.OutDebug("Failed to setup the http server: %s", err)
	}
}
