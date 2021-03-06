package main

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"bufio"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
)

func init_db(t *testing.T) {

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

func init_log(t *testing.T) {

	var err error

	G.Debug = false
	G.Test = true
	G.Log = true
	G.LogBufSize = 1000

	dir := "/dev/shm"

	{
		//init api log
		G.Apilog_Lock = new(sync.RWMutex)

		G.Apilog = &G.ApiLog{Goid: 0, Chan: nil, Clock: nil}
		G.Apilog.Clock = new(sync.RWMutex)
	}

	if G.Logger, err = G.NewLogger(&dir); err == nil {
		G.LogChan = &G.Logger.Chan
		G.LogChan3 = &G.Logger.Chan3

		go G.Logger.Output()
		go G.Logger.Output3()
		go G.Logger.Checklogs()

	} else {
		t.Errorf("Error init logger")
	}

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
		} else {
			s = sss[0]
		}
	}

	if aaa != nil && aaa[0] != "" {
		if li := strings.LastIndex(aaa[0], "."); li != -1 {
			a = aaa[0][0 : li-1]
		} else {
			a = aaa[0]
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
	var testfile string = "logs/slb1/test.log"
	var line string
	var _ip net.IP

	var test_counter, fail_counter int

	if IP.Ipdb == nil {
		init_db(t)
		init_log(t)
	}

	//t.Logf("num:%d  %+v", len(RT.Rtdb.Ips), RT.Rtdb.Ips)

	if file, err := os.Open(testfile); err == nil {
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line = scanner.Text()
			test_counter += 1
			//get line from slb log
			a := strings.Split(line, "|")
			if len(a) < 4 {
				continue
			}
			ip := a[0]
			ip_edns := a[1]
			dn := strings.ToLower(a[2])
			sss := sp_sort(a[3])
			if sss == nil {
				sss = []string{"0"}
			}
			s_ac := IP.Ipdb.GetAreaCode(net.ParseIP(sss[0]))

			//resolve in grid
			if ip_edns == "0.0.0.0" {
				_ip = net.ParseIP(ip)
			} else {
				_ip = net.ParseIP(ip_edns)
			}

			ac := IP.Ipdb.GetAreaCode(_ip)
			aaa, _, _, ok, match_ac, rid, _ := RT.Rtdb.GetAAA(dn, ac, _ip, 3) //despite node status check
			if !ok {
				G.Outlog3(G.LOG_TEST, "dn:%s ip:%s(%s) sss:%+v(%s) aaa:%+v rid:%d(%s)",
					dn, ip, ac, sss, s_ac, aaa, rid, match_ac)
				//t.Errorf("Error GetAAA: ip:%s(%s) dn:%s aaa:%v(%s) rid:%d",
				//	_ip, ac, dn, aaa, match_ac, rid)
			}

			if aaa == nil || len(aaa) < 1 {
				aaa = []string{"0"}
			}
			a_ac := IP.Ipdb.GetAreaCode(net.ParseIP(aaa[0]))

			match := false
			//compare results
			if same := check_ips(sss, aaa); same != true {
				if ac != a_ac {
					//client and aaa node not in same area code

					if li1 := strings.LastIndex(ac, "."); li1 != -1 {
						if li2 := strings.LastIndex(a_ac, "."); li2 != -1 {
							if ac[:li1-1] != a_ac[:li2-1] {
								//not in same major area
								match = false
							} else {
								match = true
							}
						} else {
							match = false
						}
					} else {
						match = false
					}

					if rid == 8 && (match_ac == "*" || match_ac[:2] == "*.") &&
						(a_ac == "CTC.CN.HAD.ZJ" || a_ac == "CTC.CN.HAN.GD" ||
							a_ac == "CUC.CN.HAB.SD") {
						match = true
					}
				} else {
					match = true
				}
			} else {
				match = true
			}

			if match {
				//t.Logf("Correct: ip:%s(%s) dn:%s sss:%+v(%s) aaa:%v(%s) rid:%d(%s)",
				//ip, ac, dn, sss[0], s_ac, aaa[0], a_ac, rid, _ac)
			} else {
				fail_counter += 1
				G.Outlog3(G.LOG_TEST, "dn:%s ip:%s(%s) sss:%+v(%s) aaa:%v(%s) rid:%d(%s)",
					dn, ip, ac, sss[0], s_ac, aaa[0], a_ac, rid, match_ac)

				//t.Errorf("ip:%s(%s) sss:%+v(%s) aaa:%v(%s) rid:%d(%s)",
				//ip, ac, sss[0], s_ac, aaa[0], a_ac, rid, match_ac)
			}
		}

		x := float64(fail_counter * 100.0 / (1.0 * test_counter))
		if x > 10 {
			// <90% match, test fail
			t.Errorf("Test resolving ip failed with %+v%% of data unmatched", x)
		} else {
			t.Logf("Test resolving ip successed with %+v%% of data unmatched", x)
		}

	}

}
