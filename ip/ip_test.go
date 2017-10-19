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

	init_db()

	ip := net.ParseIP("61.144.1.1")
	ac := Ipdb.GetAreaCode(ip)

	if ac == "CTC.CN.HAN.GD" {
		t.Logf("ip:%s ac:%s", ip.String(), ac)
	} else {
		t.Errorf("ip:%s ac:%s", ip.String(), ac)
	}
}
