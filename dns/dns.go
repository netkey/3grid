package grid_dns

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"flag"
	"github.com/miekg/dns"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	Compress = flag.Bool("compress", false, "compress replies")

	Bufsize       int
	DN            string
	IP_DN_Spliter string
	AC_DN_Spliter string
)

const Default_ttl = 60

type DNS_worker struct {
	Id     int
	Server *dns.Server
	Ipdb   *IP.IP_db
	Rtdb   *RT.Route_db
	Qsc    map[string]uint64
}

type DNS_query struct {
	Query_Type   uint16 //client query type
	Client_IP    net.IP //client ip
	DN           string //query domain name
	TTL          uint32 //ttl
	AC           string //client area code
	Matched_AC   string //matched ac in db
	Matched_Type string //mathed query type in db
	Debug        int    //debug query, more info, 0:no-debug 1:ip-debug 2:ac-debug
	RID          uint   //route plan id, for debug
}

func (wkr *DNS_worker) RR(aaa []string, q *DNS_query, w dns.ResponseWriter, r *dns.Msg) *dns.Msg {
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

	var (
		rr    dns.RR
		qtype uint16
		t     *dns.TXT
		a     net.IP
	)

	if aaa == nil {
		return nil
	}

	if G.Debug || q.Debug > 0 {
		t = &dns.TXT{
			Hdr: dns.RR_Header{Name: q.DN, Rrtype: dns.TypeTXT,
				Class: dns.ClassINET, Ttl: q.TTL},
			Txt: []string{q.AC + ":" + q.Matched_AC + ":" + strconv.Itoa(int(q.RID))},
		}
	}

	m := dns.Msg{}
	m.SetReply(r)
	m.Compress = *Compress

	if qtype = q.Query_Type; qtype != dns.TypeSOA {
		switch q.Matched_Type {
		case "A", "AAAA", "":
			qtype = dns.TypeA
		case "CNAME":
			qtype = dns.TypeCNAME
		case "TXT":
			qtype = dns.TypeTXT
		case "NS":
			qtype = dns.TypeNS
		}
	}

	/*
		TypeNone       uint16 = 0
		TypeA          uint16 = 1
		TypeNS         uint16 = 2
		TypeMD         uint16 = 3
		TypeMF         uint16 = 4
		TypeCNAME      uint16 = 5
		TypeSOA        uint16 = 6
		TypeMB         uint16 = 7
		TypeMG         uint16 = 8
		TypeMR         uint16 = 9
		TypeNULL       uint16 = 10
		TypePTR        uint16 = 12
		TypeHINFO      uint16 = 13
		TypeMINFO      uint16 = 14
		TypeMX         uint16 = 15
		TypeTXT        uint16 = 16
		TypeRP         uint16 = 17
		TypeAFSDB      uint16 = 18
		TypeX25        uint16 = 19
		TypeISDN       uint16 = 20
		TypeRT         uint16 = 21
		TypeNSAPPTR    uint16 = 23
		TypeSIG        uint16 = 24
		TypeKEY        uint16 = 25
		TypePX         uint16 = 26
		TypeGPOS       uint16 = 27
		TypeAAAA       uint16 = 28
		TypeLOC        uint16 = 29
		TypeNXT        uint16 = 30
		TypeEID        uint16 = 31
	*/

	switch qtype {
	case dns.TypeA, dns.TypeAAAA:
		for _, aa := range aaa {
			a = net.ParseIP(aa)
			rr = &dns.A{
				Hdr: dns.RR_Header{Name: q.DN, Rrtype: dns.TypeA,
					Class: dns.ClassINET, Ttl: q.TTL},
				A: a.To4(),
			}
			m.Answer = append(m.Answer, rr)

		}
		if t != nil {
			m.Extra = append(m.Extra, t)
		}
	case dns.TypeCNAME:
		for _, aa := range aaa {
			rr = &dns.CNAME{
				Hdr: dns.RR_Header{Name: q.DN, Rrtype: dns.TypeCNAME,
					Class: dns.ClassINET, Ttl: q.TTL},
				Target: aa + ".", //CNAME must has tailing "."
			}
			m.Answer = append(m.Answer, rr)

		}
		if t != nil {
			m.Extra = append(m.Extra, t)
		}
	case dns.TypeNS:
		for _, aa := range aaa {
			rr = &dns.CNAME{
				Hdr: dns.RR_Header{Name: q.DN, Rrtype: dns.TypeNS,
					Class: dns.ClassINET, Ttl: q.TTL},
				Target: aa + ".", //NS must has tailing "."
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
	}

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			m.SetTsig(r.Extra[len(r.Extra)-1].(*dns.TSIG).Hdr.Name,
				dns.HmacMD5, 300, time.Now().Unix())
		} else {
			println("Status", w.TsigStatus().Error())
		}
	}

	return &m
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
		debug          int      //debug query
		rid            uint     //route plan id
	)

	defer func() {
		if err := recover(); err != nil {
			G.Outlog3(G.LOG_GSLB, "Panic: %s", err)
		}
	}()

	debug = 0

	ttl = Default_ttl
	dn = r.Question[0].Name //query domain name

	if strings.HasSuffix(dn, ".") {
		_dn = dn[0 : len(dn)-1]
	} else {
		_dn = dn
	}

	_dn = strings.ToLower(_dn)

	if _udp_addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {

		if r.Extra == nil {
			ip = _udp_addr.IP
		} else {
			//maybe an edns0 request, try to get client ip(subnet)
			if ip = getEdnsSubNet(r); ip == nil {
				ip = _udp_addr.IP
			}
		}

		if strings.Contains(_dn, IP_DN_Spliter) {
			//if domain name contains debug ip, set it
			spl := strings.Split(_dn, IP_DN_Spliter)
			ip = net.ParseIP(spl[0])
			_dn = spl[1]
			debug = 1
		}

		if strings.Contains(_dn, AC_DN_Spliter) {
			//if domain name contains debug ac, set it
			spl := strings.Split(_dn, AC_DN_Spliter)
			ac = strings.ToUpper(spl[0])
			if strings.HasSuffix(ac, "\\") {
				ac = ac[0 : len(ac)-1]
			}
			_dn = spl[1]
			debug = 2
		} else {
			ac = wkr.Ipdb.GetAreaCode(ip)
		}

		//G.Outlog3(G.LOG_DNS, "Serving DNS query: %s %s", _dn, ip.String())

		if aaa, ttl, _type, _ok, matched_ac, rid, _ = wkr.Rtdb.GetAAA(_dn, ac, ip, debug); !_ok {

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

	//q := DNS_query{Query_Type: r.Question[0].Qtype, Client_IP: ip, DN: r.Question[0].Name,
	q := DNS_query{Query_Type: r.Question[0].Qtype, Client_IP: ip, DN: _dn + ".",
		TTL: ttl, AC: ac, Matched_AC: matched_ac, Matched_Type: _type, Debug: debug, RID: rid}

	//generate answer and send it
	if m := wkr.RR(aaa, &q, w, r); m != nil {
		w.WriteMsg(m)
	}

	//update global perf counter async
	G.GP.Chan <- wkr.Qsc

	//update domain perf counter async
	G.PC.Chan <- map[string]map[string]uint64{G.PERF_DOMAIN: {_dn: 1}}

	//output query log
	G.Outlog3(G.LOG_DNS, "%s|%d|%s|%s", ip, r.Question[0].Qtype, _dn, strings.Join(aaa, ","))
}

func Working(nets, listen, port, name, secret string, num int, ipdb *IP.IP_db, rtdb *RT.Route_db) {

	worker := DNS_worker{}
	worker.Id = num
	worker.Ipdb = ipdb
	worker.Rtdb = rtdb
	worker.Qsc = map[string]uint64{"QS": 1}

	defer func() {
		if err := recover(); err != nil {
			G.Outlog3(G.LOG_GSLB, "Panic: %s", err)
		}
	}()

	switch name {
	case "":
		worker.Server = &dns.Server{Addr: listen + ":" + port, Net: nets,
			TsigSecret: nil}
	default:
		worker.Server = &dns.Server{Addr: listen + ":" + port, Net: nets,
			TsigSecret: map[string]string{name: secret}}
	}

	worker.Server.Handler = &worker

	worker.Server.NotifyStartedFunc = func() {
		pck_conn := worker.Server.PacketConn
		if pck_conn != nil {
			udp_conn := pck_conn.(*net.UDPConn)
			if Bufsize > 0 {
				udp_conn.SetReadBuffer(Bufsize)
				udp_conn.SetWriteBuffer(Bufsize)
			}
			fd, _ := udp_conn.File()

			value, _ := syscall.GetsockoptInt(int(fd.Fd()), syscall.SOL_SOCKET, syscall.SO_SNDBUF)
			G.OutDebug("Worker %d UDP socket SNDBUF size:%d", worker.Id, value)

			value, _ = syscall.GetsockoptInt(int(fd.Fd()), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
			G.OutDebug("Worker %d UDP socket RCVBUF size:%d", worker.Id, value)

			/*
			   https://github.com/miekg/dns/blob/master/udp_linux.go#79:
			   Calling File() above results in the connection becoming blocking, we must fix that.
			   See https://github.com/miekg/dns/issues/279
			*/
			syscall.SetNonblock(int(fd.Fd()), true)
			fd.Close()
		}
	}

	if err := worker.Server.ListenAndServe(); err != nil {
		G.OutDebug("Failed to setup the "+nets+" server: %s", err)
	}
}

func getEdnsSubNet(r *dns.Msg) (ip net.IP) {
	for _, extra := range r.Extra {
		for _, o := range extra.(*dns.OPT).Option {
			switch e := o.(type) {
			case *dns.EDNS0_SUBNET:
				if e.Address != nil {
					ip = e.Address
					//ip_block := fmt.Sprintf("%s/%d", e.Address.String(), e.SourceNetmask)
				}
			}
		}
	}

	return
}
