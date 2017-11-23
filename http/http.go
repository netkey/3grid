package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	HTTP_Cache_Size int
	HTTP_Cache_TTL  int64
	HTTP_Engine     string
	HTTP_302_Mode   int
	HTTP_302_Param  string
)

type HTTP_worker struct {
	Id        int
	Name      string
	Ipdb      *IP.IP_db
	Rtdb      *RT.Route_db
	Server    *fasthttp.Server
	Server0   *http.Server
	Cache     map[string]HTTP_Cache_Record
	CacheSize int
	CacheLock *sync.RWMutex
}

type HTTP_Cache_Record struct {
	AAA  *[]string
	Data *[]byte
	TS   int64
}

func (hw *HTTP_worker) Handler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/dns":
		hw.HttpDns(ctx)
	default:
		hw.Http302(ctx)
	}
}

func (hw *HTTP_worker) Handler0() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", hw.Http3020)
	mux.HandleFunc("/dns", hw.HttpDns0)
	return mux
}

func (hw *HTTP_worker) GetCache(dn, ipac, key string) *HTTP_Cache_Record {

	hw.CacheLock.RLock()
	b := hw.Cache[dn+"#"+ipac+"#"+key]
	hw.CacheLock.RUnlock()

	if b.TS != 0 {
		if b.TS+HTTP_Cache_TTL < time.Now().Unix() {
			//cache data expired
			return nil
		} else {
			return &b
		}
	}
	return nil
}

func (hw *HTTP_worker) UpdateCache(dn, ipac, key string, a *[]string, b *[]byte) {
	hw.CacheLock.Lock()
	if b == nil {
		delete(hw.Cache, dn+"#"+ipac+"#"+key)
		if hw.CacheSize > 0 {
			hw.CacheSize -= 1
		}
	} else {
		if hw.CacheSize >= HTTP_Cache_Size {
			//cache too large, pop one
			for x, _ := range hw.Cache {
				delete(hw.Cache, x)
				hw.CacheSize -= 1
				break
			}
		}
		hw.Cache[dn+"#"+ipac+"#"+key] = HTTP_Cache_Record{AAA: a, Data: b, TS: time.Now().Unix()}
		hw.CacheSize += 1
	}
	hw.CacheLock.Unlock()
}

//HTTP 302
func (hw *HTTP_worker) Http302(ctx *fasthttp.RequestCtx) {
	var dn, ac, dom string
	var ip net.IP

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	ctx.Response.Header.Set("Content-Type", "text/pain; charset=utf-8")

	ip = ctx.RemoteIP()
	ac = hw.Ipdb.GetAreaCode(ip)

	_dn := ctx.Request.Header.Peek("Host")
	if _dn != nil {
		dom = string(_dn)
		if strings.Contains(dom, ":") {
			dn = strings.Split(dom, ":")[0]
		} else {
			dn = dom
		}
	} else {
		dom = ""
		dn = ""
	}

	aaa, _, _, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, 0)

	url := ctx.URI().String()
	url_path := string(ctx.URI().Path())
	args := ctx.URI().QueryArgs()

	if aaa != nil && len(aaa) > 0 {
		if HTTP_302_Mode == 1 {
			args.Add(HTTP_302_Param, dn)
		} else {
			url_path = "/" + dn + url_path
		}

		if args != nil && args.Len() > 0 {
			ctx.Response.Header.Set("Location", "http://"+aaa[0]+url_path+"?"+string(args.QueryString()))
		} else {
			ctx.Response.Header.Set("Location", "http://"+aaa[0]+url_path)
		}

		ctx.SetStatusCode(fasthttp.StatusFound)

		G.Outlog3(G.LOG_HTTP, "302 %s %s %s", ip, url, aaa[0])
	} else {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.Write([]byte("service unavalible"))

		G.Outlog3(G.LOG_HTTP, "302 %s %s nil", ip, url)
	}
}

func (hw *HTTP_worker) Http3020(w http.ResponseWriter, r *http.Request) {
	var dn, ac, ips, dom string
	var ip net.IP

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	w.Header().Set("Content-Type", "text/pain; charset=utf-8")
	w.Header().Set("Server", hw.Name)

	ips = strings.Split(r.RemoteAddr, ":")[0]
	ip = net.ParseIP(ips)
	ac = hw.Ipdb.GetAreaCode(ip)

	_dn := r.Host
	if _dn != "" {
		dom = _dn
		if strings.Contains(dom, ":") {
			dn = strings.Split(dom, ":")[0]
		} else {
			dn = dom
		}
	} else {
		dom = ""
		dn = ""
	}

	aaa, _, _, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, 0)

	url := r.URL.String()
	url_path := r.URL.Path
	args := r.URL.Query()

	if aaa != nil && len(aaa) > 0 {
		if HTTP_302_Mode == 1 {
			args.Add(HTTP_302_Param, dn)
		} else {
			url_path = "/" + dn + url_path
		}

		if args != nil && len(args) > 0 {
			w.Header().Set("Location", "http://"+aaa[0]+url_path+"?"+args.Encode())
		} else {
			w.Header().Set("Location", "http://"+aaa[0]+url_path)
		}

		w.WriteHeader(http.StatusFound)

		G.Outlog3(G.LOG_HTTP, "302 %s %s %s %s", ip, dn, url, aaa[0])
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavalible"))

		G.Outlog3(G.LOG_HTTP, "302 %s %s %s nil", ip, dn, url)
	}
}

