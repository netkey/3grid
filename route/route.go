package grid_route

type Route_DB struct {
	CacheServers map[int]Server_List_Record
	NodeServers  map[int]Server_List_Record
	Nodes        map[int]Node_List_Record
	Domains      map[string]Domain_List_Record
	Routes       map[string][string]Route_List_Record
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
