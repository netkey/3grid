package grid_route

import G "3grid/tools/globals"
import "log"
import "net"
import "strings"
import "strconv"

var MyACPrefix string
var Service_Cutoff_Percent uint
var Service_Deny_Percent uint

//check if the ip is in my server list
func (rt_db *Route_db) IN_Serverlist(ip net.IP) (uint, bool) {
	ir := rt_db.Read_IP_Record(ip.String())
	if ir.NodeId != 0 {
		return ir.NodeId, true
	} else {
		return 0, false
	}
}

//longest match the ac code with rid in exiting db
func (rt_db *Route_db) Match_AC_RR(ac string, rid uint) Route_List_Record {
	var _ac string
	var find bool = false

	for _ac = ac; _ac != ""; {
		if rt_db.Routes[_ac] != nil {
			if rt_db.Routes[_ac][rid].Nodes != nil {
				//find a match
				find = true
				break
			}
		}

		//shorter the ac, go next match
		if li := strings.LastIndex(_ac, "."); li != -1 {
			_ac = _ac[0 : strings.LastIndex(_ac, ".")-1]
		} else {
			break
		}
	}
	if find {
		return rt_db.Read_Route_Record(_ac, rid)
	} else {
		return Route_List_Record{}
	}
}

//return A IPs based on AreaCode and DomainName
func (rt_db *Route_db) GetAAA(dn string, acode string, ip net.IP) ([]string, uint32, string, bool) {
	var ttl uint32 = 0
	var rid uint = 0
	var aaa []string
	var ac string
	var _ac string
	var ok bool = true
	var _type string
	var x int

	if _nid, ok := rt_db.IN_Serverlist(ip); ok {
		//it's my server, change the area code to its node
		irn := rt_db.Read_Node_Record(_nid)
		if irn.Name != "" {
			ac = MyACPrefix + "." + irn.Name
		} else {
			ac = acode
		}
	} else {
		//change the area code to what existing in db
		ac = acode
	}

	_ac = ac

	if G.Debug {
		log.Printf("GETAAA dn:%s, ac:%s, ip:%s", dn, ac, ip.String())
	}

	if aaa, ttl, _type, ok = rt_db.GetRTCache(dn, ac); ok {
		//found in route cache
		return aaa, ttl, _type, ok
	}

	dr := rt_db.Read_Domain_Record(dn)
	if G.Debug {
		log.Printf("GETAAA dr: %+v", dr)
	}

	if dr.TTL == 0 {
		ok = false
		return aaa, ttl, _type, ok
	} else {
		ttl = uint32(dr.TTL)
		_type = dr.Type
	}

	if dr.Type != "" {
		//not a CDN serving domain name, typically A CNAME NS
		aaa = []string{}
		s := strings.Split(dr.Value, ",")
		for _, x := range s {
			aaa = append(aaa, x)
		}
		return aaa, ttl, _type, ok
	}

	rr := Route_List_Record{}

	if dr.RoutePlan != nil {
		//try get route_record of current plan, get next plan if no rr
		for _, v := range dr.RoutePlan {
			rid = v

			//find a longest matched AC of this route plan
			rr = rt_db.Match_AC_RR(ac, rid)
			if rr.Nodes != nil {
				break
			}
		}
	}

	if rr.Nodes == nil {
		rid = 0 //default route plan
		rr = rt_db.Match_AC_RR(ac, rid)
		if rr.Nodes == nil {
			if G.Debug {
				log.Printf("GETAAA failed, ac: %s, rid: %d", ac, rid)
			}
			return aaa, ttl, _type, false
		}
	}

	if G.Debug {
		log.Printf("GETAAA ac: %s, matched ac: %s, rid: %d, rr: %+v", ac, _ac, rid, rr)
	}

	nr := rt_db.ChoseNode(rr.Nodes)
	//nr := rt_db.Read_Node_Record(224) //for debug
	nid := nr.NodeId

	if G.Debug {
		log.Printf("GETAAA nid: %d, nr: %+v", nid, nr)
	}

	x, _ = strconv.Atoi(dr.ServerGroup)
	sl := rt_db.ChoseServer(nr.ServerList, uint(x))

	if dr.Records <= uint(len(sl)) {
		aaa = make([]string, dr.Records)
	} else {
		aaa = make([]string, len(sl))
	}

	for i, sid := range sl {
		if uint(i) >= dr.Records {
			//got enough IPs
			break
		}
		sr := rt_db.Read_Server_Record(sid)
		if G.Debug {
			log.Printf("GETAAA sr: %+v", sr)
		}
		aaa[i] = sr.ServerIp
	}

	rt_db.Update_Cache_Record(dn, ac, &RT_Cache_Record{TTL: ttl, AAA: aaa, TYPE: _type})

	return aaa, ttl, _type, ok
}

