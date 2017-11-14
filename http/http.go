package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"encoding/json"
	"github.com/kavu/go_reuseport"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTP_worker struct {
	Id       int
	Name     string
	Ipdb     *IP.IP_db
	Rtdb     *RT.Route_db
	Server   *http.Server
	Handlers map[string]interface{}
	Cache    map[string]map[string][]byte
}

func (hw *HTTP_worker) InitHandlers() *http.ServeMux {
	hw.Handlers = make(map[string]interface{})
	hw.Handlers["/dns"] = hw.HttpDns

	mux := http.NewServeMux()

	for location, handler := range hw.Handlers {
		mux.HandleFunc(location, handler.(func(http.ResponseWriter, *http.Request)))
	}

	return mux
}

func (hw *HTTP_worker) GetCache(dn, ipac string) []byte {
	var b []byte

	if hw.Cache[dn] != nil {
		if hw.Cache[ipac] != nil {
			b = hw.Cache[dn][ipac]
		}
	}

	return b
}

func (hw *HTTP_worker) UpdateCache(dn, ipac string, b []byte) {
	if hw.Cache[dn] == nil {
		hw.Cache[dn] = make(map[string][]byte)
		hw.Cache[dn][ipac] = b
	}
}

//HTTP DNS
func (hw *HTTP_worker) HttpDns(w http.ResponseWriter, r *http.Request) {
	var err error
	var body []byte
	var dn, ips, ac, ipac string
	var ip net.IP
	var debug int = 0

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-GSLB", hw.Name)

	w.WriteHeader(http.StatusOK)

	if dna := r.URL.Query()["domain"]; dna == nil {
		io.WriteString(w, "param error")
		return
	} else {
		dn = dna[0]
	}
	if ipa := r.URL.Query()["ip"]; ipa == nil {
		ips = strings.Split(r.RemoteAddr, ":")[0]
	} else {
		ips = ipa[0]
	}
	ip = net.ParseIP(ips)

	if aca := r.URL.Query()["ac"]; aca != nil {
		ac = aca[0]
		debug = 2
	} else {
		ac = hw.Ipdb.GetAreaCode(ip)
	}

	aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, debug)
	if _type == "" {
		_type = "A"
	}

	_dns_info := []string{_type, strconv.Itoa(int(ttl))}
	_dns_info = append(_dns_info, aaa...)

	if debug != 2 {
		ipac = ips
	} else {
		ipac = ac
	}

	body = hw.GetCache(dn, ipac)

	if body == nil {
		if body, err = json.Marshal(map[string]map[string]map[string][]string{"dns": {dn: {ipac: _dns_info}}}); err == nil {
			w.Write(body)
			hw.UpdateCache(dn, ipac, body)
		} else {
			io.WriteString(w, "data error")
		}
	} else {
		w.Write(body)
	}

	G.Outlog3(G.LOG_HTTP, "HttpDns %s %s", dn, ipac)
}

func Working(myname, listen string, port string, num int, ipdb *IP.IP_db, rtdb *RT.Route_db) {
	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HTTP working: %s", pan)
		}
	}()

	worker := HTTP_worker{}
	worker.Id = num
	worker.Name = myname
	worker.Ipdb = ipdb
	worker.Rtdb = rtdb
	worker.Cache = make(map[string]map[string][]byte)

	if listener, err := reuseport.Listen("tcp", listen+":"+port); err != nil {
		G.OutDebug2(G.LOG_GSLB, "Failed to listen port: %s", err)
	} else {
		defer listener.Close()

		worker.Server = &http.Server{
			Handler:      worker.InitHandlers(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  10 * time.Second,
		}

		if err := worker.Server.Serve(listener); err != nil {
			G.OutDebug2(G.LOG_GSLB, "Failed to serve: %s", err)
		}
	}

}
