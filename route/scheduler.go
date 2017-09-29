package grid_route

import G "3grid/tools/globals"
import "log"
import "net"
import "strings"
import "strconv"

var MyACPrefix string

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

	aaa = make([]string, dr.Records)
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
	var cnr Node_List_Record //chosed node record
	var nr Node_List_Record
	var nid uint = 0
	var priority uint = 0
	var weight uint = 0
	var first bool = true

	for k, v := range nodes {
		if first {
			cnr = rt_db.Read_Node_Record(k)
			nid = k
			priority = v.PW[0]
			weight = v.PW[1]
			first = false
			continue
		}

		if priority > v.PW[0] {
			nr = rt_db.Read_Node_Record(k)
			if nr.Status {
				//higher priority node which is ok
				if nr.Usage <= 90 {
					//and still available (<90% usage)
					cnr = nr
					nid = k
					priority = v.PW[0]
					weight = v.PW[1]
				}
			}
		} else {
			if priority == v.PW[0] {
				//equipotent nodes
				if weight <= v.PW[1] {
					//higher or same Weight
					nr = rt_db.Read_Node_Record(k)
					if nr.Usage <= cnr.Usage {
						//which has less Usage
						if nr.Costs < cnr.Costs {
							//which has less Costs
							if nr.Usage <= 90 {
								cnr = nr
								nid = k
								priority = v.PW[0]
								weight = v.PW[1]
							}
						} else {
							//less usage, but higher Costs
							if cnr.Usage > 85 {
								//chosen node is busy
								cnr = nr
								nid = k
								priority = v.PW[0]
								weight = v.PW[1]
							}
						}
					}
				}
			}
		}
	}
	if nid != 0 {
		return cnr
	} else {
		return Node_List_Record{}
	}
}

//scheduler algorithm of chosing available servers, based on server (load, status)
func (rt_db *Route_db) ChoseServer(servers []uint, servergroup uint) []uint {
	server_list := []uint{}

	for _, sid := range servers {
		if rt_db.Servers[sid].Status == false || rt_db.Servers[sid].ServerGroup != servergroup {
			//server down/overload or not belong to the servergroup
			continue
		}
		server_list = append(server_list, sid)
	}

	return server_list
}
