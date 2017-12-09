package grid_route

import (
	G "3grid/tools/globals"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

//route db version and file path
var RT_Version string
var RT_Db_file string
var RT_Db_file0 string
var RT_Ver_Major, RT_Ver_Minor, RT_Ver_Patch uint64

//domain db version and file path
var DM_Version string
var DM_Db_file string
var DM_Db_file0 string
var DM_Ver_Major, DM_Ver_Minor, DM_Ver_Patch uint64

//cmdb version and file path
var CM_Version string
var CM_Db_file string
var CM_Db_file0 string
var CM_Ver_Major, CM_Ver_Minor, CM_Ver_Patch uint64

//use for route/cm/domain db data update
var Chan *chan map[string]map[string]map[string]map[string][]string

var Rtdb *Route_db

var RT_Cache_TTL int
var RR_Cache_TTL int
var RT_Cache_Size int64

const (
	PERF_DOMAIN = "domain"
)

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
*RT.Chan <- map[string]map[string][]string{"domains": domain_records}
*RT.Chan <- map[string]map[string][]string{"servers": server_records}
*RT.Chan <- map[string]map[string][]string{"nodes": node_records}
*RT.Chan <- map[string]map[string][]string{"routes": route_records}

delete:
same as update, set the records value to nil
*/

type Route_db struct {
	Chan chan map[string]map[string]map[string]map[string][]string
	//^ channel for updating dbs
	Servers   map[uint]Server_List_Record           //uint for server_id
	Ips       map[string]Server_List_Record         //string for server_ip(net.IP.String())
	Nodes     map[uint]Node_List_Record             //uint for nodeid
	NodeACs   map[string]Node_List_Record           //string for node AC
	Domains   map[string]Domain_List_Record         //string for domain name
	Routes    map[string]map[uint]Route_List_Record //string for AreaCode, uint for RoutePlan ID
	Cache     map[string]map[string]RT_Cache_Record //string for AreaCode, string for domain name
	Nets      map[uint]map[uint]Net_Perf_Record     //uint for source node id, uint for target node id
	Locks     map[string]*sync.RWMutex              //locks for writing dbs
	CacheSize int64                                 //cache limit
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
	AC           string //node AreaCode
	AC2          string //node AreaCode (normal)
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
	Name        string          //domain name
	Type        string          //A CNAME NS CDN
	Value       string          //value of A/NS/CNAME
	Priority    string          //domain class
	ServerGroup uint            //server group serving this domain
	Records     uint            //A records return once
	TTL         uint            //TTL of A
	RoutePlan   []uint          //RP serving this domain
	Status      uint            //1:enable 0:disable
	Forbidden   map[string]uint //map of ACs to forbid serve(audit)
}

type Route_List_Record struct {
	Nodes map[uint]PW_List_Record //uint for node id
}

type PW_List_Record struct {
	PW [2]uint //PW[0] for Priority, PW[1] for Weight
}

type RT_Cache_Record struct {
	TS     int64    //Cache Time Stamp
	TTL    uint32   //Time To Live(cache expire time)
	RR_TTL uint32   //Random RR TTL
	AAA    []string //A records of IP
	TYPE   string   //Answer Type(A/CNAME/NS..)
	RID    uint     //Route plan ID
	MAC    string   //Matched AC
}

type Net_Perf_Record struct {
	RTT uint //Round Trip Time(ping)
	DS  uint //Download Speed(kbps)
}

func (rt_db *Route_db) RT_db_init() {
	rt_db.Servers = make(map[uint]Server_List_Record)
	rt_db.Ips = make(map[string]Server_List_Record)
	rt_db.Nodes = make(map[uint]Node_List_Record)
	rt_db.NodeACs = make(map[string]Node_List_Record)
	rt_db.Domains = make(map[string]Domain_List_Record)
	rt_db.Routes = make(map[string]map[uint]Route_List_Record)
	rt_db.Cache = make(map[string]map[string]RT_Cache_Record)
	rt_db.Nets = make(map[uint]map[uint]Net_Perf_Record)

	rt_db.Locks = map[string]*sync.RWMutex{"servers": new(sync.RWMutex), "ips": new(sync.RWMutex),
		"nodes": new(sync.RWMutex), "domains": new(sync.RWMutex), "nets": new(sync.RWMutex),
		"routes": new(sync.RWMutex), "cache": new(sync.RWMutex), "nodeacs": new(sync.RWMutex)}

	rt_db.Chan = make(chan map[string]map[string]map[string]map[string][]string, 1000)
	Chan = &rt_db.Chan

	go rt_db.Updatedb()

	rt_db.LoadDomaindb(nil)
	rt_db.LoadCMdb(nil)
	rt_db.LoadRoutedb(nil)
}

func (rt_db *Route_db) LoadDomaindb(_domain_records map[string][]string) error {
	var jf []byte
	var domain_records map[string][]string
	var err error

	if _domain_records == nil {

		G.OutDebug("Loading domain db..%s", DM_Db_file)

		jf, err = ioutil.ReadFile(DM_Db_file)
		if err != nil {
			G.OutDebug("error reading domain db: %s", err)
		}

		err = json.Unmarshal(jf, &domain_records)
		if err != nil {
			G.OutDebug("error unmarshaling domain db: %s", err)
		}
	} else {
		domain_records = _domain_records
	}

	if err == nil {
		rt_db.Convert_Domain_Record(domain_records)

		if G.Debug && _domain_records == nil {
			//time.Sleep(time.Duration(100) * time.Millisecond) //for use with channel updating
			keys := reflect.ValueOf(rt_db.Domains).MapKeys()
			G.OutDebug("domains data sample: %+v", rt_db.Domains[keys[0].String()])
		}
	}

	return err
}

//Tag: CCC
func (rt_db *Route_db) LoadCMdb(_cmdb_records map[string]map[string][]string) error {
	var jf []byte
	var cmdb_records map[string]map[string][]string
	var err error

	if _cmdb_records == nil {

		G.OutDebug("Loading cmdb..%s", CM_Db_file)

		jf, err = ioutil.ReadFile(CM_Db_file)
		if err != nil {
			G.OutDebug("error reading cmdb: %s", err)
		}

		err = json.Unmarshal(jf, &cmdb_records)
		if err != nil {
			G.OutDebug("error unmarshaling cmdb: %s", err)
		}
	} else {
		cmdb_records = _cmdb_records
	}

	if err == nil {
		for nid, servers := range cmdb_records {
			if servers != nil {
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

				rt_db.Convert_Node_Record(node_records)
				rt_db.Convert_Server_Record(server_records)
			} else {
				rt_db.Convert_Node_Record(map[string][]string{nid: nil})

				server_records := make(map[string][]string)
				x, _ := strconv.Atoi(nid)
				xnid := uint(x)
				rt_db.Locks["servers"].RLock()
				for sid, server := range rt_db.Servers {
					if server.NodeId == xnid {
						server_records[strconv.Itoa(int(sid))] = nil
					}
				}
				rt_db.Locks["servers"].RUnlock()
				rt_db.Convert_Server_Record(server_records)
			}
		}

		if G.Debug && _cmdb_records == nil {
			keys := reflect.ValueOf(rt_db.Nodes).MapKeys()
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("nodes data sample: %+v",
				rt_db.Nodes[uint(keys[0].Uint())]))

			/*keys = reflect.ValueOf(rt_db.NodeACs).MapKeys()
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("nodeacs data sample: %+v",
				rt_db.NodeACs[keys[0].String()]))
			*/
			keys = reflect.ValueOf(rt_db.Servers).MapKeys()
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("servers data sample: %+v",
				rt_db.Servers[uint(keys[0].Uint())]))

			keys = reflect.ValueOf(rt_db.Ips).MapKeys()
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("ips data sample: %+v",
				rt_db.Ips[keys[0].String()]))
		}
	}

	return err
}

