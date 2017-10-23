package grid_route

import (
	"net"
	"testing"
)

func init_db() {

	RT_Db_file = "../db/route.db"
	CM_Db_file = "../db/cmdb.db"
	DM_Db_file = "../db/domain.db"

	Rtdb = &Route_db{}
	Rtdb.RT_db_init()

	MyACPrefix = "MMY"
	Service_Cutoff_Percent = 85
	Service_Deny_Percent = 95
	RT_Cache_TTL = 10
	RT_Cache_Size = 1000

}

func TestGetAAA(t *testing.T) {

	if Rtdb == nil {
		init_db()
	}

	ip := net.ParseIP("120.25.166.1")
	ac := "*.CN.HAD.SH"
	dn := "image227-c.poco.cn"

	//aaa, ttl, _type, ok, _ac := Rtdb.GetAAA(dn, ac, ip)
	aaa, ttl, _type, ok, _ac := Rtdb.GetAAA(dn, ac, ip)

	if _type == "" {
		_type = "CDN"
	}

	if ok {
		t.Logf("dn:%s ac:%s matched_ac:%s ip:%+v ttl:%d type:%s", dn, ac, _ac, aaa, ttl, _type)
	} else {
		t.Errorf("dn:%s ac:%s matched_ac:%s ip:%+v ttl:%d type:%s", dn, ac, _ac, aaa, ttl, _type)
	}
}

func TGetAAA(t *testing.T) {

	if Rtdb == nil {
		init_db()
	}

	ip := net.ParseIP("120.25.166.1")
	ac := "*.CN.HAD.SH"
	dn := "image227-c.poco.cn"

	//aaa, ttl, _type, ok, _ac := Rtdb.GetAAA(dn, ac, ip)
	aaa, ttl, _type, ok, _ac := Rtdb.GetAAA(dn, ac, ip)

	if _type == "" {
		_type = "CDN"
	}

	if ok && aaa != nil && _ac != "" && ttl != 0 {

	}
}

func TestBenchAAA(t *testing.T) {
	res := testing.Benchmark(BenchmarkAAA)
	t.Logf("RT gen_time: %f s/q", float64(res.T)/(float64(res.N)*1000000000))
	t.Logf("RT gen_rate: %d q/s", uint64(1.0/(float64(res.T)/(float64(res.N)*1000000000))))
}

func BenchmarkAAA(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		t := &testing.T{}
		for pb.Next() {
			TGetAAA(t)
		}
	})
}
