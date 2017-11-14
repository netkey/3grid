package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/kavu/go_reuseport"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HTTP_worker struct {
	Id        int
	Name      string
	Ipdb      *IP.IP_db
	Rtdb      *RT.Route_db
	Server    *http.Server
	Handlers  map[string]interface{}
	Cache     map[string][]byte
	CacheLock *sync.RWMutex
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

func (hw *HTTP_worker) GetCache(dn, ipac, key string) []byte {

	hw.CacheLock.RLock()
	b := hw.Cache[dn+"#"+ipac+"#"+key]
	hw.CacheLock.RUnlock()

	return b
}

func (hw *HTTP_worker) UpdateCache(dn, ipac, key string, b *[]byte) {
	_key := dn + "#" + ipac + "#" + key

	hw.CacheLock.Lock()
	if hw.Cache[_key] == nil {
		hw.Cache[_key] = []byte{}
	}
	hw.Cache[_key] = append(hw.Cache[_key], *b...)
	hw.CacheLock.Unlock()
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
	var dn, ips, client_ips, ac, ipac, key string
	var ip net.IP
	var debug int = 0
	var buf bytes.Buffer
	var b []byte

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-GSLB", hw.Name)

	if dna := r.URL.Query()["domain"]; dna == nil {
		w.Write([]byte("param error"))
		return
	} else {
		dn = dna[0]
	}

	client_ips = strings.Split(r.RemoteAddr, ":")[0]
	if ipa := r.URL.Query()["ip"]; ipa == nil {
		ips = client_ips
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

	if debug != 2 {
		ipac = ips
	} else {
		ipac = ac
	}

	if r.Header["Accept-Encoding"] != nil {
		w.Header().Set("Content-Encoding", "gzip")
		key = "gzip"
	} else {
		key = "nc"
	}

	if body = hw.GetCache(dn, ipac, key); body == nil {

		aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, debug)
		if _type == "" {
			_type = "A"
		}

		_dns_info := []string{_type, strconv.Itoa(int(ttl))}
		_dns_info = append(_dns_info, aaa...)

		body, err = json.Marshal(map[string]map[string]map[string][]string{"dns": {dn: {ipac: _dns_info}}})
		if err == nil {
			w.Header().Set("X-GSLB-Cache", "Miss")
			if key == "gzip" {
				g := gzip.NewWriter(&buf)
				g.Write(body)
				g.Close()

				b = buf.Bytes()
				w.Write(b)
				hw.UpdateCache(dn, ipac, key, &b)
			} else {
				w.Write(body)
				hw.UpdateCache(dn, ipac, key, &body)
			}
		} else {
			w.Write([]byte("data error"))
		}
	} else {
		w.Header().Set("X-GSLB-Cache", "Hit")
		w.Write(body)
	}

	G.Outlog3(G.LOG_HTTP, "HttpDns %s %s %s", client_ips, dn, ipac)
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
	worker.Cache = make(map[string][]byte)
	worker.CacheLock = new(sync.RWMutex)

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
