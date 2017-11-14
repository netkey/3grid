package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"bytes"
	"compress/gzip"
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
	hw.Handlers["/"] = hw.Http302

	mux := http.NewServeMux()

	for location, handler := range hw.Handlers {
		mux.HandleFunc(location, handler.(func(http.ResponseWriter, *http.Request)))
	}

	return mux
}

func (hw *HTTP_worker) GetCache(dn, ipac string) *[]byte {
	if hw.Cache[dn] != nil {
		if hw.Cache[dn][ipac] != nil {
			b := hw.Cache[dn][ipac]
			if b == nil {
				return nil
			} else {
				return &b
			}
		}
	}
	return nil
}

func (hw *HTTP_worker) UpdateCache(dn, ipac string, b *[]byte) {
	if hw.Cache[dn] == nil {
		hw.Cache[dn] = make(map[string][]byte)
	}
	if hw.Cache[dn][ipac] == nil {
		hw.Cache[dn][ipac] = []byte{}
	}
	hw.Cache[dn][ipac] = append(hw.Cache[dn][ipac], *b...)
}

//HTTP 302
func (hw *HTTP_worker) Http302(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/pain; charset=utf-8")
	w.Header().Set("X-GSLB", hw.Name)

	w.Write([]byte("Hello, 302"))
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

	_pb := hw.GetCache(dn, ipac)
	if _pb != nil {
		body = *_pb
	}

	if body == nil {
		w.Header().Set("X-Cache", "Miss")

		if body, err = json.Marshal(map[string]map[string]map[string][]string{"dns": {dn: {ipac: _dns_info}}}); err == nil {
			if false && r.Header["Accept-Encoding"] != nil {
				var buf bytes.Buffer
				var b []byte
				g := gzip.NewWriter(&buf)
				g.Write(body)
				g.Close()

				b = buf.Bytes()
				w.Header().Set("Content-Encoding", "gzip")
				w.Write(b)
				hw.UpdateCache(dn, ipac, &b)
			} else {
				w.Write(body)
				hw.UpdateCache(dn, ipac, &body)
			}
		} else {
			w.Write([]byte("data error"))
		}
	} else {
		w.Header().Set("X-Cache", "Hit")
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
