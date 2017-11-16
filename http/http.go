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
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	HTTP_Cache_Size int
	HTTP_Cache_TTL  int64
)

type HTTP_worker struct {
	Id        int
	Name      string
	Ipdb      *IP.IP_db
	Rtdb      *RT.Route_db
	Server    *fasthttp.Server
	Handlers  map[string]interface{}
	Cache     map[string]HTTP_Cache_Record
	CacheSize int
	CacheLock *sync.RWMutex
}

type HTTP_Cache_Record struct {
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

func (hw *HTTP_worker) GetCache(dn, ipac, key string) *[]byte {

	hw.CacheLock.RLock()
	b := hw.Cache[dn+"#"+ipac+"#"+key]
	hw.CacheLock.RUnlock()

	if b.TS != 0 {
		if b.TS+HTTP_Cache_TTL < time.Now().Unix() {
			//cache data expired
			return nil
		} else {
			return b.Data
		}
	}
	return nil
}

func (hw *HTTP_worker) UpdateCache(dn, ipac, key string, b *[]byte) {
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
		hw.Cache[dn+"#"+ipac+"#"+key] = HTTP_Cache_Record{Data: b, TS: time.Now().Unix()}
		hw.CacheSize += 1
	}
	hw.CacheLock.Unlock()
}

//HTTP 302
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
				hw.UpdateCache(dn, ipac, key, &b)
			} else {
				ctx.Write(data)
				hw.UpdateCache(dn, ipac, key, &data)
			}
		} else {
			ctx.Write([]byte("data error"))
		}
	} else {
		ctx.Response.Header.Set("X-Gslb-Cache", "Hit")
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

	worker.Cache = make(map[string]HTTP_Cache_Record)
	worker.CacheLock = new(sync.RWMutex)

	if listener, err := reuseport.Listen("tcp4", listen+":"+port); err != nil {
		G.OutDebug2(G.LOG_GSLB, "Failed to listen port: %s", err)
	} else {
		defer listener.Close()

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
	}

}
