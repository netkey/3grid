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
	var rr Route_List_Record

	for _ac = ac; _ac != ""; {
		//G.Outlog3(G.LOG_SCHEDULER, "AC: matching:%s", _ac)
		rr = rt_db.Read_Route_Record(_ac, rid)

		if rr.Nodes != nil {
			//find a match
			find = true
			break
		}

		//shorter the ac, go next match
		if li := strings.LastIndex(_ac, "."); li != -1 {
			_ac = _ac[:li]
		} else {
			break
		}
	}

	if !find {
		_ac = "*"
		rr = rt_db.Read_Route_Record(_ac, rid)
		if rr.Nodes != nil {
			find = true
		}
	}

	if !find {
		_ac = "*.*"
		rr = rt_db.Read_Route_Record(_ac, rid)
		if rr.Nodes != nil {
			find = true
		}
	}

	if find {
		G.Outlog3(G.LOG_SCHEDULER, "AC:%s matched in route plan %d, %+v", _ac, rid, rr)
		return _ac, rr
	} else {
		G.Outlog3(G.LOG_SCHEDULER, "No matched ac:%s in route plan %d", ac, rid)
		return "", Route_List_Record{}
	}
}

//longest match the domain name
func (rt_db *Route_db) Match_DN(query_dn string) (Domain_List_Record, bool) {
	var dn string
	var dr Domain_List_Record
	var find bool = false

	for dn = query_dn; dn != ""; {
		//G.Outlog3(G.LOG_SCHEDULER, "DN: matching:%s", dn)
		dr = rt_db.Read_Domain_Record(dn)
		if dr.TTL != 0 {
			//domain name not found
			find = true
			break
		}

		//if the first section in dn is "*", cut it
		if dn[:2] == "*." {
			dn = dn[2:]
		}

		//replace the first section in dn with *, go next match
		if li := strings.Index(dn, "."); li != -1 {
			dn = "*" + dn[li:]
		} else {
			break
		}
	}

	if find {
		G.Outlog3(G.LOG_SCHEDULER, "DN:%s matched for %s, %+v", dn, query_dn, dr)
		return dr, true
	} else {
		G.Outlog3(G.LOG_SCHEDULER, "DN:No matched for %s", query_dn)
		return Domain_List_Record{}, false
	}

}

func (rt_db *Route_db) Match_FB(ac string, dr *Domain_List_Record) (_ac string, _matched bool) {
	_ac = ac
	_matched = false

	/*
		ac cases: client_ac, node_ac
		forbidden  CT.CN.HAN    *.CN.HAD
		client_ac  CT.CN.HAN.HN *.CN.HAD.SH
		node_ac    CT.CN.HAN.HN CT.CN.HAD.SH CT.CN.HAD.ZJ.JH
	*/

	if _ac != "" && dr != nil && dr.Forbidden != nil {
		for fb, _ := range dr.Forbidden {
			if strings.Index(ac, fb) == 0 {
				_matched = true
				if li := strings.LastIndex(fb, "."); li != -1 {
					_ac = fb[:li]
				}
			} else {
				if fb[:2] == "*." {
					if li := strings.Index(ac, fb[2:]); li > 0 {
						_matched = true
						_ac = ac[:li-1]
					}
				}
			}
		}
	}

	if _matched {
		G.Outlog3(G.LOG_SCHEDULER, "Match_FB mangle client ac:%s to %s", ac, _ac)
	}

	return
}

