package grid_route

import G "3grid/tools/globals"
import "log"
import "net"
import "strings"

//check if the ip is in my server list
func (rt_db *Route_db) IN_Serverlist(ip net.IP) (uint, bool) {
	ir := rt_db.Read_IP_Record(ip.String())
	if ir.NodeId != 0 {
		return ir.NodeId, true
	} else {
		return 0, false
	}
}

//return A IPs based on AreaCode and DomainName
func (rt_db *Route_db) GetAAA(dn string, acode string, ip net.IP) ([]string, uint32, string, bool) {
	var ttl uint32 = 0
	var rid uint = 0
	var aaa []string
	var ac string
	var ok bool = true
	var _type string

	if _nid, ok := rt_db.IN_Serverlist(ip); ok {
		//it's my server, change the area code to its node
		irn := rt_db.Read_Node_Record(_nid)
		if irn.Name != "" {
			ac = "MMY." + irn.Name
		} else {
			ac = acode
		}
	} else {
		ac = acode
	}

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
			rr = rt_db.Read_Route_Record(ac, rid)
			if G.Debug {
				log.Printf("GETAAA ac: %s, rid: %d, rr: %+v", ac, rid, rr)
			}
			if rr.Nodes != nil {
				break
			}
		}
	} else {
		rid = 0 //default route plan
		rr = rt_db.Read_Route_Record(ac, rid)
		if rr.Nodes == nil {
			return aaa, ttl, _type, false
		}
	}

	nr := rt_db.ChoseNode(rr.Nodes)
	//nr := rt_db.Read_Node_Record(224) //for debug
	nid := nr.NodeId

	if G.Debug {
		log.Printf("GETAAA nid: %d, nr: %+v", nid, nr)
	}

	sl := rt_db.ChoseServer(nr.ServerList)

	aaa = make([]string, dr.Records)
	for i, sid := range sl {
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
	return Node_List_Record{}
}

//scheduler algorithm of chosing available servers, based on server (load, status)
func (rt_db *Route_db) ChoseServer(servers []uint) []uint {
	server_list := []uint{}

	for _, sid := range servers {
		if rt_db.Servers[sid].Status == false {
			//server down or overload
			continue
		}
		server_list = append(server_list, sid)
	}

	return server_list
}
