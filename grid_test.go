package main

import (
	IP "3grid/ip"
	RT "3grid/route"
	"bufio"
	"net"
	"os"
	"strings"
	"testing"
)

func init_db() {

	IP.Db_file = "db/ip.db"
	ipdb := IP.IP_db{}

	IP.Ipdb = &ipdb
	IP.Ipdb.IP_db_init()

	RT.RT_Db_file = "db/route.db"
	RT.CM_Db_file = "db/cmdb.db"
	RT.DM_Db_file = "db/domain.db"

	RT.Rtdb = &RT.Route_db{}
	RT.Rtdb.RT_db_init()

	RT.MyACPrefix = "MMY"
	RT.Service_Cutoff_Percent = 85
	RT.Service_Deny_Percent = 95
	RT.RT_Cache_TTL = 10
	RT.RT_Cache_Size = 1000

}

func sp_sort(sp string) []string {
	var aaa = []string{}
	var _aaa = []string{}
	var first bool = true

	sss := strings.Split(sp, ",")
	for _, s := range sss {
		if first {
			first = false
			aaa = append(aaa, s)
			continue
		}
		for i, a := range aaa {
			//sort, a<b<c...
			if s < a {
				if i == 0 {
					aaa = append([]string{s}, aaa...)
				} else {
					_aaa = append(aaa[0:i-1], s)
					aaa = append(_aaa, aaa[i:]...)
				}
			}
		}
	}

	return aaa
}

func check_ips(sss, aaa []string) (same bool) {
	var s, a string

	if sss != nil && sss[0] != "" {
		if li := strings.LastIndex(sss[0], "."); li != -1 {
			s = sss[0][0 : li-1]
		}
	}

	if aaa != nil && aaa[0] != "" {
		if li := strings.LastIndex(aaa[0], "."); li != -1 {
			a = aaa[0][0 : li-1]
		}
	}

	if s == a {
		same = true
	} else {
		same = false
	}

	return
}

func TestGrid(t *testing.T) {
	var testfile string = "logs/slb1/test_cdn.log"
	var line string

	if IP.Ipdb == nil {
		init_db()
	}

	if file, err := os.Open(testfile); err == nil {
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line = scanner.Text()

			//get line from slb log
			a := strings.Split(line, "|")
			ip := a[0]
			dn := strings.ToLower(a[1])
			sss := sp_sort(a[2])
			if sss == nil {
				sss = []string{"0"}
			}
			s_ac := IP.Ipdb.GetAreaCode(net.ParseIP(sss[0]))

			//resolve in grid
			_ip := net.ParseIP(ip)
			ac := IP.Ipdb.GetAreaCode(net.ParseIP(ip))
			aaa, _, _, _, _ac, rid := RT.Rtdb.GetAAA(dn, ac, _ip)
			if aaa == nil {
				aaa = []string{"0"}
			}
			a_ac := IP.Ipdb.GetAreaCode(net.ParseIP(aaa[0]))

			if same := check_ips(sss, aaa); same != true {
				t.Errorf("ip:%s(%s:%s) dn:%s sss:%+v(%s) aaa:%v(%s) rid:%d", ip, ac, _ac, dn, sss[0], s_ac, aaa[0], a_ac, rid)
			}
		}
	}

}
