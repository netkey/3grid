package grid

import (
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"log"
	"net"
	"sync"
	"time"
)

var (
	printf   = flag.Bool("print", false, "print replies")
	compress = flag.Bool("compress", false, "compress replies")
)

const dom = "www.chinamaincloud.com."

type DNS_worker struct {
	Id      int
	Server  *dns.Server
	Ipcache map[string]string
	Ipdb    *geoip2.Reader
	Lock    *sync.RWMutex
}

func (wkr *DNS_worker) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		v4  bool
		rr  dns.RR
		str string
		a   net.IP
		ipc string
	)
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = *compress
	if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		ips := ip.IP.String()
		//wkr.Lock.RLock()
		ipc = wkr.Ipcache[ips]
		//wkr.Lock.RUnlock()

		if ipc == "" {
			re, err := wkr.Ipdb.City(ip.IP)
			if err != nil {
				log.Fatal(err)
			}
			ipc = re.City.Names["en"] + "/" + re.Country.IsoCode

			wkr.Lock.Lock()
			wkr.Ipcache[ips] = ipc
			wkr.Lock.Unlock()
		}

		//str = "You are from: " + r.City.Names["en"] + "/" + r.Country.Names["en"]
		str = "You are from: " + ipc

		a = ip.IP
		v4 = a.To4() != nil
	}
	/*
		if ip, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			str = "Port: " + strconv.Itoa(ip.Port) + " (tcp)"
			a = ip.IP
			v4 = a.To4() != nil
		}
	*/
	if v4 {
		rr = &dns.A{
			Hdr: dns.RR_Header{Name: dom, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
			A:   a.To4(),
		}
	} else {
		rr = &dns.AAAA{
			Hdr:  dns.RR_Header{Name: dom, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
			AAAA: a,
		}
	}

	t := &dns.TXT{
		Hdr: dns.RR_Header{Name: dom, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0},
		Txt: []string{str},
	}

	switch r.Question[0].Qtype {
	case dns.TypeTXT:
		m.Answer = append(m.Answer, t)
		m.Extra = append(m.Extra, rr)
	default:
		fallthrough
	case dns.TypeAAAA, dns.TypeA:
		m.Answer = append(m.Answer, rr)
		m.Extra = append(m.Extra, t)
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
	if *printf {
		//fmt.Printf("%v\n", m.String())
		fmt.Printf("Query from: %s\n", a.String())
	}
	// set TC when question is tc.miek.nl.
	/*
		if m.Question[0].Name == "tc.miek.nl." {
			m.Truncated = true
			// send half a message
			buf, _ := m.Pack()
			w.Write(buf[:len(buf)/2])
			return
		}
	*/
	w.WriteMsg(m)
}
