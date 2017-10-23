package grid_ip

import (
	"net"
	"testing"
)

func init_db() {
	Db_file = "../db/ip.db"
	ipdb := IP_db{}

	Ipdb = &ipdb
	Ipdb.IP_db_init()
}

func TestGetAreaCode(t *testing.T) {

	if Ipdb == nil {
		init_db()
	}

	ip := net.ParseIP("61.144.1.1")
	ac := Ipdb.GetAreaCode(ip)

	if ac == "CTC.CN.HAN.GD" {
		t.Logf("ip:%s ac:%s", ip.String(), ac)
	} else {
		t.Errorf("ip:%s ac:%s", ip.String(), ac)
	}
}

func TAC(t *testing.T) {

	if Ipdb == nil {
		init_db()
	}

	ip := net.ParseIP("61.144.1.1")
	ac := Ipdb.GetAreaCode(ip)

	if ac == "CTC.CN.HAN.GD" {
	} else {
	}
}

func TestBenchAC(t *testing.T) {
	res := testing.Benchmark(BenchmarkAC)
	t.Logf("AC gen_time: %f s/q", float64(res.T)/(float64(res.N)*1000000000))
	t.Logf("AC gen_rate: %d q/s", uint64(1.0/(float64(res.T)/(float64(res.N)*1000000000))))
}

func BenchmarkAC(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		t := &testing.T{}
		for pb.Next() {
			TAC(t)
		}
	})
}
