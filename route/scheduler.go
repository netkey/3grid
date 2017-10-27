package grid_route

import (
	IP "3grid/ip"
	G "3grid/tools/globals"
	"net"
	"strings"
	"time"
)

var MyACPrefix string
var Service_Cutoff_Percent uint
var Service_Deny_Percent uint

//check if the ip is in my server list
func (rt_db *Route_db) IN_Serverlist(ip net.IP) (uint, bool) {
	if ip != nil {
		ir := rt_db.Read_IP_Record(ip.String())
		if ir.NodeId != 0 {
			return ir.NodeId, true
		} else {
			return 0, false
		}
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

		rr = rt_db.Read_Route_Record(_ac, rid)

		//G.Outlog3(G.LOG_SCHEDULER, "AC_RR matching:%s rr:%+v", _ac, rr)

		if rr.Nodes != nil && len(rr.Nodes) > 0 {
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
		if rr.Nodes != nil && len(rr.Nodes) > 0 {
			find = true
		}
	}

	if !find {
		_ac = "*.*"
		rr = rt_db.Read_Route_Record(_ac, rid)
		if rr.Nodes != nil && len(rr.Nodes) > 0 {
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
func (rt_db *Route_db) GetAAA(query_dn string, acode string, ip net.IP,
	debug int) ([]string, uint32, string, bool, string, uint, string) {

	var ttl uint32 = 0   //domain ttl
	var rid uint = 0     //route plan id
	var aaa []string     //server ips to return
	var ac string        //client ac or mangled by Match_FB()
	var _ac string       //actually ac looking in route plan
	var client_ac string //original client ac or mangled by IN_Serverlist()
	var ok bool = true   //GetAAA status
	var _type string     //dn type
	var dn string        //actually dn matched by Match_DN() to looking at
	var sl []uint        //server list
	var _nid uint        //edge server's node id

	if debug != 2 {
		//not in ac debug mode
		if _nid, ok = rt_db.IN_Serverlist(ip); ok {
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
	} else {
		//debug mode, set ac directly
		ac = acode
	}

	ok = true
	_ac, client_ac = ac, ac
	dn = query_dn

	if aaa, ttl, _type, rid, ok = rt_db.GetRTCache(query_dn, client_ac); ok {
		//found in route cache
		if aaa == nil {
			//a fail cache result
			ok = false
		}
		return aaa, ttl, _type, ok, _ac, rid, dn
	}

	//find domain record
	dr, ok := rt_db.Match_DN(dn)

	if ok {
		ttl = uint32(dr.TTL)
		_type = dr.Type
	} else {
		return aaa, ttl, _type, false, _ac, rid, dn
	}

	if dr.Type != "" {
		//not a CDN serving domain name, typically A CNAME NS
		var spli int
		var s []string
		aaa = []string{}

		if spli = strings.Index(dr.Value, ","); spli != -1 {
			s = strings.Split(dr.Value, ",")
		} else if spli = strings.Index(dr.Value, "|"); spli != -1 {
			s = strings.Split(dr.Value, "|")
		}

		for _, x := range s {
			aaa = append(aaa, x)
		}

		return aaa, ttl, _type, ok, _ac, rid, dn
	} else {
		dn = dr.Value
	}

	if dr.Forbidden != nil {
		//match client ac in domain forbidden map and mangle it
		ac, _ = rt_db.Match_FB(ac, &dr)
	}

	var cnr, snr Node_List_Record
	var nid uint

	rr := Route_List_Record{}
	if dr.RoutePlan != nil {
		rp := append(dr.RoutePlan, 0)

		//try get route_record of current plan, get next plan if no rr, finally the default plan(0)
		for _, v := range rp {
			rid = v

			G.OutDebug("Searching route plan:%d", rid)

			//find a longest matched AC of this route plan
			_ac, rr = rt_db.Match_AC_RR(ac, rid)
			if rr.Nodes != nil && len(rr.Nodes) > 0 {

				G.OutDebug("GETAAA matched, ac: %s, rid: %d, node:%+v", _ac, rid, rr.Nodes)

				//choose primary and secondary node for serving
				cnr, snr = rt_db.ChooseNodeS(rr.Nodes, _ac, &dr)
				if nid = cnr.NodeId; nid != 0 {
					//find one
					break
				} else {
					G.OutDebug("rr:%+v is not available", rr)

					//try to search upper level
					if li := strings.LastIndex(_ac, "."); li != -1 {
						_ac = _ac[:li]
					}

					_ac, rr = rt_db.Match_AC_RR(_ac, rid)
					if rr.Nodes != nil && len(rr.Nodes) > 0 {

						G.OutDebug("GETAAA matched, ac: %s, rid: %d, node:%+v", _ac, rid, rr.Nodes)

						cnr, snr = rt_db.ChooseNodeS(rr.Nodes, _ac, &dr)
						if nid = cnr.NodeId; nid != 0 {
							//find one
							break
						} else {
							G.OutDebug("rr:%+v is not available", rr)
						}
					}
				}
			}
		}
	}

	G.Outlog3(G.LOG_ROUTE, "GETAAA ac:%s, matched ac:%s, rid:%d, noder_p:%+v node_s:%+v",
		ac, _ac, rid, cnr, snr)

	if nid == 0 {
		//cache the fail rusult for a few seconds
		rt_db.Update_Cache_Record(query_dn, client_ac,
			&RT_Cache_Record{TS: time.Now().Unix(), TTL: 5, AAA: aaa, TYPE: _type, RID: rid})

		return aaa, ttl, _type, false, _ac, rid, dn
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

	rt_db.Update_Cache_Record(query_dn, client_ac,
		&RT_Cache_Record{TS: time.Now().Unix(), TTL: ttl, AAA: aaa, TYPE: _type, RID: rid})

	return aaa, ttl, _type, true, _ac, rid, dn
}

//check if a node covered the client ac
func (rt_db *Route_db) Match_Local_AC(nr *Node_List_Record, client_ac string, check_nearby bool) bool {
	/*  node_ac   MMY.CN-CT-JX-NC-C1
	    _node_ac  MMY.CN.CT.JX.NC.C1
	    client_ac CTC.CN.JX.NC
	*/
	var node_ac string
	var _node_ac string
	var match bool = false

	node_ac = nr.AC

	if strings.Contains(node_ac, "-") {
		//convert MMY.CN-CT-JX-NC-C1 type AC to MMY.CN.CT.JX.NC.C1
		_node_ac = strings.Replace(node_ac, "-", ".", -1)
	}

	_node_ac = strings.Replace(_node_ac, "MMY.", "", -1)
	_node_ac = strings.Replace(_node_ac, ".C1", "", -1)

	//CN.CT.JX.NC
	_node_ac = strings.Replace(_node_ac, "CN.CT", "CTC.CN", -1)
	_node_ac = strings.Replace(_node_ac, "CN.CHU", "CUC.CN", -1)
	_node_ac = strings.Replace(_node_ac, "CN.CMCC", "CMCC.CN", -1)

	//CTC.CN.JX.NC
	match = strings.Contains(client_ac, _node_ac)

	if !match && check_nearby {
		if li := strings.LastIndex(_node_ac, "."); li != -1 {
			match = strings.Contains(client_ac, _node_ac[:li-1])
		}

		if !match {
			//look at the normal AC
			if nr.AC2 != "" {
				_node_ac = nr.AC2
			} else {
				if nr.ServerList != nil && len(nr.ServerList) > 0 {
					sr := rt_db.Read_Server_Record(nr.ServerList[0])
					_node_ac = IP.Ipdb.GetAreaCode(net.ParseIP(sr.ServerIp))
					nr.AC2 = _node_ac
				}
			}

			if li := strings.LastIndex(_node_ac, "."); li != -1 {
				match = strings.Contains(client_ac, _node_ac[:li-1])
			}
		}
	}

	return match
}

func (rt_db *Route_db) ChooseNodeS(nodes map[uint]PW_List_Record, client_ac string, dr *Domain_List_Record) (Node_List_Record, Node_List_Record) {
	var p, s Node_List_Record
	var _nrecords uint

	_nrecords = dr.Records

	p, s = rt_db.ChooseNode(nodes, client_ac, dr, 0)

	if p.NodeId != 0 && s.NodeId == 0 {
		if _nrecords > uint(len(p.ServerList)) {
			s, _ = rt_db.ChooseNode(nodes, client_ac, dr, p.NodeId)
		}
	}

	return p, s
}

//scheduler algorithm of chosing available nodes, based on priority & weight & costs & usage(%)
func (rt_db *Route_db) ChooseNode(nodes map[uint]PW_List_Record, client_ac string, dr *Domain_List_Record, forbid uint) (Node_List_Record, Node_List_Record) {
	var nr, cnr, snr Node_List_Record
	//^cnr: chosen node record, isnr: secondary chosen node, nr: node record to compare
	var nid uint = 0                      //nid: chosen node id, default to 0
	var priority uint = 100               //priority: chosen node priority, default to lowest
	var weight uint = 0                   //weight: chosen node weight, default to lowest
	var weight_idle, _weight_idle float64 //weight_idle = weight*(1-usage/100)

	nr, cnr, snr = Node_List_Record{}, Node_List_Record{}, Node_List_Record{}

	for k, v := range nodes {

		if k == forbid {
			continue
		}

		nr = rt_db.Read_Node_Record(k)

		G.Outlog3(G.LOG_SCHEDULER, "Looking at node:%s(%d), p:%d w:%d u:%d c:%d s:%t ac:%s",
			nr.Name, nr.NodeId, v.PW[0], v.PW[1], nr.Usage, nr.Costs, nr.Status, nr.AC)

		if nr.Status == false || nr.Usage >= Service_Deny_Percent ||
			nr.Priority == 0 || v.PW[0] == 0 { //priority==0 means node disabled
			//not available(status algorithm) to serve(cutoff algorithm)
			G.Outlog3(G.LOG_SCHEDULER, "%s(%d) not available to serve", nr.Name, nr.NodeId)
			continue
		}
		if _, fb_matched := rt_db.Match_FB(nr.AC, dr); fb_matched {
			//node's AC match domain forbidden map's
			G.Outlog3(G.LOG_SCHEDULER, "%s(%d) match domain forbidden map, pass it", nr.Name, nr.NodeId)
			continue
		}
		if nr.Status && cnr.NodeId != 0 &&
			(nr.Usage < Service_Cutoff_Percent) && (cnr.Usage > Service_Cutoff_Percent) {
			//chosen node is in cutoff state, and i am not(cutoff algorithm)
			if !rt_db.Match_Local_AC(&cnr, client_ac, false) || cnr.Usage > Service_Deny_Percent {
				//cutoff none local client access, then local's
				G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is in busy, use %s(%d) instead",
					cnr.Name, cnr.NodeId, nr.Name, nr.NodeId)

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

				if _weight_idle > weight_idle {
					//higher weight&&idle(weight&usage algorithm), means more idle
					G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is more idle(%f>%f), use it",
						nr.Name, nr.NodeId, _weight_idle, weight_idle)

					snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]
				} else {
					//lower or same weight_idle and not cheaper
					//if client and node are in same major area, use it first
					if rt_db.Match_Local_AC(&cnr, client_ac, true) {
						G.Outlog3(G.LOG_SCHEDULER, "%s(%d) is nearby client(%s), use it",
							nr.Name, nr.NodeId, client_ac)

						snr, cnr, nid, priority, weight = cnr, nr, k, v.PW[0], v.PW[1]
					}
				}
			} else if nr.Costs > cnr.Costs {
				//higher Costs
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

	if cnr.NodeId != 0 && snr.NodeId != 0 {
		G.Outlog3(G.LOG_SCHEDULER,
			"Chosen node:%s(%d) p:%d w:%d u:%d c:%d s:%t, second node:%s(%d) u:%d c:%d s:%t, for ac:%s",
			cnr.Name, cnr.NodeId, priority, weight, cnr.Usage, cnr.Costs, cnr.Status,
			snr.Name, snr.NodeId, snr.Usage, snr.Costs, snr.Status, client_ac)
	} else if cnr.NodeId != 0 && snr.NodeId == 0 {
		G.Outlog3(G.LOG_SCHEDULER, "Chosen node:%s(%d) p:%d w:%d u:%d c:%d s:%t",
			cnr.Name, cnr.NodeId, priority, weight, cnr.Usage, cnr.Costs, cnr.Status)
	} else if cnr.NodeId == 0 && snr.NodeId == 0 {
		G.Outlog3(G.LOG_SCHEDULER, "No further node being chosen")
	}

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

		weight_idle = float64(float64(rt_db.Servers[sid].Weight) *
			(1.0 - float64(rt_db.Servers[sid].Usage)/100.0))

		sorted = false

		for i, _sid := range server_list {
			//sort by weight&&idle, weight*(1-usage/100)
			_weight_idle = float64(rt_db.Servers[_sid].Weight *
				(1 - rt_db.Servers[_sid].Usage/100))

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
