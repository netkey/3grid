package grid_route

import G "3grid/tools/globals"
import "encoding/json"
import "io/ioutil"
import "log"
import "net"
import "reflect"
import "strconv"
import "strings"
import "sync"

//route db version and file path
var RT_Version string
var RT_Db_file string
var RT_Ver_Major, RT_Ver_Minor, RT_Ver_Patch uint64

//domain db version and file path
var DM_Version string
var DM_Db_file string
var DM_Ver_Major, DM_Ver_Minor, DM_Ver_Patch uint64

//cmdb version and file path
var CM_Version string
var CM_Db_file string
var CM_Ver_Major, CM_Ver_Minor, CM_Ver_Patch uint64

//use for route/cm/domain db data update
var Chan *chan map[string]map[string][]string

/*Route DB update method

domain/servers/nodes update record : map[string][]string

domain_name(key) : [ type value priority servergroup_id records ttl route_plan_list ]
image16-c.poco.cn:["" "image16-c.poco.cn.mmycdn.com" "10" "1" "3" "300" "8,9.10"]

node_id(key) : [priority, weight, costs, usage, status, bandwidth, type, server_id_list]
155:["1" "10" "0" "3000" "0" "0" "A" "140,141,"]

server_id(key) : [ip, usage, servergroup_id, weight, status, node_id]
140:["211.162.56.33" "0" "1" "1" "1" "155"]

area_code(key) : [node_id, route_plan_id, priority, weight]
MMY.MUT-TJ-TJ-C1-CHU.*:["42" "788" "3" "0"]

update:
RT.Chan <- map[string]map[string][]string{"domains": domain_records}
RT.Chan <- map[string]map[string][]string{"servers": server_records}
RT.Chan <- map[string]map[string][]string{"nodes": node_records}
RT.Chan <- map[string]map[string][]string{"routes": route_records}

delete:
same as update, set the records value to nil
*/

type Route_db struct {
	Servers map[uint]Server_List_Record           //uint for server_id
	Ips     map[string]Server_List_Record         //string for server_ip(net.IP.String())
	Nodes   map[uint]Node_List_Record             //uint for nodeid
	Domains map[string]Domain_List_Record         //string for domain name
	Routes  map[string]map[uint]Route_List_Record //string for AreaCode, uint for RoutePlan ID
	Cache   map[string]map[string]RT_Cache_Record //string for AreaCode, string for domain name
	Chan    chan map[string]map[string][]string   //channel for updating dbs
	Locks   map[string]*sync.RWMutex              //locks for writing dbs
}

type Server_List_Record struct {
	ServerId       uint   //Id of server
	ServerIp       string //IP address of server
	ServerGroup    uint   //description of server type, like 'page', 'video'
	NodeId         uint   //same as nodeid in Node_List_Record
	ServerCapacity uint64 //bandwidth
	Usage          uint   //percentage
	Status         bool   //1:ok 0:fail
	Weight         uint   //responde to ServerCapacity
}

type Node_List_Record struct {
	NodeId       uint   //node id
	Name         string //node name
	NodeCapacity uint64 //bandwidth
	Usage        uint   //percentage
	Status       bool   //1:ok 0:fail
	Priority     uint   //network quality
	Weight       uint   //responde to number of servers
	Costs        int    //cost of node(95), negative value representing that it doesn't reach minimum-guarantee
	Type         string //node type: A CNAME
	ServerList   []uint //server list in ids
}

type Domain_List_Record struct {
	Name        string //domain name
	Type        string //A CNAME NS CDN
	Value       string //value of A/NS/CNAME
	Priority    string //domain class
	ServerGroup string //server group serving this domain
	Records     uint   //A records return once
	TTL         uint   //TTL of A
	RoutePlan   []uint //RP serving this domain
}

type Route_List_Record struct {
	Nodes map[uint]PW_List_Record //uint for node id
}

type PW_List_Record struct {
	PW [2]uint //PW[0] for priority, PW[1] for weight
}

