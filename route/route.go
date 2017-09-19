package grid_route

import "net"

var Version string
var Db_file string
var Ver_Major, Ver_Minor, Ver_Patch uint64

type Route_db struct {
	Servers map[string]Server_List_Record         //string for net.IP.String()
	Nodes   map[uint]Node_List_Record             //uint for nodeid
	Domains map[string]Domain_List_Record         //string for domain name
	Routes  map[string]map[uint]Route_List_Record //string for AreaCode, uint for RoutePlan ID
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