//scheduler algorithm of chosing available nodes, based on priority & weight & costs & usage(%)
func (rt_db *Route_db) ChoseNode(nodes map[uint]PW_List_Record) Node_List_Record {
	var nr, cnr Node_List_Record //cnr: chosen node record, nr: node record to compare
	var nid uint = 0             //nid: chosen node id, default to 0
	var priority uint = 100      //priority: chosen node priority, default to lowest
	var weight uint = 0          //weight: chosen node weight, default to lowest

	for k, v := range nodes {
		nr = rt_db.Read_Node_Record(k)
		if v.PW[0] < priority {
			//higher priority node(priority&weight algorithm)
			if nr.Status && (nr.Usage <= Service_Deny_Percent) {
				//still available(status algorithm) to serve(cutoff algorithm)
				cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
			}
			continue
		}
		if v.PW[0] == priority {
			//equipotent priority nodes
			if v.PW[1] >= weight {
				//higher or same Weight(priority&weight algorithm)
				if nr.Status && (nr.Usage <= cnr.Usage) {
					//which has less Usage (usage algorithm)
					if nr.Costs < cnr.Costs {
						//which has less Costs (cost algorithm)
						if nr.Usage <= Service_Deny_Percent {
							//and still available(cutoff algorithm)
							cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
						}
					} else {
						//less usage, but higher Costs
						if cnr.Usage > Service_Cutoff_Percent {
							//chosen node is busy(cutoff algorithm)
							cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
						}
					}
				}
			} else {
				//lower Weight
				if nr.Status && (nr.Usage < Service_Cutoff_Percent) && (cnr.Usage > Service_Cutoff_Percent) {
					//chosen node is in cutoff state, and i am not
					cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
				}
			}
			continue
		}
		if v.PW[0] > priority {
			//lower priority node
			if nr.Status && (nr.Usage < Service_Cutoff_Percent) && (cnr.Usage > Service_Cutoff_Percent) {
				//chosen node is in cutoff state, and i am not
				cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
			}
			continue
		}
	}

	if nid == 0 {
		cnr = Node_List_Record{}
	}

	return cnr
}

//scheduler algorithm of chosing available servers, based on server (load, status), sort by weight&&idle
func (rt_db *Route_db) ChoseServer(servers []uint, servergroup uint) []uint {
	var sorted bool = false

	server_list := []uint{}
	_server_list := []uint{}

	var weight_idle, _weight_idle float64

	for _, sid := range servers {
		if rt_db.Servers[sid].Status == false || rt_db.Servers[sid].ServerGroup != servergroup {
			//server down/overload or not belong to the servergroup
			continue
		}
		weight_idle = float64(rt_db.Servers[sid].Weight * (1 - rt_db.Servers[sid].Usage/100))
		sorted = false
		for i, _sid := range server_list {
			//sort by weight&&idle, weight*(1-usage/100)
			_weight_idle = float64(rt_db.Servers[_sid].Weight * (1 - rt_db.Servers[_sid].Usage/100))

			if _weight_idle < weight_idle {
				if i == 0 {
					//insert into head of slice
					_server_list = append([]uint{sid}, server_list...)
				} else {
					//insert into middle of slice
					_server_list = append(server_list[0:i-1], sid)
					_server_list = append(_server_list, server_list[i:]...)
				}
				sorted = true
				break
			}
		}
		if sorted {
			//inserted
			server_list = _server_list
		} else {
			//append to tail of slice
			server_list = append(server_list, sid)
		}
	}

	return server_list
}
