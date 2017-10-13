package grid_route

import G "3grid/tools/globals"
import "fmt"
import "net"
import "strings"
import "time"

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
func (rt_db *Route_db) Match_AC_RR(ac string, rid uint) (string, Route_List_Record) {
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
		return _ac, rt_db.Read_Route_Record(_ac, rid)
	} else {
		return "", Route_List_Record{}
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
		G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GETAAA dn:%s, ac:%s, ip:%s", dn, ac, ip.String()))
	}

	if aaa, ttl, _type, ok = rt_db.GetRTCache(dn, ac); ok {
		//found in route cache
		return aaa, ttl, _type, ok
	}

	dr := rt_db.Read_Domain_Record(dn)
	if G.Debug {
		G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GETAAA dr: %+v", dr))
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
			_ac, rr = rt_db.Match_AC_RR(ac, rid)
			if rr.Nodes != nil {
				break
			}
		}
	}

	if rr.Nodes == nil {
		rid = 0 //default route plan
		_ac, rr = rt_db.Match_AC_RR(ac, rid)
		if rr.Nodes == nil {
			if G.Debug {
				G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GETAAA failed, ac: %s, rid: %d", ac, rid))
			}
			return aaa, ttl, _type, false
		}
	}

	if G.Log {
		G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GETAAA match_ac_rr: %+v", rr))
		G.Outlog(G.LOG_ROUTE, fmt.Sprintf("GETAAA ac:%s, matched ac:%s, rid:%d, rr:%+v", ac, _ac, rid, rr))
	}

	nr := rt_db.ChoseNode(rr.Nodes, ac)
	//nr := rt_db.Read_Node_Record(224) //for debug
	nid := nr.NodeId

	if G.Debug {
		G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GETAAA nid: %d, nr: %+v", nid, nr))
	}

	sl := rt_db.ChoseServer(nr.ServerList, dr.ServerGroup)

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
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("GETAAA sr: %+v", sr))
		}
		aaa[i] = sr.ServerIp
	}

	rt_db.Update_Cache_Record(dn, ac, &RT_Cache_Record{TS: time.Now().Unix(), TTL: ttl, AAA: aaa, TYPE: _type})

	return aaa, ttl, _type, ok
}

//check if a node covered the client ac
func (rt_db *Route_db) Match_Local_AC(node_ac string, client_ac string) bool {
	return strings.Contains(client_ac, node_ac)
}

//scheduler algorithm of chosing available nodes, based on priority & weight & costs & usage(%)
func (rt_db *Route_db) ChoseNode(nodes map[uint]PW_List_Record, client_ac string) Node_List_Record {
	var nr, cnr Node_List_Record //cnr: chosen node record, nr: node record to compare
	var nid uint = 0             //nid: chosen node id, default to 0
	var priority uint = 100      //priority: chosen node priority, default to lowest
	var weight uint = 0          //weight: chosen node weight, default to lowest
	var weight_idle, _weight_idle float64

	nr, cnr = Node_List_Record{}, Node_List_Record{}

	if G.Debug {
		G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("we have nodes: %+v", nodes))
	}

	for k, v := range nodes {
		nr = rt_db.Read_Node_Record(k)
		if G.Debug {
			G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("looking at node: %+v", nr))
		}
		if nr.Status == false || nr.Usage >= Service_Deny_Percent {
			//not available(status algorithm) to serve(cutoff algorithm)
			if G.Debug {
				G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("%s not available to serve", nr.Name))
			}
			continue
		}
		if nr.Status && cnr.NodeId != 0 &&
			(nr.Usage < Service_Cutoff_Percent) && (cnr.Usage > Service_Cutoff_Percent) {
			//chosen node is in cutoff state, and i am not(cutoff algorithm)
			if rt_db.Match_Local_AC(cnr.AC, client_ac) || cnr.Usage > Service_Deny_Percent {
				//cutoff none local client access, then local's
				if G.Debug {
					G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("%s is in busy, use %s instead", cnr.Name, nr.Name))
				}
				cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
			}
			continue
		}
		if v.PW[0] < priority {
			//higher priority node(priority&weight algorithm)
			cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
			if G.Debug {
				G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("%s has higher priority", nr.Name))
			}
			continue
		}
		if v.PW[0] == priority {
			//equipotent priority node
			if nr.Costs < cnr.Costs {
				//which has less Costs (cost algorithm)
				cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
				if G.Log {
					G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("%s is less costs, use it", nr.Name))
				}
			} else if nr.Costs == cnr.Costs {
				//same Costs
				weight_idle = float64(float64(weight) * (1.0 - float64(cnr.Usage)/100.0))
				_weight_idle = float64(float64(v.PW[1]) * (1.0 - float64(nr.Usage)/100.0))

				if _weight_idle >= weight_idle {
					//higher or same weight&&idle(weight&usage algorithm), means more idle
					cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
					if G.Log {
						G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("%s is more idle(%f %f), use it", nr.Name, weight_idle, _weight_idle))
					}
				} else {
					//lower weight_idle and not cheaper
				}
			}
			continue
		}
		if v.PW[0] > priority {
			//lower priority node
			weight_idle = float64(weight * (1 - cnr.Usage/100))
			_weight_idle = float64(v.PW[1] * (1 - nr.Usage/100))

			if _weight_idle >= weight_idle && nr.Costs < cnr.Costs {
				//higher or same weight&&idle(weight&usage algorithm) and cheaper node
				cnr, nid, priority, weight = nr, k, v.PW[0], v.PW[1]
				if G.Log {
					G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("%s is more idle and less costs, use it", nr.Name))
				}
			} else {
				//lower weight_idle or not cheaper
			}
			continue
		}
	}

	if G.Log {
		G.Outlog(G.LOG_SCHEDULER, fmt.Sprintf("Chose node:%+v for ac:%s", cnr, client_ac))
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