type RT_Cache_Record struct {
	TTL uint32
	AAA []string
}

func (rt_db *Route_db) RT_db_init() {
	rt_db.Servers = make(map[uint]Server_List_Record)
	rt_db.Ips = make(map[string]Server_List_Record)
	rt_db.Nodes = make(map[uint]Node_List_Record)
	rt_db.Domains = make(map[string]Domain_List_Record)
	rt_db.Routes = make(map[string]map[uint]Route_List_Record)
	rt_db.Cache = make(map[string]map[string]RT_Cache_Record)

	rt_db.Locks = map[string]*sync.RWMutex{"servers": new(sync.RWMutex), "ips": new(sync.RWMutex),
		"nodes": new(sync.RWMutex), "domains": new(sync.RWMutex),
		"routes": new(sync.RWMutex), "cache": new(sync.RWMutex)}
	rt_db.Chan = make(chan map[string]map[string][]string, 100)
	Chan = &rt_db.Chan

	go rt_db.Updatedb()

	rt_db.LoadDomaindb()
	rt_db.LoadCMdb()
	rt_db.LoadRoutedb()
}

func (rt_db *Route_db) LoadDomaindb() error {
	var jf []byte
	var domain_records map[string][]string
	var err error

	if G.Debug {
		log.Printf("Loading domain db..")
	}

	jf, err = ioutil.ReadFile(DM_Db_file)
	if err != nil {
		if G.Debug {
			log.Printf("error reading domain db: %s", err)
		}
	}

	err = json.Unmarshal(jf, &domain_records)
	if err != nil {
		if G.Debug {
			log.Printf("error unmarshaling domain db: %s", err)
		}
	} else {
		rt_db.Convert_Domain_Record(&domain_records)
		//can also update in this way:
		//rt_db.Chan <- map[string]map[string][]string{"domains": domain_records}
	}

	if G.Debug {
		//time.Sleep(time.Duration(100) * time.Millisecond) //for use with channel updating
		keys := reflect.ValueOf(rt_db.Domains).MapKeys()
		log.Printf("domains data sample: %+v", rt_db.Domains[keys[0].String()])
	}

	return err
}

func (rt_db *Route_db) LoadCMdb() error {
	var jf []byte
	var cmdb_records map[string]map[string][]string
	var err error

	if G.Debug {
		log.Printf("Loading cmdb..")
	}

	jf, err = ioutil.ReadFile(CM_Db_file)
	if err != nil {
		if G.Debug {
			log.Printf("error reading cmdb: %s", err)
		}
	}

	err = json.Unmarshal(jf, &cmdb_records)
	if err != nil {
		if G.Debug {
			log.Printf("error unmarshaling cmdb: %s", err)
		}
	} else {
		for nid, servers := range cmdb_records {
			node_records := make(map[string][]string)
			server_records := make(map[string][]string)
			server_list := ""
			for sid, server := range servers {
				if sid == "0" {
					node_records[nid] = server
				} else {
					server_records[sid] = server
					server_records[sid] = append(server_records[sid], nid)
					server_list = server_list + sid + ","
				}
			}
			node_records[nid] = append(node_records[nid], server_list)

			rt_db.Convert_Node_Record(&node_records)
			rt_db.Convert_Server_Record(&server_records)
		}
	}

	if G.Debug {
		keys := reflect.ValueOf(rt_db.Nodes).MapKeys()
		log.Printf("nodes data sample: %+v", rt_db.Nodes[uint(keys[0].Uint())])
		keys = reflect.ValueOf(rt_db.Servers).MapKeys()
		log.Printf("servers data sample: %+v", rt_db.Servers[uint(keys[0].Uint())])
		keys = reflect.ValueOf(rt_db.Ips).MapKeys()
		log.Printf("ips data sample: %+v", rt_db.Ips[keys[0].String()])
	}

	return err
}

