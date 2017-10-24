package grid_dns

import (
	"github.com/miekg/dns"
	"net"
	"testing"
)

var (
	worker DNS_worker
	q      *DNS_query
	r      *dns.Msg
	aaaa   = []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
)

func init_db() {

	worker = DNS_worker{}
	worker.Id = 1
	worker.Qsc = map[string]uint64{"QS": 1}

}

func TestRR_A(t *testing.T) {

	if worker.Id == 0 {
		init_db()
	}

	qtype := dns.TypeA
	_type := "A"

	aaa := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	ip := net.ParseIP("120.25.166.1")

	ac := "*.CN.HAD.SH"
	matched_ac := "*"

	dn := "image227-c.poco.cn."
	ttl := uint32(300)

	_r := &dns.Msg{}

	_q := &DNS_query{Query_Type: qtype, Client_IP: ip, DN: dn,
		TTL: ttl, AC: ac, Matched_AC: matched_ac, Matched_Type: _type}

	if m := worker.RR(aaa, _q, nil, _r); m != nil {
		t.Logf("dn:%s ac:%s matched_ac:%s m:\n%+v", dn, ac, matched_ac, m)
	} else {
		t.Errorf("dn:%s ac:%s matched_ac:%s m:%+v", dn, ac, matched_ac, m)
	}
}

func TestRR_CNAME(t *testing.T) {

	if worker.Id == 0 {
		init_db()
	}

	qtype := dns.TypeCNAME
	_type := "CNAME"

	aaa := []string{"image-1.poco.cn"}
	ip := net.ParseIP("120.25.166.1")

	ac := "*.CN.HAD.SH"
	matched_ac := "*"

	dn := "image227-c.poco.cn."
	ttl := uint32(300)

	r := &dns.Msg{}

	q := DNS_query{Query_Type: qtype, Client_IP: ip, DN: dn,
		TTL: ttl, AC: ac, Matched_AC: matched_ac, Matched_Type: _type}

	if m := worker.RR(aaa, &q, nil, r); m != nil {
		t.Logf("dn:%s ac:%s matched_ac:%s m:\n%+v", dn, ac, matched_ac, m)
	} else {
		t.Errorf("dn:%s ac:%s matched_ac:%s m:%+v", dn, ac, matched_ac, m)
	}
}

func TRR_A(t *testing.T) {

	if worker.Id == 0 {
		init_db()
	}

	if q == nil {
		qtype := dns.TypeA
		_type := "A"

		ip := net.ParseIP("120.25.166.1")

		ac := "*.CN.HAD.SH"
		matched_ac := "*"

		dn := "image227-c.poco.cn."
		ttl := uint32(300)

		r = &dns.Msg{}

		q = &DNS_query{Query_Type: qtype, Client_IP: ip, DN: dn,
			TTL: ttl, AC: ac, Matched_AC: matched_ac, Matched_Type: _type}
	}

	if m := worker.RR(aaaa, q, nil, r); m != nil {
	}
}

func TestBenchRR(t *testing.T) {
	res := testing.Benchmark(BenchmarkRR)
	t.Logf("RR gen_time: %f s/q", float64(res.T)/(float64(res.N)*1000000000))
	t.Logf("RR gen_rate: %d q/s", uint64(1.0/(float64(res.T)/(float64(res.N)*1000000000))))
}

func BenchmarkRR(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		t := &testing.T{}
		for pb.Next() {
			TRR_A(t)
		}
	})
}
