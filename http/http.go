package grid_http

import (
	G "3grid/tools/globals"
	"github.com/kavu/go_reuseport"
	"io"
	"net/http"
)

var Handlers map[string]interface{}

func InitHandlers() {
	Handlers = make(map[string]interface{})

	Handlers["/dns"] = HttpDns

	for location, handler := range Handlers {
		http.HandleFunc(location, handler.(func(http.ResponseWriter, *http.Request)))
	}

}

func HttpDns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-GSLB", "grid")
	w.WriteHeader(http.StatusOK)

	io.WriteString(w, "Hello, http!\n")
}

type HTTP_worker struct {
	Id     int
	Server *http.Server
}

func Working(listen string, port string, num int) {
	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HTTP working: %s", pan)
		}
	}()

	worker := HTTP_worker{}
	worker.Id = num

	if listener, err := reuseport.Listen("tcp", listen+":"+port); err != nil {
		G.OutDebug2(G.LOG_GSLB, "Failed to listen port: %s", err)
	} else {
		defer listener.Close()
		worker.Server = &http.Server{}
		if err := worker.Server.Serve(listener); err != nil {
			G.OutDebug2(G.LOG_GSLB, "Failed to serve: %s", err)
		}
	}

}