func (rt_db *Route_db) LoadRoutedb() error {
	var jf []byte
	var rtdb_records map[string]map[string]map[string][]string
	var route_records map[string][]string
	var err error

	if G.Debug {
		log.Printf("Loading route db..")
	}

	jf, err = ioutil.ReadFile(RT_Db_file)
	if err != nil {
		if G.Debug {
			log.Printf("error reading route db: %s", err)
		}
	}

	err = json.Unmarshal(jf, &rtdb_records)
	if err != nil {
		if G.Debug {
			log.Printf("error unmarshaling route db: %s", err)
		}
	} else {
		//CMCC-CN-BJ-BJ : [rid, nid, p, w]
		for rid, plan := range rtdb_records {
			if plan == nil || rid == "" {
				continue
			}
			for ac, nodes := range plan {
				if nodes == nil || ac == "" {
					continue
				}
				for nid, pws := range nodes {
					route_records = make(map[string][]string)
					if pws == nil || nid == "" {
						continue
					}
					route_records[ac] = append(route_records[ac], rid)
					route_records[ac] = append(route_records[ac], nid)
					route_records[ac] = append(route_records[ac], pws[0])
					route_records[ac] = append(route_records[ac], pws[1])
					rt_db.Convert_Route_Record(&route_records)
				}
			}
		}
	}

	if G.Debug {
		keys := reflect.ValueOf(rt_db.Routes).MapKeys()
		log.Printf("routes data sample: %s %+v", keys[0].String(), rt_db.Routes[keys[0].String()])
	}

	return err
}

//Tag: xxx
func (rt_db *Route_db) Updatedb() error {
	for {
		m := <-rt_db.Chan
		for t, a := range m {
			switch t {
			case "domains":
				rt_db.Convert_Domain_Record(&a)
			case "servers":
				rt_db.Convert_Server_Record(&a)
			case "nodes":
				rt_db.Convert_Node_Record(&a)
			case "routes":
				rt_db.Convert_Route_Record(&a)
			default:
			}
		}
	}

	return nil
}

func (rt_db *Route_db) Convert_Domain_Record(m *map[string][]string) {
	for k, v := range *m {
		x := 0
		r := new(Domain_List_Record)

		if len(v) > 6 {
			r.Name = k
			r.Type = v[0]
			r.Value = v[1]
			r.Priority = v[2]
			r.ServerGroup = v[3]
			x, _ = strconv.Atoi(v[4])
			r.Records = uint(x)
			x, _ = strconv.Atoi(v[5])
			r.TTL = uint(x)
			s := strings.Split(v[6], ",")
			if len(s) > 0 {
				p := make([]uint, len(s))
				for i, v := range s {
					x, _ = strconv.Atoi(v)
					p[i] = uint(x)
				}
				r.RoutePlan = p
			}
			rt_db.Update_Domain_Record(k, r)
			if G.Debug {
				//log.Printf("update domain record: %+v", r)
			}
		}
	}
}

