package grid_http

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	HTTP_Cache_Size      int
	HTTP_Cache_TTL       int64
	HTTP_Engine          string
	HTTP_302_Mode        int
	HTTP_302_Param       string
	RTMP_URL_MODE_HEADER string
	RTMP_URL_HEADER      string
	ChanOut              bool
	ChanOutQueue         int
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
	Chan      chan WB
}

type WB struct {
	C *net.Conn
	O *bufio.ReadWriter
	W *http.ResponseWriter
	R *http.Request
	B *[]byte
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
	var dn, ac, dom, ips string
	var ip net.IP

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic HttpDns: %s", pan)
		}
	}()

	ctx.Response.Header.Set("Content-Type", "text/pain; charset=utf-8")

	ip = ctx.RemoteIP()
	ips = ip.String()

	ac = hw.Ipdb.GetAreaCode(ip, ips)

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

	http_302_mode := HTTP_302_Mode

	statuscode := fasthttp.StatusFound
	proto := "http://"
	url := ctx.URI().String()
	url_path := string(ctx.URI().Path())
	args := ctx.URI().QueryArgs()

	if rtmp_mode := ctx.Request.Header.Peek(RTMP_URL_MODE_HEADER); rtmp_mode != nil {
		if rtmp_url := ctx.Request.Header.Peek(RTMP_URL_HEADER); rtmp_url != nil {
			http_302_mode, _ = strconv.Atoi(string(rtmp_mode))
			http_302_mode = http_302_mode - 2
			statuscode = fasthttp.StatusOK
			proto = "rtmp://"
			url = string(rtmp_url)
			if li := strings.Index(url, "/"); li != -1 {
				dn = url[:li]
				url_path = url[li:]
			}
			url = proto + url
			args = &fasthttp.Args{}
		}
	}

	aaa, _, _, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, ips, 0)

	if aaa != nil && len(aaa) > 0 {
		if http_302_mode == 1 {
			if args != nil {
				args.Add(HTTP_302_Param, dn)
			}
		} else if http_302_mode == 0 {
			url_path = "/" + dn + url_path
		} else {
			url_path = ""
		}

		location_url := ""
		if args != nil && args.Len() > 0 {
			location_url = proto + aaa[0] + url_path + "?" + string(args.QueryString())
		} else {
			location_url = proto + aaa[0] + url_path
		}

		ctx.Response.Header.Set("Location", location_url)
		ctx.SetStatusCode(statuscode)

		if statuscode == fasthttp.StatusOK {
			ctx.Write([]byte(location_url))
		}

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
	ac = hw.Ipdb.GetAreaCode(ip, ips)

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

	http_302_mode := HTTP_302_Mode

	statuscode := fasthttp.StatusFound
	proto := "http://"
	_url := r.URL.String()
	url_path := r.URL.Path
	args := r.URL.Query()

	if rtmp_mode := r.Header.Get(RTMP_URL_MODE_HEADER); rtmp_mode != "" {
		if rtmp_url := r.Header.Get(RTMP_URL_HEADER); rtmp_url != "" {
			http_302_mode, _ = strconv.Atoi(rtmp_mode)
			http_302_mode = http_302_mode - 2
			statuscode = http.StatusOK
			proto = "rtmp://"
			_url = rtmp_url
			if li := strings.Index(_url, "/"); li != -1 {
				dn = _url[:li]
				url_path = _url[li:]
			}
			_url = proto + _url
			args = url.Values{}
		}
	}

	aaa, _, _, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, ips, 0)

	if aaa != nil && len(aaa) > 0 {
		if http_302_mode == 1 {
			if args != nil {
				args.Add(HTTP_302_Param, dn)
			}
		} else if http_302_mode == 0 {
			url_path = "/" + dn + url_path
		} else {
			url_path = ""
		}

		location_url := ""
		if args != nil && len(args) > 0 {
			location_url = proto + aaa[0] + url_path + "?" + args.Encode()
		} else {
			location_url = proto + aaa[0] + url_path
		}

		w.Header().Set("Location", location_url)
		w.WriteHeader(statuscode)

		if statuscode == http.StatusOK {
			w.Write([]byte(location_url))
		}

		G.Outlog3(G.LOG_HTTP, "302 %s %s %s %s", ip, dn, _url, aaa[0])
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavalible"))

		G.Outlog3(G.LOG_HTTP, "302 %s %s %s nil", ip, dn, _url)
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
		ac = hw.Ipdb.GetAreaCode(ip, ips)
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

		aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, ips, debug)
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

	client_ips := strings.Split(r.RemoteAddr, ":")[0]
	client_ip = net.ParseIP(client_ips)

	if ipa := r.URL.Query()["ip"]; ipa == nil {
		ip = client_ip
		ips = client_ip.String()
	} else {
		ips = ipa[0]
		ip = net.ParseIP(ips)
	}

	ipac = ips
	if aca := r.URL.Query()["ac"]; aca == nil {
		ac = hw.Ipdb.GetAreaCode(ip, ips)
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

		aaa, ttl, _type, _, _, _, _ := hw.Rtdb.GetAAA(dn, ac, ip, ips, debug)
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
				hw.HttpOut0(&w, r, &b)
				hw.UpdateCache(dn, ipac, key, &aaa, &b)

			} else {
				hw.HttpOut0(&w, r, &data)
				hw.UpdateCache(dn, ipac, key, &aaa, &data)
			}
		} else {
			_b := []byte("data error")
			hw.HttpOut0(&w, r, &_b)
		}
	} else {
		w.Header().Set("X-Gslb-Cache", "Hit")
		hw.HttpOut0(&w, r, body.Data)
		aip = (*body.AAA)[0]
	}

	G.Outlog3(G.LOG_HTTP, "Dns %s %s %s %s", client_ips, dn, ipac, aip)
}

func (hw *HTTP_worker) HttpOut0(w *http.ResponseWriter, r *http.Request, data *[]byte) {
	if false { //not implemented yet
		if c, o, err := (*w).(http.Hijacker).Hijack(); err == nil {
			hw.Chan <- WB{&c, o, w, r, data}
		} else {
			(*w).Write(*data)
		}
	} else {
		(*w).Write(*data)
	}
}

func (hw *HTTP_worker) ChanOut0() {
	for {
		wb := <-hw.Chan
		c, o, b, w, r := *wb.C, *wb.O, *wb.B, *wb.W, *wb.R

		headers := r.Proto + " 200 ok\n"
		w.Header().Set("Date", time.Now().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		for k, v := range w.Header() {
			headers = headers + k + ": " + v[0] + "\n"
		}
		if r.ProtoMajor == 1 && r.ProtoMinor == 0 {
			if !r.Close {
				headers = headers + "Connection: keep-alive\n"
			}
		}
		headers += "\n"

		buf := append([]byte(headers), b...)
		o.Write(buf)
		o.Flush()

		if r.Close {
			c.Close()
		}
	}
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

	if G.PProf {
		if num == 0 {
			go func() {
				http.ListenAndServe("127.0.0.1:6060", nil) //for net/http/pprof
			}()
		}
	}

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
			if ChanOut {
				worker.Chan = make(chan WB, ChanOutQueue)
				go worker.ChanOut0()
			}
			if err := worker.Server0.Serve(listener); err != nil {
				G.OutDebug2(G.LOG_GSLB, "Failed to serve: %s", err)
			}
		}
	}

}