func (rt_db *Route_db) LoadRoutedb(_rtdb_records map[string]map[string]map[string][]string) error {
	var jf []byte
	var rtdb_records map[string]map[string]map[string][]string
	var route_records map[string][]string
	var err error

	if _rtdb_records == nil {
		G.OutDebug("Loading route db..%s", RT_Db_file)

		jf, err = ioutil.ReadFile(RT_Db_file)
		if err != nil {
			G.OutDebug("error reading route db: %s", err)
		}

		err = json.Unmarshal(jf, &rtdb_records)
		if err != nil {
			G.OutDebug("error unmarshaling route db: %s", err)
		}
	} else {
		rtdb_records = _rtdb_records
	}

	if err == nil {
		//CMCC-CN-BJ-BJ : [rid, nid, p, w]
		for rid, plan := range rtdb_records {
			if plan != nil && rid != "" {
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
						rt_db.Convert_Route_Record(route_records)
					}
				}
			} else if plan == nil {
				if rid != "" {
					rt_db.Convert_Route_Record(map[string][]string{rid: nil})
				}
			}
		}

		if G.Debug && _rtdb_records == nil {
			keys := reflect.ValueOf(rt_db.Routes).MapKeys()
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("routes data sample: %s %+v",
				keys[0].String(), rt_db.Routes[keys[0].String()]))
		}
	}

	return err
}

