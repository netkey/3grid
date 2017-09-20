package grid_route

import G "3grid/tools/globals"
import "log"
import "net"
import "sync"

var Version string
var Db_file string
var Ver_Major, Ver_Minor, Ver_Patch uint64

type Route_db struct {
	Servers map[string]Server_List_Record         //string for net.IP.String()
	Nodes   map[uint]Node_List_Record             //uint for nodeid
	Domains map[string]Domain_List_Record         //string for domain name
	Routes  map[string]map[uint]Route_List_Record //string for AreaCode, uint for RoutePlan ID
	Locks   map[string]*sync.RWMutex
	Chan    chan map[string][]string
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

	rt_db.Locks = map[string]*sync.RWMutex{"servers": new(sync.RWMutex), "nodes": new(sync.RWMutex), "domains": new(sync.RWMutex), "routes": new(sync.RWMutex)}
	rt_db.Chan = make(chan map[string][]string, 100)

	if G.Debug {
		log.Printf("Initializing route db..")
	}

	go rt_db.UpdateRoutedb()
}

func (rt_db *Route_db) UpdateRoutedb() error {
	for {
		m := <-rt_db.Chan
		for t, a := range m {
			rt_db.Locks[t].Lock()
			switch t {
			case "domains":
				rt_db.Conver_Domain_Record(a)
			case "servers":
				rt_db.Conver_Server_Record(a)
			case "nodes":
				rt_db.Conver_Node_Record(a)
			case "routes":
				rt_db.Conver_Route_Record(a)
			default:
			}
			rt_db.Locks[t].Unlock()
		}
	}

	return nil
}

func (rt_db *Route_db) Conver_Domain_Record(a []string) {
	var r Domain_List_Record
	var k string

	rt_db.Domains[k] = r
}

func (rt_db *Route_db) Conver_Server_Record(a []string) {
	var r Server_List_Record
	var k string

	rt_db.Servers[k] = r
}

func (rt_db *Route_db) Conver_Node_Record(a []string) {
	var r Node_List_Record
	var k uint

	rt_db.Nodes[k] = r
}

func (rt_db *Route_db) Conver_Route_Record(a []string) {
	var r Route_List_Record
	var k string
	var nid uint

	rt_db.Routes[k][nid] = r
}
