package grid_dns

import (
	"github.com/miekg/dns"
	"net"
	"testing"
)

var worker DNS_worker

func init_db() {

	worker = DNS_worker{}
	worker.Qsc = map[string]uint64{"QS": 1}

}

func TestRR_A(t *testing.T) {

	init_db()

	qtype := dns.TypeA
	_type := "A"

	aaa := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
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

func TestRR_CNAME(t *testing.T) {

	init_db()

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
