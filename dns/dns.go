package grid_dns

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"strings"
	"time"
)

var (
	debug    = flag.Bool("dns-debug", true, "output debug info")
	compress = flag.Bool("compress", false, "compress replies")
)

const DN = "mmycdn.com"
const Default_ttl = 60

type DNS_worker struct {
	Id     int
	Server *dns.Server
	Ipdb   *IP.IP_db
	Rtdb   *RT.Route_db
}

func (wkr *DNS_worker) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		rr             dns.RR
		a              net.IP
		ip             net.IP
		ac, matched_ac string
		t              *dns.TXT
		qtype          string
		dn             string
		_dn            string
		ttl            uint32
		aaa            []string
		_type          string
		_ok            bool
	)
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = *compress

	ttl = Default_ttl
	dn = r.Question[0].Name //query domain name

	if strings.HasSuffix(dn, ".") {
		_dn = dn[0 : len(dn)-1]
	} else {
		_dn = dn
	}

	if _udp_addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		ip = _udp_addr.IP
		ac = wkr.Ipdb.GetAreaCode(ip)
		if aaa, ttl, _type, _ok, matched_ac = wkr.Rtdb.GetAAA(_dn, ac, ip); !_ok {
			if G.Debug {
				G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GetAAA failed ip:%s ac:%s dn:%s", ip, ac, _dn))
			}
			return
		}
	} else {
		return
	}

	/*
		ipv4 RR:
		rr = &dns.A{
			Hdr: dns.RR_Header{Name: dn, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
			A:   a.To4(),
		}

		ipv6 RR:
		rr = &dns.AAAA{
			Hdr:  dns.RR_Header{Name: dn, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
			AAAA: a,
		}
	*/

	if G.Debug {
		t = &dns.TXT{
			Hdr: dns.RR_Header{Name: dn, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
			Txt: []string{ac + ":" + matched_ac},
		}
	}

	//return result based on dns query type
	switch r.Question[0].Qtype {
	case dns.TypeA, dns.TypeAAAA:
		qtype = "A"
		if aaa != nil && (_type == "" || _type == "A") {
			for _, aa := range aaa {
				a = net.ParseIP(aa)
				rr = &dns.A{
					Hdr: dns.RR_Header{Name: dn, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   a.To4(),
				}
				m.Answer = append(m.Answer, rr)

			}
			if t != nil {
				m.Extra = append(m.Extra, t)
			}
		}
	case dns.TypeCNAME:
		qtype = "CNAME"
		if aaa != nil && _type == "CNAME" {
			for _, aa := range aaa {
				rr = &dns.CNAME{
					Hdr:    dns.RR_Header{Name: dn, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
					Target: aa,
				}
				m.Answer = append(m.Answer, rr)

			}
			if t != nil {
				m.Extra = append(m.Extra, t)
			}
		}
	case dns.TypeTXT:
		qtype = "TXT"
		m.Answer = append(m.Answer, t)
	case dns.TypeSOA:
		qtype = "SOA"
		soa, _ := dns.NewRR(DN + `. 0 IN SOA master.chinamaincloud.com. chinamaincloud.com. 20170310002 21600 7200 604800 3600`)
		m.Answer = append(m.Answer, soa)
	case dns.TypeAXFR, dns.TypeIXFR:
		qtype = "XFR"
		c := make(chan *dns.Envelope)
		tr := new(dns.Transfer)
		defer close(c)
		if err := tr.Out(w, r, c); err != nil {
			return
		}
		soa, _ := dns.NewRR(DN + `. 0 IN SOA master.chinamaincloud.com. chinamaincloud.com. 20170310002 21600 7200 604800 3600`)
		c <- &dns.Envelope{RR: []dns.RR{soa, t, rr, soa}}
		// w.Hijack()
		// w.Close() // Client closes connection
		//return
	default:
	}

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			m.SetTsig(r.Extra[len(r.Extra)-1].(*dns.TSIG).Hdr.Name, dns.HmacMD5, 300, time.Now().Unix())
		} else {
			println("Status", w.TsigStatus().Error())
		}
	}

	w.WriteMsg(m)

	if G.Log {
		G.Outlog(G.LOG_DNS, fmt.Sprintf("ip:%s type:%s name:%s result:%+v", ip.String(), qtype, _dn, aaa))
	}

	//update perf counter async
	G.GP.Chan <- map[string]uint64{"QS": 1}
	G.PC.Chan <- map[string]map[string]uint64{G.PERF_DOMAIN: {_dn: 1}}
}

func Working(net, port, name, secret string, num int, ipdb *IP.IP_db, rtdb *RT.Route_db) {

	worker := DNS_worker{}
	worker.Id = num
	worker.Ipdb = ipdb
	worker.Rtdb = rtdb

	switch name {
	case "":
		worker.Server = &dns.Server{Addr: ":" + port, Net: net, TsigSecret: nil}
		worker.Server.Handler = &worker
		if err := worker.Server.ListenAndServe(); err != nil {
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Failed to setup the "+net+" server: %s\n", err.Error()))
		}
	default:
		worker.Server = &dns.Server{Addr: ":" + port, Net: net, TsigSecret: map[string]string{name: secret}}
		worker.Server.Handler = &worker
		if err := worker.Server.ListenAndServe(); err != nil {
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Failed to setup the "+net+" server: %s\n", err.Error()))
		}
	}
}
