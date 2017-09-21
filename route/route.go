package grid_route

import G "3grid/tools/globals"
import "encoding/json"
import "io/ioutil"
import "log"
import "net"
import "strconv"
import "strings"
import "sync"

var Version string
var Db_file string
var Ver_Major, Ver_Minor, Ver_Patch uint64

var DM_Version string
var DM_Db_file string
var DM_Ver_Major, DM_Ver_Minor, DM_Ver_Patch uint64

type Route_db struct {
	Servers map[string]Server_List_Record         //string for net.IP.String()
	Nodes   map[uint]Node_List_Record             //uint for nodeid
	Domains map[string]Domain_List_Record         //string for domain name
	Routes  map[string]map[uint]Route_List_Record //string for AreaCode, uint for RoutePlan ID
	Locks   map[string]*sync.RWMutex
	Chan    chan map[string]map[string][]string
}

type Server_List_Record struct {
	ServerIp       net.IP
	ServerGroup    string //description of server type, like 'page', 'video'
	NodeId         uint   //same as nodeid in Node_List_Record
	ServerCapacity uint64 //bandwidth
	Usage          uint   //percentage
	Status         int    //1:ok 0:fail -1:unknown
	Weight         uint   //responde to ServerCapacity
}

type Node_List_Record struct {
	NodeId       uint
	NodeCapacity uint64 //bandwidth
	Usage        uint   //percentage
	Status       int    //1:ok 0:fail -1:unknown
	Priority     uint   //network quality
	Weight       uint   //responde to number of servers
	Costs        int    //cost of node(95), negative value representing that it doesn't reach minimum-guarantee
	ServerList   []net.IP
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
	PW []uint //PW[0] for priority, PW[1] for weight
}

func (rt_db *Route_db) RT_db_init() {
	rt_db.Servers = make(map[string]Server_List_Record)
	rt_db.Nodes = make(map[uint]Node_List_Record)
	rt_db.Domains = make(map[string]Domain_List_Record)
	rt_db.Routes = make(map[string]map[uint]Route_List_Record)

	rt_db.Locks = map[string]*sync.RWMutex{"servers": new(sync.RWMutex),
		"nodes": new(sync.RWMutex), "domains": new(sync.RWMutex), "routes": new(sync.RWMutex)}
	rt_db.Chan = make(chan map[string]map[string][]string, 100)

	if G.Debug {
		log.Printf("Initializing route db..")
	}

	rt_db.LoadDomaindb()

	go rt_db.UpdateRoutedb()
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
			log.Printf("error reading  domain db: %s", err)
		}
	}

	err = json.Unmarshal(jf, &domain_records)
	if err != nil {
		if G.Debug {
			log.Printf("error unmarshaling domain db: %s", err)
		}
	} else {
		rt_db.Convert_Domain_Record(&domain_records)
	}

	return err
}

func (rt_db *Route_db) UpdateRoutedb() error {
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
		}

		rt_db.Update_Domain_Record(k, r)

		if G.Debug {
			log.Printf("json domain record: %s:%+v", k, v)
			log.Printf("update domain record: %+v", r)
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
	var r Server_List_Record
	var k string

	rt_db.Update_Server_Record(k, &r)
}

func (rt_db *Route_db) Read_Server_Record(k string) Server_List_Record {
	var r Server_List_Record

	rt_db.Locks["servers"].RLock()
	r = rt_db.Servers[k]
	rt_db.Locks["servers"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Server_Record(k string, r *Server_List_Record) {
	rt_db.Locks["servers"].Lock()
	if r == nil {
		delete(rt_db.Servers, k)
	} else {
		rt_db.Servers[k] = *r
	}
	rt_db.Locks["servers"].Unlock()
}

func (rt_db *Route_db) Convert_Node_Record(m *map[string][]string) {
	var r Node_List_Record
	var k uint

	rt_db.Update_Node_Record(k, &r)
}

func (rt_db *Route_db) Read_Node_Record(k uint) Node_List_Record {
	var r Node_List_Record

	rt_db.Locks["nodes"].RLock()
	r = rt_db.Nodes[k]
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
	var r Route_List_Record
	var k string
	var nid uint

	rt_db.Update_Route_Record(k, nid, &r)
}

func (rt_db *Route_db) Read_Route_Record(k string, nid uint) Route_List_Record {
	var r Route_List_Record

	rt_db.Locks["routes"].RLock()
	r = rt_db.Routes[k][nid]
	rt_db.Locks["routes"].RUnlock()

	return r
}

func (rt_db *Route_db) Update_Route_Record(k string, nid uint, r *Route_List_Record) {
	rt_db.Locks["routes"].Lock()
	if r == nil {
		delete(rt_db.Routes[k], nid)
	} else {
		rt_db.Routes[k][nid] = *r
	}
	rt_db.Locks["routes"].Unlock()
}
