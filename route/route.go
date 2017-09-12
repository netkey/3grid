package grid_route

import "net"

var Version string
var Db_file string
var Ver_Major, Ver_Minor, Ver_Patch uint64

type Route_db struct {
	Servers map[net.IP]Server_List_Record
	Nodes   map[uint]Node_List_Record
	Domains map[string]Domain_List_Record
	Routes  map[string]map[uint]Route_List_Record //map[string]map[uint]  string for AreaCode, uint for RoutePlan ID
}

type Server_List_Record struct {
	ServerIp       net.IP
	ServerGroup    string //description of server type, like 'page', 'video'
	NodeId         uint
	ServerCapacity uint64 //bandwidth
	Usage          uint   //percentage
	Status         int    //1:ok 0:fail
	Weight         uint   //responde to ServerCapacity
}

type Node_List_Record struct {
	NodeId       uint
	NodeCapacity uint64 //bandwidth
	Usage        uint   //percentage
	Status       int    //1:ok 0:fail
	Priority     uint   //network quality
	Weight       uint   //responde to number of servers
	Costs        int    //cost of node(95)
	ServerList   []net.IP
}

type Domain_List_Record struct {
	Name        string
	Type        string
	Value       string
	Priority    string
	ServerGroup string
	Records     int
	TTL         int
	RoutePlan   []int
}

type Route_List_Record struct {
	Nodes []uint
}