//Tag: AAA
//return A IPs based on AreaCode and DomainName
func (rt_db *Route_db) GetAAA(query_dn string, acode string, ip net.IP) ([]string, uint32, string, bool, string) {
	var ttl uint32 = 0
	var rid uint = 0
	var aaa []string
	var ac string
	var _ac string
	var ok bool = true
	var _type string
	var dn string
	var sl []uint

	if _nid, ok := rt_db.IN_Serverlist(ip); ok {
		//it's my server, change the area code to its node
		irn := rt_db.Read_Node_Record(_nid)
		if irn.AC != "" {
			ac = MyACPrefix + "." + irn.AC
		} else {
			ac = acode
		}
	} else {
		//change the area code to what existing in db
		ac = acode
	}

	_ac = ac
	dn = query_dn

	if aaa, ttl, _type, ok = rt_db.GetRTCache(dn, ac); ok {
		//found in route cache
		if aaa == nil {
			//a fail cache result
			ok = false
		}
		return aaa, ttl, _type, ok, _ac
	}

	//find domain record
	dr, ok := rt_db.Match_DN(dn)

	if ok {
		dn = dr.Name
		ttl = uint32(dr.TTL)
		_type = dr.Type
	} else {
		return aaa, ttl, _type, false, _ac
	}

	if dr.Type != "" {
		//not a CDN serving domain name, typically A CNAME NS
		aaa = []string{}
		s := strings.Split(dr.Value, ",")
		for _, x := range s {
			aaa = append(aaa, x)
		}
		return aaa, ttl, _type, ok, _ac
	}

	if dr.Forbidden != nil {
		//match client ac in domain forbidden map and mangle it
		ac, _ = rt_db.Match_FB(ac, &dr)
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
			return aaa, ttl, _type, false, _ac
		}
	}

	G.Outlog3(G.LOG_ROUTE, "GETAAA ac:%s, matched ac:%s, rid:%d, rr:%+v", ac, _ac, rid, rr)

	//choose primary and secondary node for serving
	cnr, snr := rt_db.ChooseNodeS(rr.Nodes, ac, &dr)

	nid := cnr.NodeId
	if nid == 0 {
		//cache the fail rusult for a few seconds
		rt_db.Update_Cache_Record(dn, ac, &RT_Cache_Record{TS: time.Now().Unix(), TTL: 5, AAA: aaa, TYPE: _type})
		return aaa, ttl, _type, false, ac
	}

	if snr.NodeId != 0 {
		cnr_sl := rt_db.ChooseServer(cnr.ServerList, dr.ServerGroup)
		snr_sl := rt_db.ChooseServer(snr.ServerList, dr.ServerGroup)

		sl = append(cnr_sl, snr_sl...)
	} else {
		sl = rt_db.ChooseServer(cnr.ServerList, dr.ServerGroup)
	}

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
		aaa[i] = sr.ServerIp
	}

	rt_db.Update_Cache_Record(dn, ac, &RT_Cache_Record{TS: time.Now().Unix(), TTL: ttl, AAA: aaa, TYPE: _type})

	return aaa, ttl, _type, true, _ac
}

//check if a node covered the client ac
func (rt_db *Route_db) Match_Local_AC(node_ac string, client_ac string) bool {
	return strings.Contains(client_ac, node_ac)
}

func (rt_db *Route_db) ChooseNodeS(nodes map[uint]PW_List_Record, client_ac string, dr *Domain_List_Record) (Node_List_Record, Node_List_Record) {
	var p, s Node_List_Record
	var _nodes map[uint]PW_List_Record
	var _nrecords uint

	_nodes = nodes
	_nrecords = dr.Records

	p, s = rt_db.ChooseNode(nodes, client_ac, dr)

	if p.NodeId != 0 && s.NodeId == 0 {
		if _nrecords > uint(len(p.ServerList)) {
			delete(_nodes, p.NodeId)
			s, _ = rt_db.ChooseNode(_nodes, client_ac, dr)
		}
	}

	return p, s
}

