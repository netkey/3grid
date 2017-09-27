package grid_route

func (rt_db *Route_db) ChoseNode(nodes map[uint]PW_List_Record) Node_List_Record {
	return Node_List_Record{}
}

func (rt_db *Route_db) ChoseServer(servers []uint) []uint {
	server_list := []uint{}

	for _, sid := range servers {
		if rt_db.Servers[sid].Status == false {
			continue
		}
		server_list = append(server_list, sid)
	}

	return nil
}
