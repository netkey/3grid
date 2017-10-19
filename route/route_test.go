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

	init_db()

	ip := net.ParseIP("120.25.166.1")
	ac := "*.CN.HAD.SH"
	dn := "image227-c.poco.cn"

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
