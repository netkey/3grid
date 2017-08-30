package grid_route

var Version string
var Db_file string
var Ver_Major, Ver_Minor, Ver_Patch uint64

type Route_db struct {
	CacheServers map[int]Server_List_Record
	NodeServers  map[int]Server_List_Record
	Nodes        map[int]Node_List_Record
	Domains      map[string]Domain_List_Record
	Routes       map[string]map[string]Route_List_Record
}

type Server_List_Record struct {
	ServerIp    int
	ServerGroup string
	NodeId      int
	Usage       int
	Status      int
	Weight      int
}

type Node_List_Record struct {
	NodeId       int
	NodeCapacity int
	Usage        int
	Status       int
	Priority     int
}

type Domain_List_Record struct {
	DomainId    int
	DomainName  string
	ServerGroup string
}

type Route_List_Record struct {
	DomainName   string
	ClientRegion string
	NodeIds      string
	Ext          string
}
