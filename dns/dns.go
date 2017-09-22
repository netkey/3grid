package grid_dns

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"flag"
	"github.com/miekg/dns"
	"log"
	"net"
	"time"
)

var (
	debug    = flag.Bool("dns-debug", true, "output debug info")
	compress = flag.Bool("compress", false, "compress replies")
)

const DN = "mmycdn.com"
const Default_ttl = 60

var Qs, Qps, Load uint64

type DNS_worker struct {
	Id     int
	Server *dns.Server
	Ipdb   *IP.IP_db
	Rtdb   *RT.Route_db
}

func (wkr *DNS_worker) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		v4    bool
		rr    dns.RR
		txt   string
		a     net.IP
		ac    string
		t     *dns.TXT
		qtype string
		dn    string
		ttl   uint32
		aaa   []string
	)
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = *compress

	dn = r.Question[0].Name //query domain name
	ttl = Default_ttl

	if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {

		ac = wkr.Ipdb.GetAreaCode(ip)
		aaa = wkr.Rtdb.GetAAA(dn, ac)

		if G.Debug {
			txt = ac
		}

		a = ip.IP
		v4 = a.To4() != nil
	}
	if v4 {
		rr = &dns.A{
			Hdr: dns.RR_Header{Name: dn, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
			A:   a.To4(),
		}
	} else {
		rr = &dns.AAAA{
			Hdr:  dns.RR_Header{Name: dn, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
			AAAA: a,
		}
	}

	if G.Debug {
		t = &dns.TXT{
			Hdr: dns.RR_Header{Name: dn, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
			Txt: []string{txt},
		}
	}

	//return result based on dns query type
	switch r.Question[0].Qtype {
	case dns.TypeA, dns.TypeAAAA:
		qtype = "A/AAAA"
		m.Answer = append(m.Answer, rr)
		if t != nil {
			m.Extra = append(m.Extra, t)
		}
	case dns.TypeCNAME:
		qtype = "CNAME"
		m.Answer = append(m.Answer, rr)
		if t != nil {
			m.Extra = append(m.Extra, t)
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

	if G.Debug {
		log.Printf("Query from: %s, type %s, name %s, result %+v", a.String(), qtype, dn, aaa)
	}

	w.WriteMsg(m)
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
			log.Printf("Failed to setup the "+net+" server: %s\n", err.Error())
		}
	default:
		worker.Server = &dns.Server{Addr: ":" + port, Net: net, TsigSecret: map[string]string{name: secret}}
		worker.Server.Handler = &worker
		if err := worker.Server.ListenAndServe(); err != nil {
			log.Printf("Failed to setup the "+net+" server: %s\n", err.Error())
		}
	}
}