//scheduler algorithm of chosing available nodes, based on priority & weight & costs & usage(%)
func (rt_db *Route_db) ChooseNode(nodes map[uint]PW_List_Record, client_ac string, dr *Domain_List_Record) (Node_List_Record, Node_List_Record) {
	var nr, cnr, snr Node_List_Record
	//^cnr: chosen node record, isnr: secondary chosen node, nr: node record to compare
	var nid uint = 0                      //nid: chosen node id, default to 0
	var priority uint = 100               //priority: chosen node priority, default to lowest
	var weight uint = 0                   //weight: chosen node weight, default to lowest
	var weight_idle, _weight_idle float64 //weight_idle = weight*(1-usage/100)

	nr, cnr, snr = Node_List_Record{}, Node_List_Record{}, Node_List_Record{}

	for k, v := range nodes {
		nr = rt_db.Read_Node_Record(k)

		G.Outlog3(G.LOG_SCHEDULER, "Looking at node:%s(%d), p:%d w:%d u:%d c:%d s:%t",
			nr.Name, nr.NodeId, v.PW[0], v.PW[1], nr.Usage, nr.Costs, nr.Status)

		if nr.Status == false || nr.Usage >= Service_Deny_Percent {
			//not available(status algorithm) to serve(cutoff algorithm)
			G.Outlog3(G.LOG_SCHEDULER, "%s(%d) not available to serve", nr.Name, nr.NodeId)
			continue
		}
		if _, fb_matched := rt_db.Match_FB(nr.AC, dr); fb_matched {
			//node's AC match domain forbidden map's
			G.Outlog3(G.LOG_SCHEDULER, "%s(%d) match domain forbidden map, pass", nr.Name, nr.NodeId)
			continue
		}
		if nr.Status && cnr.NodeId != 0 &&
			(nr.Usage < Service_Cutoff_Percent) && (cnr.Usage > Service_Cutoff_Percent) {
			//chosen node is in cutoff state, and i am not(cutoff algorithm)
			if rt_db.Match_Local_AC(cnr.AC, client_ac) || cnr.Usage > Service_Deny_Percent {
				//cutoff none local client access, then local's
				G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is in busy, use %s(%d) instead", cnr.Name, cnr.NodeId, nr.Name, nr.NodeId)

				snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]
			}
			continue
		}
		if v.PW[0] < priority {
			//higher priority node(priority&weight algorithm)
			if priority == 100 {
				G.Outlog3(G.LOG_SCHEDULER, "%s(%d) has made default", nr.Name, nr.NodeId)
			} else {
				G.Outlog3(G.LOG_SCHEDULER, "%s(%d) has higher priority", nr.Name, nr.NodeId)
			}

			snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]

			continue
		}
		if v.PW[0] == priority {
			//equipotent priority node
			if nr.Costs < cnr.Costs {
				//which has less Costs (cost algorithm)
				G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is less costs, use it", nr.Name, nr.NodeId)

				snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]

			} else if nr.Costs == cnr.Costs {
				//same Costs
				weight_idle = float64(float64(weight) * (1.0 - float64(cnr.Usage)/100.0))
				_weight_idle = float64(float64(v.PW[1]) * (1.0 - float64(nr.Usage)/100.0))

				if _weight_idle >= weight_idle {
					//higher or same weight&&idle(weight&usage algorithm), means more idle
					G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is more idle(%f>%f), use it", nr.Name, nr.NodeId, _weight_idle, weight_idle)

					snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]
				} else {
					//lower weight_idle and not cheaper
				}
			}
			continue
		}
		if v.PW[0] > priority {
			//lower priority node
			weight_idle = float64(float64(weight) * (1.0 - float64(cnr.Usage)/100.0))
			_weight_idle = float64(float64(v.PW[1]) * (1.0 - float64(nr.Usage)/100.0))

			if _weight_idle >= weight_idle && nr.Costs < cnr.Costs {
				//higher or same weight&&idle(weight&usage algorithm) and cheaper node
				G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is more idle and less costs, use it", nr.Name)

				snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]
			} else {
				//lower weight_idle or not cheaper
			}
			continue
		}
	}

	G.Outlog3(G.LOG_SCHEDULER, "Chosen node:%s(%d), p:%d w:%d u:%d c:%d s:%t, for ac:%s. Second node:%s(%d), u:%d c:%d s:%t",
		cnr.Name, cnr.NodeId, priority, weight, cnr.Usage, cnr.Costs, cnr.Status, client_ac,
		snr.Name, snr.NodeId, snr.Usage, snr.Costs, snr.Status)

	if nid == 0 {
		//no node has been selected
		cnr, snr = Node_List_Record{}, Node_List_Record{}

	} else if cnr.NodeId == snr.NodeId {
		//first node and second node are same
		snr = Node_List_Record{}
	}

	return cnr, snr
}

//scheduler algorithm of chosing available servers, based on server (load, status), sort by weight&&idle
func (rt_db *Route_db) ChooseServer(servers []uint, servergroup uint) []uint {
	var sorted bool = false

	server_list := []uint{}
	_server_list := []uint{}

	var weight_idle, _weight_idle float64

	for _, sid := range servers {
		if rt_db.Servers[sid].Status == false || rt_db.Servers[sid].ServerGroup != servergroup {
			//server down/overload or not belong to the servergroup
			continue
		}

		weight_idle = float64(float64(rt_db.Servers[sid].Weight) * (1.0 - float64(rt_db.Servers[sid].Usage)/100.0))
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