func (rt_db *Route_db) Read_Domain_Record(k string) Domain_List_Record {
	var r Domain_List_Record

	rt_db.Locks["domains"].RLock()
	r = rt_db.Domains[k]
	rt_db.Locks["domains"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Domain_Record(k string, r *Domain_List_Record) {
	rt_db.Locks["domains"].Lock()
	if r == nil {
		delete(rt_db.Domains, k)
	} else {
		rt_db.Domains[k] = *r
	}
	rt_db.Locks["domains"].Unlock()
}

func (rt_db *Route_db) Convert_Server_Record(m *map[string][]string) {
	for k, v := range *m {
		x := 0
		r := new(Server_List_Record)

		if len(v) > 5 {
			x, _ = strconv.Atoi(k)
			r.ServerId = uint(x)
			r.ServerIp = v[0]
			x, _ = strconv.Atoi(v[1])
			r.Usage = uint(x)
			x, _ = strconv.Atoi(v[2])
			r.ServerGroup = uint(x)
			x, _ = strconv.Atoi(v[3])
			r.Weight = uint(x)
			if v[4] == "1" {
				r.Status = true
			} else {
				r.Status = false
			}
			x, _ = strconv.Atoi(v[5])
			r.NodeId = uint(x)

			rt_db.Update_Server_Record(r.ServerId, r)
			if G.Debug {
				//log.Printf("update server record: %+v", r)
			}
		}
	}
}

func (rt_db *Route_db) Read_Server_Record(k uint) Server_List_Record {
	var r Server_List_Record

	rt_db.Locks["servers"].RLock()
	r = rt_db.Servers[k]
	rt_db.Locks["servers"].RUnlock()

	return r
}

func (rt_db *Route_db) Read_IP_Record(k string) Server_List_Record {
	var r Server_List_Record

	rt_db.Locks["ips"].RLock()
	r = rt_db.Ips[k]
	rt_db.Locks["ips"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Server_Record(k uint, r *Server_List_Record) {
	rt_db.Locks["servers"].Lock()
	if r == nil {
		delete(rt_db.Servers, k)
		delete(rt_db.Ips, (*r).ServerIp)
	} else {
		rt_db.Servers[k] = *r
		rt_db.Ips[(*r).ServerIp] = *r
	}
	rt_db.Locks["servers"].Unlock()
}

func (rt_db *Route_db) Convert_Node_Record(m *map[string][]string) {
	for k, v := range *m {
		x := 0
		r := new(Node_List_Record)

		if len(v) > 8 {
			x, _ = strconv.Atoi(k)
			r.NodeId = uint(x)
			r.Name = v[0]
			x, _ = strconv.Atoi(v[1])
			r.Priority = uint(x)
			x, _ = strconv.Atoi(v[2])
			r.Weight = uint(x)
			x, _ = strconv.Atoi(v[3])
			r.Costs = int(x)
			x, _ = strconv.Atoi(v[4])
			r.Usage = uint(x)
			if v[5] == "1" {
				r.Status = true
			} else {
				r.Status = false
			}
			x, _ = strconv.Atoi(v[6])
			r.NodeCapacity = uint64(x)
			r.Type = v[7]
			s := strings.Split(v[8], ",")
			if len(s) > 0 {
				p := make([]uint, len(s)-1)
				for i, v := range s {
					x, _ = strconv.Atoi(v)
					if x != 0 {
						p[i] = uint(x)
					}
				}
				r.ServerList = p
			}

			rt_db.Update_Node_Record(r.NodeId, r)
			if G.Debug {
				//log.Printf("update node record: %+v", r)
			}
		}
	}
}

func (rt_db *Route_db) Read_Node_Record(k uint) Node_List_Record {
	var r Node_List_Record

	rt_db.Locks["nodes"].RLock()
	r = rt_db.Nodes[k]
	if G.Debug {
		//log.Printf("Nodes: %+v", rt_db.Nodes)
	}
	rt_db.Locks["nodes"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Node_Record(k uint, r *Node_List_Record) {
	rt_db.Locks["nodes"].Lock()
	if r == nil {
		delete(rt_db.Nodes, k)
	} else {
		rt_db.Nodes[k] = *r
	}
	rt_db.Locks["nodes"].Unlock()
}

func (rt_db *Route_db) Convert_Route_Record(m *map[string][]string) {
	var rid uint
	var nid uint
	for k, v := range *m {
		x := 0
		r := new(Route_List_Record)
		pw := new(PW_List_Record)
		r.Nodes = make(map[uint]PW_List_Record)

		if len(v) > 3 {
			x, _ = strconv.Atoi(v[0])
			rid = uint(x)
			x, _ = strconv.Atoi(v[1])
			nid = uint(x)
			x, _ = strconv.Atoi(v[2])
			pw.PW[0] = uint(x)
			x, _ = strconv.Atoi(v[3])
			pw.PW[1] = uint(x)

			r.Nodes[nid] = *pw

			rt_db.Update_Route_Record(k, rid, r)
			if G.Debug {
				//log.Printf("update route record: %s:%d:%+v", k, nid, r)
			}
		}
	}
}

func (rt_db *Route_db) Read_Route_Record(k string, rid uint) Route_List_Record {
	var r Route_List_Record

	rt_db.Locks["routes"].RLock()
	r = rt_db.Routes[k][rid]
	rt_db.Locks["routes"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Route_Record(k string, rid uint, r *Route_List_Record) {
	rt_db.Locks["routes"].Lock()
	if r == nil {
		delete(rt_db.Routes[k], rid)
	} else {
		if rt_db.Routes[k] == nil {
			rt_db.Routes[k] = make(map[uint]Route_List_Record)
		}
		rt_db.Routes[k][rid] = *r
	}
	rt_db.Locks["routes"].Unlock()
}

func (rt_db *Route_db) Read_Cache_Record(dn string, ac string) RT_Cache_Record {
	var r RT_Cache_Record

	rt_db.Locks["cache"].RLock()
	if rt_db.Cache[ac] != nil {
		if rt_db.Cache[ac][dn].TTL != 0 {
			r = rt_db.Cache[ac][dn]
		}
	}
	rt_db.Locks["cache"].RUnlock()

	return r
}

func (rt_db *Route_db) GetRTCache(dn string, ac string) ([]string, uint32, bool) {
	var r RT_Cache_Record

	r = rt_db.Read_Cache_Record(dn, ac)

	if r.TTL != 0 {
		return r.AAA, r.TTL, true
	} else {
		return nil, 0, false
	}
}

func (rt_db *Route_db) Update_Cache_Record(dn string, ac string, r *RT_Cache_Record) {
	rt_db.Locks["cache"].Lock()
	if r == nil {
		delete(rt_db.Cache[ac], dn)
	} else {
		if rt_db.Cache[ac] == nil {
			rt_db.Cache[ac] = make(map[string]RT_Cache_Record)
		}
		rt_db.Cache[ac][dn] = *r
	}
	rt_db.Locks["cache"].Unlock()
}

//Tag: AAA
func (rt_db *Route_db) GetAAA(dn string, _ac string, ip net.IP) ([]string, uint32) {
	var ttl uint32 = 0
	var rid uint = 0
	var aaa []string
	var ac string
	var ok bool

	ir := rt_db.Read_IP_Record(ip.String())
	if ir.NodeId != 0 {
		irn := rt_db.Read_Node_Record(ir.NodeId)
		if irn.Name != "" {
			ac = "MMY." + irn.Name
		} else {
			ac = _ac
		}
	} else {
		ac = _ac
	}
	if G.Debug {
		log.Printf("GETAAA dn:%s, ac:%s, ip:%s", dn, ac, ip.String())
	}

	if aaa, ttl, ok = rt_db.GetRTCache(dn, ac); ok {
		//found in route cache
		return aaa, ttl
	}

	dr := rt_db.Read_Domain_Record(dn)
	if G.Debug {
		log.Printf("GETAAA dr: %+v", dr)
	}
	ttl = uint32(dr.TTL)

	if dr.RoutePlan != nil {
		for _, v := range dr.RoutePlan {
			if true { //for debug
				rid = v
				break //for debug
			}
		}
	}

	rr := rt_db.Read_Route_Record(ac, rid)
	if G.Debug {
		log.Printf("GETAAA ac: %s, rid: %d, rr: %+v", ac, rid, rr)
	}

	if true {
		aaa = make([]string, dr.Records)
		for nid, _ := range rr.Nodes {
			if true {
				nr := rt_db.Read_Node_Record(224) //for debug
				if G.Debug {
					log.Printf("GETAAA nid: %d, nr: %+v", nid, nr)
				}
				for i, sid := range nr.ServerList {
					if true {
						sr := rt_db.Read_Server_Record(sid)
						if G.Debug {
							log.Printf("GETAAA sr: %+v", sr)
						}
						aaa[i] = sr.ServerIp
					}
				}
				break
			}
		}
	}

	rt_db.Update_Cache_Record(dn, ac, &RT_Cache_Record{TTL: ttl, AAA: aaa})
	return aaa, ttl
}
