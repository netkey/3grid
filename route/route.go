package grid

type Route_QA struct {
	SrcIP   net.IP
	DstDN   string
	DstIP   net.IP
	DstPath *Route_Path
}

type Route_Path struct {
	Way map[int]int
}