//HTTP DNS
func (hw *HTTP_worker) HttpDns(ctx *fasthttp.RequestCtx) {
	var err error
	var body *HTTP_Cache_Record
	var data []byte
	var dn, ips, ac, ipac, key, aip string
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

	ipac = ips
	if ac = string(ctx.QueryArgs().Peek("ac")); ac == "" {
		ac = hw.Ipdb.GetAreaCode(ip)
	} else {
		debug = 2
		ipac = ac
	}

	aeh := ctx.Request.Header.Peek("Accept-Encoding")
	if aeh != nil {
		if strings.Contains(string(aeh), "gzip") {
			ctx.Response.Header.Set("Content-Encoding", "gzip")
			key = "gzip"
		} else {
			key = "nc"
		}
	} else {
		key = "nc"
	}

	if body = hw.GetCache(dn, ipac, key); body == nil {

		aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, debug)
		if aaa == nil {
			aaa = []string{""}
		} else if len(aaa) == 0 {
			aaa = []string{""}
		}
		aip = aaa[0]

		if _type == "" {
			_type = "A"
		}

		_dns_info := []string{_type, strconv.Itoa(int(ttl))}
		_dns_info = append(_dns_info, aaa...)

		data, err = json.Marshal(map[string]map[string]map[string][]string{"dns": {dn: {ipac: _dns_info}}})
		if err == nil {
			ctx.Response.Header.Set("X-Gslb-Cache", "Miss")
			if key == "gzip" {
				g := gzip.NewWriter(&buf)
				g.Write(data)
				g.Close()

				b = buf.Bytes()
				ctx.Write(b)
				hw.UpdateCache(dn, ipac, key, &aaa, &b)
			} else {
				ctx.Write(data)
				hw.UpdateCache(dn, ipac, key, &aaa, &data)
			}
		} else {
			ctx.Write([]byte("data error"))
		}
	} else {
		ctx.Response.Header.Set("X-Gslb-Cache", "Hit")
		ctx.Write(*body.Data)
		aip = (*body.AAA)[0]
	}

	G.Outlog3(G.LOG_HTTP, "Dns %s %s %s %s", client_ip.String(), dn, ipac, aip)
}

func (hw *HTTP_worker) HttpDns0(w http.ResponseWriter, r *http.Request) {
	var err error
	var body *HTTP_Cache_Record
	var data []byte
	var dn, ips, ac, ipac, key, aip string
	var client_ip, ip net.IP
	var debug int = 0
	var buf bytes.Buffer
	var b []byte

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Server", hw.Name)

	if dna := r.URL.Query()["domain"]; dna == nil {
		w.Write([]byte("param error"))
		return
	} else {
		dn = dna[0]
	}

	client_ip = net.ParseIP(strings.Split(r.RemoteAddr, ":")[0])
	if ipa := r.URL.Query()["ip"]; ipa == nil {
		ip = client_ip
		ips = client_ip.String()
	} else {
		ips = ipa[0]
		ip = net.ParseIP(ips)
	}

	ipac = ips
	if aca := r.URL.Query()["ac"]; aca == nil {
		ac = hw.Ipdb.GetAreaCode(ip)
	} else {
		debug = 2
		ipac = aca[0]
	}

	aeh := r.Header["Accept-Encoding"]
	if aeh != nil {
		if strings.Contains(aeh[0], "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			key = "gzip"
		} else {
			key = "nc"
		}
	} else {
		key = "nc"
	}

	if body = hw.GetCache(dn, ipac, key); body == nil {

		aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, debug)
		if aaa == nil {
			aaa = []string{""}
		} else if len(aaa) == 0 {
			aaa = []string{""}
		}
		aip = aaa[0]

		if _type == "" {
			_type = "A"
		}

		_dns_info := []string{_type, strconv.Itoa(int(ttl))}
		_dns_info = append(_dns_info, aaa...)

		data, err = json.Marshal(map[string]map[string]map[string][]string{"dns": {dn: {ipac: _dns_info}}})
		if err == nil {
			w.Header().Set("X-Gslb-Cache", "Miss")
			if key == "gzip" {
				g := gzip.NewWriter(&buf)
				g.Write(data)
				g.Close()

				b = buf.Bytes()
				w.Write(b)
				hw.UpdateCache(dn, ipac, key, &aaa, &b)
			} else {
				w.Write(data)
				hw.UpdateCache(dn, ipac, key, &aaa, &data)
			}
		} else {
			w.Write([]byte("data error"))
		}
	} else {
		w.Header().Set("X-Gslb-Cache", "Hit")
		w.Write(*body.Data)
		aip = (*body.AAA)[0]
	}

	G.Outlog3(G.LOG_HTTP, "Dns %s %s %s %s", client_ip.String(), dn, ipac, aip)
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

	worker.Cache = make(map[string]HTTP_Cache_Record)
	worker.CacheLock = new(sync.RWMutex)

	if listener, err := reuseport.Listen("tcp4", listen+":"+port); err != nil {
		G.OutDebug2(G.LOG_GSLB, "Failed to listen port: %s", err)
	} else {
		defer listener.Close()

		if HTTP_Engine == "fasthttp" {
			worker.Server = &fasthttp.Server{
				Handler:              worker.Handler,
				Name:                 myname,
				ReadTimeout:          10 * time.Second,
				WriteTimeout:         10 * time.Second,
				MaxKeepaliveDuration: 10 * time.Second,
			}
			if err := worker.Server.Serve(listener); err != nil {
				G.OutDebug2(G.LOG_GSLB, "Failed to serve: %s", err)
			}
		} else if HTTP_Engine == "gohttp" {
			worker.Server0 = &http.Server{
				Handler:      worker.Handler0(),
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  10 * time.Second,
			}
			if err := worker.Server0.Serve(listener); err != nil {
				G.OutDebug2(G.LOG_GSLB, "Failed to serve: %s", err)
			}
		}
	}

}
