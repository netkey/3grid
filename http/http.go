package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"bytes"
	"compress/gzip"
	"encoding/json"
	//"github.com/kavu/go_reuseport"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
	"net"
	//"net/http"
	"strconv"
	"sync"
)

type HTTP_worker struct {
	Id         int
	Name       string
	Ipdb       *IP.IP_db
	Rtdb       *RT.Route_db
	Server     *fasthttp.Server
	Handlers   map[string]interface{}
	Cache      map[string]*[]byte
	ACache     map[string]string
	CacheLock  *sync.RWMutex
	ACacheLock *sync.RWMutex
}

/* net/http
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
*/

func (hw *HTTP_worker) Handler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/dns":
		hw.HttpDns(ctx)
	default:
		hw.Http302(ctx)
	}
}

func (hw *HTTP_worker) GetCache(dn, ipac, key string) *[]byte {

	hw.CacheLock.RLock()
	b := hw.Cache[dn+"#"+ipac+"#"+key]
	hw.CacheLock.RUnlock()

	return b
}

func (hw *HTTP_worker) UpdateCache(dn, ipac, key string, b *[]byte) {
	hw.CacheLock.Lock()
	hw.Cache[dn+"#"+ipac+"#"+key] = b
	hw.CacheLock.Unlock()
}

func (hw *HTTP_worker) GetACache(ips string) string {

	hw.ACacheLock.RLock()
	s := hw.ACache[ips]
	hw.ACacheLock.RUnlock()

	return s
}

func (hw *HTTP_worker) UpdateACache(ips string, s string) {
	hw.ACacheLock.Lock()
	hw.ACache[ips] = s
	hw.ACacheLock.Unlock()
}

//HTTP 302
/* net/http
func (hw *HTTP_worker) Http302(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/pain; charset=utf-8")
	w.Header().Set("X-GSLB", hw.Name)

	w.Write([]byte("Hello, 302"))
}
*/
func (hw *HTTP_worker) Http302(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "text/pain; charset=utf-8")
	ctx.Write([]byte("Hello, 302"))
}

//HTTP DNS
func (hw *HTTP_worker) HttpDns(ctx *fasthttp.RequestCtx) {
	var err error
	var body *[]byte
	var data []byte
	var dn, ips, ac, ipac, key string
	var client_ip, ip net.IP
	var debug int = 0
	var buf bytes.Buffer
	var b []byte

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	ctx.Response.Header.Set("Content-Type", "application/json; charset=utf-8")

	if dn = string(ctx.QueryArgs().Peek("domain")); dn == "" {
		ctx.Write([]byte("param error"))
		return
	}

	client_ip = ctx.RemoteIP()
	if ips = string(ctx.QueryArgs().Peek("ip")); ips == "" {
		ip = client_ip
		ips = client_ip.String()
	} else {
		ip = net.ParseIP(ips)
	}

	if ac = string(ctx.QueryArgs().Peek("ac")); ac == "" {
		if ac = hw.GetACache(ips); ac == "" {
			ac = hw.Ipdb.GetAreaCode(ip)
			hw.UpdateACache(ips, ac)
		}
	} else {
		debug = 2
	}

	if debug != 2 {
		ipac = ips
	} else {
		ipac = ac
	}

	if ctx.Request.Header.Peek("Accept-Encoding") != nil {
		ctx.Response.Header.Set("Content-Encoding", "gzip")
		key = "gzip"
	} else {
		key = "nc"
	}

	/*
		ips = "61.144.1.1"
		dn = "gzcmcc.chinamaincloud.com"
		ip = net.ParseIP(ips)
		client_ip = ip
		ac = "CTC.CN.GD.GZ"
		ipac = ips
		key = "nc"
	*/

	if body = hw.GetCache(dn, ipac, key); body == nil {

		aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, debug)
		if _type == "" {
			_type = "A"
		}

		_dns_info := []string{_type, strconv.Itoa(int(ttl))}
		_dns_info = append(_dns_info, aaa...)

		data, err = json.Marshal(map[string]map[string]map[string][]string{"dns": {dn: {ipac: _dns_info}}})
		if err == nil {
			ctx.Response.Header.Set("X-GSLB-Cache", "Miss")
			if key == "gzip" {
				g := gzip.NewWriter(&buf)
				g.Write(data)
				g.Close()

				b = buf.Bytes()
				ctx.Write(b)
				hw.UpdateCache(dn, ipac, key, &b)
			} else {
				ctx.Write(data)
				hw.UpdateCache(dn, ipac, key, &data)
			}
		} else {
			ctx.Write([]byte("data error"))
		}
	} else {
		ctx.Response.Header.Set("X-GSLB-Cache", "Hit")
		ctx.Write(*body)
	}

	G.Outlog3(G.LOG_HTTP, "HttpDns %s %s %s", client_ip.String(), dn, ipac)
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
	worker.Cache = make(map[string]*[]byte)
	worker.ACache = make(map[string]string)
	worker.CacheLock = new(sync.RWMutex)
	worker.ACacheLock = new(sync.RWMutex)

	if listener, err := reuseport.Listen("tcp4", listen+":"+port); err != nil {
		G.OutDebug2(G.LOG_GSLB, "Failed to listen port: %s", err)
	} else {
		defer listener.Close()

		/* net/http
		worker.Server = &http.Server{
			Handler:      worker.InitHandlers(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  10 * time.Second,
		}
		*/
		worker.Server = &fasthttp.Server{
			Handler: worker.Handler,
			Name:    myname,
		}

		if err := worker.Server.Serve(listener); err != nil {
			G.OutDebug2(G.LOG_GSLB, "Failed to serve: %s", err)
		}
	}

}
