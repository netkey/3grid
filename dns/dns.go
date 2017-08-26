package grid_dns

import (
	"3grid/ip"
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

const dom = "www.chinamaincloud.com."
const default_ttl = 60

var Qs, Qps, Load uint64

type DNS_worker struct {
	Id     int
	Server *dns.Server
	Ipdb   *grid_ip.IP_db
}

func (wkr *DNS_worker) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		v4  bool
		rr  dns.RR
		txt string
		a   net.IP
		ipc string
		t   *dns.TXT
	)
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = *compress
	if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		ipc = wkr.Ipdb.GetAreaCode(ip)

		if *debug {
			txt = ipc
		}

		a = ip.IP
		v4 = a.To4() != nil
	}
	if v4 {
		rr = &dns.A{
			Hdr: dns.RR_Header{Name: dom, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: default_ttl},
			A:   a.To4(),
		}
	} else {
		rr = &dns.AAAA{
			Hdr:  dns.RR_Header{Name: dom, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: default_ttl},
			AAAA: a,
		}
	}

	if *debug {
		t = &dns.TXT{
			Hdr: dns.RR_Header{Name: dom, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: default_ttl},
			Txt: []string{txt},
		}
	}

	//return result based on dns query type
	switch r.Question[0].Qtype {
	case dns.TypeTXT:
		m.Answer = append(m.Answer, t)
		m.Extra = append(m.Extra, rr)
	default:
		fallthrough
	case dns.TypeAAAA, dns.TypeA:
		m.Answer = append(m.Answer, rr)
		if t != nil {
			m.Extra = append(m.Extra, t)
		}
	case dns.TypeAXFR, dns.TypeIXFR:
		c := make(chan *dns.Envelope)
		tr := new(dns.Transfer)
		defer close(c)
		if err := tr.Out(w, r, c); err != nil {
			return
		}
		soa, _ := dns.NewRR(`www.chinamaincloud.com. 0 IN SOA master.chinamaincloud.com. chinamaincloud.com. 20170310002 21600 7200 604800 3600`)
		c <- &dns.Envelope{RR: []dns.RR{soa, t, rr, soa}}
		w.Hijack()
		// w.Close() // Client closes connection
		return
	}

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			m.SetTsig(r.Extra[len(r.Extra)-1].(*dns.TSIG).Hdr.Name, dns.HmacMD5, 300, time.Now().Unix())
		} else {
			println("Status", w.TsigStatus().Error())
		}
	}

	if *debug {
		log.Printf("Query from: %s\n", a.String())
	}

	w.WriteMsg(m)
}

func Working(net, port, name, secret string, num int) {
	ipdb := grid_ip.IP_db{}
	ipdb.IP_db_init()

	worker := DNS_worker{}
	worker.Id = num
	worker.Ipdb = &ipdb

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
