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

	DN string
)

const Default_ttl = 60

type DNS_worker struct {
	Id     int
	Server *dns.Server
	Ipdb   *IP.IP_db
	Rtdb   *RT.Route_db
	Qsc    map[string]uint64
}

func (wkr *DNS_worker) RR(query_type uint16, client_ip net.IP, dn string, ttl uint32, ac, matched_ac, _type string, aaa []string, w dns.ResponseWriter, r *dns.Msg) *dns.Msg {
	//return result based on dns query type

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

	var rr dns.RR
	var qtype uint16
	var t *dns.TXT
	var a net.IP

	G.OutDebug("aaa:%+v", aaa)
	if aaa == nil {
		return nil
	}

	if G.Debug {
		t = &dns.TXT{
			Hdr: dns.RR_Header{Name: dn, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
			Txt: []string{ac + ":" + matched_ac},
		}
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = *compress

	qtype = query_type

	switch _type {
	case "A", "AAAA", "":
		qtype = dns.TypeA
	case "CNAME":
		qtype = dns.TypeCNAME
	case "TXT":
		qtype = dns.TypeTXT
	case "SOA":
		qtype = dns.TypeSOA
	case "NS":
		qtype = dns.TypeNS
	}

	switch qtype {
	case dns.TypeA, dns.TypeAAAA:
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
	case dns.TypeCNAME:
		for _, aa := range aaa {
			rr = &dns.CNAME{
				Hdr:    dns.RR_Header{Name: dn, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
				Target: aa + ".",
			}
			m.Answer = append(m.Answer, rr)

		}
		if t != nil {
			m.Extra = append(m.Extra, t)
		}
	case dns.TypeNS:
		for _, aa := range aaa {
			rr = &dns.CNAME{
				Hdr:    dns.RR_Header{Name: dn, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
				Target: aa + ".",
			}
			m.Answer = append(m.Answer, rr)

		}
		if t != nil {
			m.Extra = append(m.Extra, t)
		}
	case dns.TypeTXT:
		m.Answer = append(m.Answer, t)
	case dns.TypeSOA:
		soa, _ := dns.NewRR(DN + `. 0 IN SOA master.chinamaincloud.com. chinamaincloud.com. 20170310002 21600 7200 604800 3600`)
		m.Answer = append(m.Answer, soa)
	case dns.TypeAXFR, dns.TypeIXFR:
		c := make(chan *dns.Envelope)
		tr := new(dns.Transfer)
		defer close(c)
		if err := tr.Out(w, r, c); err != nil {
			return nil
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

	//output query log
	G.Outlog3(G.LOG_DNS, "ip:%s type:%s name:%s result:%+v", client_ip.String(), qtype, dn, aaa)

	return m
}

func (wkr *DNS_worker) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		ip             net.IP   //client ip
		ac, matched_ac string   //client AC, actually matched AC
		dn             string   //query domain name
		_dn            string   //mangled dn
		ttl            uint32   //answer ttl
		aaa            []string //ips
		_type          string   //query type in string
		_ok            bool     //GetAAA status
	)

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

			G.OutDebug("GetAAA failed ip:%s ac:%s dn:%s", ip, ac, _dn)

			//return will cause the client to re-query 2 times more
			//it's better to answer a null record to reduce client re-query times

			//return
		} else {
			G.OutDebug("GetAAA ip:%s ac:%s dn:%s type:%s aaa:%+v", ip, ac, _dn, _type, aaa)
		}

	} else {
		//some error under UDP server level
		return
	}

	//generate answer and send it
	if m := wkr.RR(r.Question[0].Qtype, ip, dn, ttl, ac, matched_ac, _type, aaa, w, r); m != nil {
		w.WriteMsg(m)
	}

	G.OutDebug("Query ip:%s ac:%s dn:%s type:%s aaa:%+v", ip, ac, _dn, _type, aaa)

	//update global perf counter async
	G.GP.Chan <- wkr.Qsc

	//update domain perf counter async
	G.PC.Chan <- map[string]map[string]uint64{G.PERF_DOMAIN: {_dn: 1}}
}

func Working(net, port, name, secret string, num int, ipdb *IP.IP_db, rtdb *RT.Route_db) {

	worker := DNS_worker{}
	worker.Id = num
	worker.Ipdb = ipdb
	worker.Rtdb = rtdb

	worker.Qsc = map[string]uint64{"QS": 1}

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