func (rt_db *Route_db) Updatedb() error {
	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic Update_Routedb: %s", pan)
		}
	}()

	for {
		m := <-rt_db.Chan
		for t, a := range m {
			switch t {
			case "domains":
				rt_db.Convert_Domain_Record(a["domains"]["domains"])
			case "servers":
				rt_db.Convert_Server_Record(a["servers"]["servers"])
			case "nodes":
				rt_db.Convert_Node_Record(a["nodes"]["nodes"])
			case "routes":
				rt_db.Convert_Route_Record(a["routes"]["routes"])
			case "Cmdb":
				rt_db.LoadCMdb(a["Cmdb"])
			case "Domain":
				rt_db.LoadDomaindb(a["Domain"]["Domain"])
			case "Routedb":
				rt_db.LoadRoutedb(a)
			default:
			}
		}
	}

	return nil
}

func (rt_db *Route_db) Convert_Domain_Record(m map[string][]string) {
	for k, v := range m {
		x := 0
		r := new(Domain_List_Record)

		if v != nil {
			if len(v) > 6 {
				r.Name = k
				r.Type = strings.ToUpper(v[0])
				r.Value = v[1]
				r.Priority = v[2]
				x, _ = strconv.Atoi(v[3])
				r.ServerGroup = uint(x)
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
				r.Status = 1
				if len(v) > 7 {
					x, _ = strconv.Atoi(v[7])
					r.Status = uint(x)

					if v[8] != "" {
						r.Forbidden = make(map[string]uint)
						for _, fac := range strings.Split(v[8], ",") {
							r.Forbidden[fac] = 1
						}
					}

				}

				//for CDN client query name is r.Value, for regular is k
				if r.Type == "" {
					rt_db.Update_Domain_Record(r.Value, r)
				} else {
					rt_db.Update_Domain_Record(k, r)
				}
			}
		} else {
			rt_db.Update_Domain_Record(k, nil)
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

//Tag: SSS
func (rt_db *Route_db) Convert_Server_Record(m map[string][]string) {
	for k, v := range m {
		x := 0
		r := new(Server_List_Record)

		if v != nil {
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
			} else {
				rt_db.Update_Server_Record(r.ServerId, nil)
			}
		} else {
			x, _ = strconv.Atoi(k)
			rt_db.Update_Server_Record(uint(x), nil)
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
	rt_db.Locks["ips"].Lock()
	if r == nil {
		rt_db.Locks["servers"].RLock()
		delete(rt_db.Ips, rt_db.Servers[k].ServerIp)
		rt_db.Locks["servers"].RUnlock()
	} else {
		rt_db.Ips[(*r).ServerIp] = *r
	}
	rt_db.Locks["ips"].Unlock()

	rt_db.Locks["servers"].Lock()
	if r == nil {
		delete(rt_db.Servers, k)
	} else {
		rt_db.Servers[k] = *r
	}
	rt_db.Locks["servers"].Unlock()
}

//Tag: NNN
func (rt_db *Route_db) Convert_Node_Record(m map[string][]string) {
	for k, v := range m {
		x := 0
		r := new(Node_List_Record)

		if v != nil {
			if len(v) > 9 {
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
				r.AC = v[8]
				if r.AC == "" {
					r.AC = r.Name
				}
				s := strings.Split(v[9], ",")
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
			} else {
				rt_db.Update_Node_Record(r.NodeId, nil)
			}
		} else {
			x, _ = strconv.Atoi(k)
			rt_db.Update_Node_Record(uint(x), nil)
		}
	}
}

func (rt_db *Route_db) Read_Node_Record(k uint) Node_List_Record {
	var r Node_List_Record

	rt_db.Locks["nodes"].RLock()
	r = rt_db.Nodes[k]
	rt_db.Locks["nodes"].RUnlock()

	return r
}

func (rt_db *Route_db) Read_Node_Record_AC(ac string) Node_List_Record {
	var r Node_List_Record

	rt_db.Locks["nodeacs"].RLock()
	r = rt_db.NodeACs[ac]
	rt_db.Locks["nodeacs"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Node_Record(k uint, r *Node_List_Record) {
	var _r Node_List_Record
	var ac string

	rt_db.Locks["nodes"].RLock()
	ac = rt_db.Nodes[k].AC
	rt_db.Locks["nodes"].RUnlock()

	if r == nil {
		if ac != "" {
			rt_db.Locks["nodeacs"].Lock()
			delete(rt_db.NodeACs, ac)
			rt_db.Locks["nodeacs"].Unlock()
		}

		rt_db.Locks["nodes"].Lock()
		delete(rt_db.Nodes, k)
		rt_db.Locks["nodes"].Unlock()
	} else {
		rt_db.Locks["nodes"].Lock()
		_r = *r
		if ac2 := rt_db.Nodes[k].AC2; ac2 != "" {
			_r.AC2 = ac2
		}
		rt_db.Nodes[k] = _r
		rt_db.Locks["nodes"].Unlock()

		ac = _r.AC
		if ac != "" {
			rt_db.Locks["nodeacs"].Lock()
			rt_db.NodeACs[ac] = _r
			rt_db.Locks["nodeacs"].Unlock()
		}
	}
}

func (rt_db *Route_db) Convert_Route_Record(m map[string][]string) {
	var rid uint
	var nid uint
	r := new(Route_List_Record)
	r.Nodes = make(map[uint]PW_List_Record)

	for k, v := range m {
		x := 0
		pw := new(PW_List_Record)

		if v != nil {
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
			}
		} else {
			x, _ = strconv.Atoi(k)
			rid = uint(x)
			rt_db.Update_Route_Record("", rid, nil)
		}
	}
}

func (rt_db *Route_db) Read_Route_Record(k string, rid uint) Route_List_Record {
	var r Route_List_Record

	rt_db.Locks["routes"].RLock()
	if rt_db.Routes[k] != nil {
		r = rt_db.Routes[k][rid]
	}
	rt_db.Locks["routes"].RUnlock()

	return r
}

//Tag: CCC
func (rt_db *Route_db) Read_Cmdb_Record_All_JSON() []byte {

	var cmdb_json []byte
	var err error
	var mnode []string
	var mserver []string
	var mcmdb = make(map[string]map[string]map[string][]string)
	var sstatus string

	rt_db.Locks["nodes"].RLock()
	defer rt_db.Locks["nodes"].RUnlock()

	if mcmdb["Cmdb"] == nil {
		mcmdb["Cmdb"] = make(map[string]map[string][]string)
	}

	for nid, nr := range rt_db.Nodes {
		nid_s := strconv.Itoa(int(nid))
		if mcmdb["Cmdb"][nid_s] == nil {
			mcmdb["Cmdb"][nid_s] = make(map[string][]string)
		}
		mnode = []string{}
		mnode = append(mnode, nr.Name)
		mnode = append(mnode, strconv.Itoa(int(nr.Priority)))
		mnode = append(mnode, strconv.Itoa(int(nr.Weight)))
		mnode = append(mnode, strconv.Itoa(int(nr.Costs)))
		mnode = append(mnode, strconv.Itoa(int(nr.Usage)))
		if nr.Status {
			sstatus = "1"
		} else {
			sstatus = "0"
		}
		mnode = append(mnode, sstatus)
		mnode = append(mnode, strconv.Itoa(int(nr.NodeCapacity)))
		mnode = append(mnode, nr.Type)
		mnode = append(mnode, nr.AC)

		mcmdb["Cmdb"][nid_s]["0"] = mnode

		/* node record sample:
		"69": {"0": ["CN-CMCC-ZJ-HZ-C1", "1", "10", "0", "0", "1", "10000", "A", "CN-CMCC-ZJ-HZ-C1"],
		"252": ["112.17.39.163", "0", "1", "1", "1"],
		"253": ["112.17.39.164", "0", "1", "1", "1"],
		"70": ["112.17.39.162", "0", "1", "1", "1"]}
		*/

		for _, sid := range nr.ServerList {
			sid_s := strconv.Itoa(int(sid))
			sr := rt_db.Read_Server_Record(sid)
			mserver = []string{}
			mserver = append(mserver, sr.ServerIp)
			mserver = append(mserver, strconv.Itoa(int(sr.ServerCapacity)))
			mserver = append(mserver, strconv.Itoa(int(sr.ServerGroup)))
			mserver = append(mserver, strconv.Itoa(int(sr.Weight)))
			if sr.Status {
				sstatus = "1"
			} else {
				sstatus = "0"
			}
			mserver = append(mserver, sstatus)
			mcmdb["Cmdb"][nid_s][sid_s] = mserver
		}

	}

	if cmdb_json, err = json.Marshal(mcmdb); err != nil {
		G.Outlog3(G.LOG_ROUTE, "Error marshaling cmdb data: %s", err)
		return nil
	} else {
		G.OutDebug("Cmdb data: %+v", mcmdb)
	}

	return cmdb_json
}

//Tag: JJJ
func (rt_db *Route_db) Read_Route_Record_All_JSON() []byte {

	var routes_json []byte
	var err error
	var mnodes map[string][]string
	var mroutes = make(map[string]map[string]map[string][]string)

	rt_db.Locks["routes"].RLock()
	defer rt_db.Locks["routes"].RUnlock()

	//Routes:map[string]map[uint]Route_List_Record
	for ac, rrs := range rt_db.Routes {
		for rid, rr := range rrs {
			rid_s := strconv.Itoa(int(rid))
			if mroutes[rid_s] == nil {
				mroutes[rid_s] = make(map[string]map[string][]string)
			}
			if mroutes[rid_s][ac] == nil {
				mroutes[rid_s][ac] = make(map[string][]string)
			}
			mnodes = make(map[string][]string)
			for nid, pw := range rr.Nodes {
				nid_s := strconv.Itoa(int(nid))
				pw0_s := strconv.Itoa(int(pw.PW[0]))
				pw1_s := strconv.Itoa(int(pw.PW[1]))
				mnodes[nid_s] = []string{pw0_s, pw1_s}
			}
			mroutes[rid_s][ac] = mnodes
		}
	}

	if routes_json, err = json.Marshal(mroutes); err != nil {
		G.Outlog3(G.LOG_ROUTE, "Error marshaling routes data: %s", err)
		return nil
	} else {
		G.OutDebug("Routes data: %+v", mroutes)
	}

	return routes_json
}

func (rt_db *Route_db) Update_Route_Record(k string, rid uint, r *Route_List_Record) {
	rt_db.Locks["routes"].Lock()
	if r == nil {
		if rid == 0 && k != "" {
			delete(rt_db.Routes, k)
		} else if k == "" && rid != 0 {
			for ac, _ := range rt_db.Routes {
				delete(rt_db.Routes[ac], rid)
			}
		} else if k != "" && rid != 0 {
			if rt_db.Routes[k] != nil {
				delete(rt_db.Routes[k], rid)
			}
		}
	} else {
		if rt_db.Routes[k] == nil {
			rt_db.Routes[k] = make(map[uint]Route_List_Record)
		}
		if rt_db.Routes[k][rid].Nodes == nil {
			//new one
			rt_db.Routes[k][rid] = *r
		} else {
			//add or update new nodes
			for nid, pw := range r.Nodes {
				rt_db.Routes[k][rid].Nodes[nid] = pw
			}
		}
	}
	rt_db.Locks["routes"].Unlock()
}

func (rt_db *Route_db) Read_Cache_Record(dn string, ac string) RT_Cache_Record {
	var r RT_Cache_Record
	var _ttl uint32

	rt_db.Locks["cache"].RLock()
	if rt_db.Cache[ac] != nil {
		if rt_db.Cache[ac][dn].TTL != 0 {
			if rt_db.Cache[ac][dn].TTL > rt_db.Cache[ac][dn].RR_TTL {
				_ttl = rt_db.Cache[ac][dn].RR_TTL
			} else {
				_ttl = rt_db.Cache[ac][dn].TTL
			}

			if int64(_ttl)+rt_db.Cache[ac][dn].TS >= time.Now().Unix() {
				//cache not expired
				r = rt_db.Cache[ac][dn]
			}
		}
	}
	rt_db.Locks["cache"].RUnlock()

	return r
}

func (rt_db *Route_db) GetRTCache(dn string, ac string) ([]string, uint32, string, uint, string, bool) {
	var r RT_Cache_Record

	r = rt_db.Read_Cache_Record(dn, ac)

	if r.TTL != 0 {
		return r.AAA, r.TTL, r.TYPE, r.RID, r.MAC, true
	} else {
		return nil, 0, "", 0, "", false
	}
}

func (rt_db *Route_db) Update_Cache_Record(dn string, ac string, r *RT_Cache_Record) {
	rt_db.Locks["cache"].Lock()
	if r == nil {
		if _, ok := rt_db.Cache[ac][dn]; ok {
			delete(rt_db.Cache[ac], dn)
			rt_db.CacheSize = rt_db.CacheSize - 1
		}
	} else {
		if rt_db.Cache[ac] == nil {
			rt_db.Cache[ac] = make(map[string]RT_Cache_Record)
		}
		if rt_db.CacheSize >= RT_Cache_Size {
			//cache too large, pop one
			for x, _ := range rt_db.Cache {
				for y, _ := range rt_db.Cache[x] {
					//it's safe to del keys from golang map within a range loop
					delete(rt_db.Cache[x], y)
					rt_db.CacheSize = rt_db.CacheSize - 1
					break
				}
				break
			}
		}
		if _, ok := rt_db.Cache[ac][dn]; !ok {
			rt_db.CacheSize = rt_db.CacheSize + 1
		}
		rt_db.Cache[ac][dn] = *r
	}
	rt_db.Locks["cache"].Unlock()
}

func (rt_db *Route_db) Read_NetPerf_Record(src, dst uint) Net_Perf_Record {
	var r Net_Perf_Record

	rt_db.Locks["nets"].RLock()
	if rt_db.Nets[src] != nil {
		r = rt_db.Nets[src][dst]
	}
	rt_db.Locks["nets"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_NetPerf_Record(src, dst uint, r *Net_Perf_Record) {

	rt_db.Locks["nets"].Lock()
	if r == nil {
		if rt_db.Nets[src] != nil {
			delete(rt_db.Nets[src], dst)
		}
	} else {
		if rt_db.Nets[src] == nil {
			rt_db.Nets[src] = make(map[uint]Net_Perf_Record)
		}
		rt_db.Nets[src][dst] = *r
	}
	rt_db.Locks["nets"].Unlock()
}

func (rt_db *Route_db) Read_NetPerf_Record_All_JSON() []byte {
	var netperf_json []byte
	var err error
	var m = make(map[uint]map[uint]map[string]uint)

	rt_db.Locks["nets"].RLock()
	defer rt_db.Locks["nets"].RUnlock()

	//NetPerf:map[uint]map[uint]Route_List_Record
	for src_id, pps := range rt_db.Nets {
		for dst_id, r := range pps {
			if m[src_id] == nil {
				m[src_id] = make(map[uint]map[string]uint)
			}
			m[src_id][dst_id] = map[string]uint{"rtt": r.RTT, "ds": r.DS}
		}
	}

	if netperf_json, err = json.Marshal(m); err != nil {
		G.Outlog3(G.LOG_ROUTE, "Error marshaling nets data: %s", err)
		return nil
	} else {
		G.OutDebug("Nets data: %+v", m)
	}

	return netperf_json
}

func (rt_db *Route_db) Read_NetPerf_Record_All_JSON2() []byte {
	var netperf_json []byte
	var err error
	var m = make(map[string]map[string]map[string][]string)

	rt_db.Locks["nets"].RLock()
	defer rt_db.Locks["nets"].RUnlock()

	//NetPerf:map[uint]map[uint]Route_List_Record
	for src_id, pps := range rt_db.Nets {
		for dst_id, r := range pps {
			sid := strconv.Itoa(int(src_id))
			did := strconv.Itoa(int(dst_id))
			if m[sid] == nil {
				m[sid] = make(map[string]map[string][]string)
			}
			m[sid][did] = map[string][]string{"rtt": {strconv.Itoa(int(r.RTT))}, "ds": {strconv.Itoa(int(r.DS))}}
		}
	}

	if netperf_json, err = json.Marshal(m); err != nil {
		G.Outlog3(G.LOG_ROUTE, "Error marshaling nets data: %s", err)
		return nil
	} else {
		G.OutDebug("Nets data: %+v", m)
	}

	return netperf_json
}
