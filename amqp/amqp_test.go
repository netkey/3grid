package grid_amqp

import (
	IP "3grid/ip"
	RT "3grid/route"
	"testing"
	"time"
)

func init_db() {

	IP.Db_file = "../db/ip.db"
	ipdb := IP.IP_db{}

	IP.Ipdb = &ipdb
	IP.Ipdb.IP_db_init()

	RT.RT_Db_file = "../db/route.db"
	RT.CM_Db_file = "../db/cmdb.db"
	RT.DM_Db_file = "../db/domain.db"

	RT.Rtdb = &RT.Route_db{}
	RT.Rtdb.RT_db_init()

	RT.MyACPrefix = "MMY"
	RT.Service_Cutoff_Percent = 85
	RT.Service_Deny_Percent = 95
	RT.RT_Cache_TTL = 10
	RT.RR_Cache_TTL = 10
	RT.RT_Cache_Size = 1000

}

func TestStateCmdbNode(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_msg1 = map[string]map[string]map[string][]string{"": {"": {"": {}}}}
	_msg2 = ""

	//"122": ["CN-CRC-GD-GZ-C1", " 1", "10", "0", "0", "1", "1700", "A", "CN-CRC-GD-GZ-C1"]
	_param = map[string]string{"122": "CN-CRC-GD-GZ-C1,1,10,0,50,0,1234,A,CN-CRC-GD-GZ-C1"}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "State",
		Params:  &_param,
		Object:  "Cmdb",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	State_Recv = true

	c.State(&am)

	time.Sleep(time.Duration(1) * time.Second)
	nr := RT.Rtdb.Read_Node_Record(122)

	if nr.Status == false && nr.NodeCapacity == 1234 && nr.Usage == 50 &&
		(nr.ServerList != nil && len(nr.ServerList) > 0) {
		t.Logf("Node state updated: %+v", nr)
	} else {
		t.Errorf("Error updating node state: %+v", nr)
	}

}

func TestStateCmdbServer(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_msg1 = map[string]map[string]map[string][]string{"": {"": {"": {}}}}
	_msg2 = ""

	//"267" "122.72.12.19", "0", "1", "1", "1"  id ip usage group weight status
	_param = map[string]string{"122.72.12.19": "122.72.12.19,30,1,1,0,122"}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "State",
		Params:  &_param,
		Object:  "Cmdb",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	State_Recv = true

	c.State(&am)

	time.Sleep(time.Duration(1) * time.Second)
	sr := RT.Rtdb.Read_Server_Record(267)

	if sr.Status == false && sr.Usage == 30 {
		t.Logf("Server state updated: %+v", sr)
	} else {
		t.Errorf("Error updating server state: %+v", sr)
	}

}

func TestStateDomain(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_msg1 = map[string]map[string]map[string][]string{"": {"": {"": {}}}}
	_msg2 = ""

	//"image15-c.poco.cn": ["", "image15-c.poco.cn.mmycdn.com", "10", "1", "3", "300", "8,70", "1", ""]
	_param = map[string]string{"image15-c.poco.cn": ",image15-c.poco.cn.mmycdn.com,10,1,3,600,8:70,0,CTC:CUC"}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "State",
		Params:  &_param,
		Object:  "Domain",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	State_Recv = true
	c.State(&am)

	time.Sleep(time.Duration(1) * time.Second)
	dr := RT.Rtdb.Read_Domain_Record("image15-c.poco.cn.mmycdn.com")

	if dr.TTL == 600 && dr.Status == 0 {
		t.Logf("Domain updated: %+v", dr)
	} else {
		t.Errorf("Error updating domain: %+v", dr)
	}

}

func TestUpdateCmdb(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_param = map[string]string{}
	_msg2 = ""

	//"122": {"0": ["CN-CRC-GD-GZ-C1", " 1", "10", "0", "0", "1", "1700", "A", "CN-CRC-GD-GZ-C1"]}
	_msg1 = map[string]map[string]map[string][]string{"Cmdb": {"122": {"0": {"CN-CRC-GD-GZ-C1", " 1", "10", "0", "50", "0", "1234", "A", "CN-CRC-GD-GZ-C1"}, "267": {"122.72.12.19", "0", "1", "1", "1"}, "92": {"122.72.12.18", "0", "1", "1", "1"}}}}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "Update",
		Params:  &_param,
		Object:  "Cmdb",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Update(&am)

	time.Sleep(time.Duration(1) * time.Second)
	nr := RT.Rtdb.Read_Node_Record(122)

	if nr.Status == false && nr.NodeCapacity == 1234 && nr.Usage == 50 && (nr.ServerList != nil && len(nr.ServerList) > 0) {
		if sr := RT.Rtdb.Read_Server_Record(267); sr.ServerIp == "122.72.12.19" {
			if sr := RT.Rtdb.Read_Server_Record(92); sr.ServerIp == "122.72.12.18" {
				t.Logf("Node updated: %+v", nr)
			}
		}
	} else {
		t.Errorf("Error updating node: %+v", nr)
	}

}

func TestUpdateRoutedb(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_param = map[string]string{}
	_msg2 = ""

	//{"1": {"*.*": {"65": ["2", "100"]}, "CMCC.CN.XIN.SC": {"81": ["1", "100"]}}}
	_msg1 = map[string]map[string]map[string][]string{"1": {"*.*": {"65": {"5", "123"}}, "CMCC.CN.XIN.SC": {"81": {"10", "1000"}}}}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "Update",
		Params:  &_param,
		Object:  "Routedb",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Update(&am)

	time.Sleep(time.Duration(1) * time.Second)

	if rr := RT.Rtdb.Read_Route_Record("*.*", 1); rr.Nodes != nil && rr.Nodes[65].PW[0] == 5 && rr.Nodes[65].PW[1] == 123 {
		if rr2 := RT.Rtdb.Read_Route_Record("CMCC.CN.XIN.SC", 1); rr2.Nodes != nil && rr2.Nodes[81].PW[0] == 10 && rr2.Nodes[81].PW[1] == 1000 {
			t.Logf("Route updated: %+v %+v", rr, rr2)
		}
	} else {
		t.Errorf("Error updating route: %+v", _msg1)
	}

}

func TestDeleteRoutedb(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_param = map[string]string{}
	_msg2 = ""

	//{"1": {"*.*": {"65": ["2", "100"]}, "CMCC.CN.XIN.SC": {"81": ["1", "100"]}}}
	_msg1 = map[string]map[string]map[string][]string{"1": {"*.*": {"65": {"5", "123"}}, "CMCC.CN.XIN.SC": {"81": {"10", "1000"}}}}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "Delete",
		Params:  &_param,
		Object:  "Routedb",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Delete(&am)

	time.Sleep(time.Duration(1) * time.Second)

	if rr := RT.Rtdb.Read_Route_Record("*.*", 1); rr.Nodes == nil {
		t.Logf("Route deleted: %+v", rr)
	} else {
		t.Errorf("Error delete route: %+v", _msg1)
	}

}

func TestDeleteCmdb(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_param = map[string]string{}
	_msg2 = ""

	//"122": {"0": ["CN-CRC-GD-GZ-C1", " 1", "10", "0", "0", "1", "1700", "A", "CN-CRC-GD-GZ-C1"]}
	_msg1 = map[string]map[string]map[string][]string{"Cmdb": {"122": {"0": {"CN-CRC-GD-GZ-C1", " 1", "10", "0", "50", "0", "1234", "A", "CN-CRC-GD-GZ-C1"}, "267": {"122.72.12.19", "0", "1", "1", "1"}, "92": {"122.72.12.18", "0", "1", "1", "1"}}}}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "Delete",
		Params:  &_param,
		Object:  "Cmdb",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Delete(&am)

	time.Sleep(time.Duration(1) * time.Second)
	nr := RT.Rtdb.Read_Node_Record(122)

	if nr.NodeId == 0 {
		t.Logf("Node deleted: %d", 122)
	} else {
		t.Errorf("Error deleting node: %d %+v", 122, nr)
	}

	sr := RT.Rtdb.Read_Server_Record(267)
	if sr.ServerId == 0 {
		t.Logf("Server deleted: %d", 267)
	} else {
		t.Errorf("Error deleting server: %d %+v", 267, sr)
	}

	sr2 := RT.Rtdb.Read_IP_Record("122.72.12.18")
	if sr2.ServerId == 0 {
		t.Logf("Server deleted: %d", 92)
	} else {
		t.Errorf("Error deleting server: %d %+v", 92, sr2)
	}

}

func TestUpdateDomain(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_param = map[string]string{}
	_msg2 = ""

	//"image15-c.poco.cn": ["", "image15-c.poco.cn.mmycdn.com", "10", "1", "3", "300", "8,70", "1", ""]
	_msg1 = map[string]map[string]map[string][]string{"Domain": {"Domain": {"image15-c.poco.cn": {"", "image15-c.poco.cn.mmycdn.com", "10", "1", "3", "600", "8,70", "1", ""}}}}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "Update",
		Params:  &_param,
		Object:  "Domain",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Update(&am)

	time.Sleep(time.Duration(1) * time.Second)
	dr := RT.Rtdb.Read_Domain_Record("image15-c.poco.cn.mmycdn.com")

	if dr.TTL == 600 {
		t.Logf("Domain updated: %+v", dr)
	} else {
		t.Errorf("Error updating domain: %+v", dr)
	}

}

func TestDeleteDomain(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_param = map[string]string{}
	_msg2 = ""

	//"image15-c.poco.cn": ["", "image15-c.poco.cn.mmycdn.com", "10", "1", "3", "300", "8,70", "1", ""]
	_msg1 = map[string]map[string]map[string][]string{"Domain": {"Domain": {"image15-c.poco.cn.mmycdn.com": {"", "image15-c.poco.cn.mmycdn.com", "10", "1", "3", "600", "8,70", "1", ""}}}}

	am := AMQP_Message{
		ID:      1,
		Sender:  "gslb-center",
		Command: "Delete",
		Params:  &_param,
		Object:  "Domain",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Delete(&am)

	time.Sleep(time.Duration(1) * time.Second)
	dr := RT.Rtdb.Read_Domain_Record("image15-c.poco.cn.mmycdn.com")

	if dr.TTL == 0 {
		t.Logf("Domain deleted: image15-c.poco.cn.mmycdn.com")
	} else {
		t.Errorf("Error deleting domain: %+v", dr)
	}

}

func TestCmdGet(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_msg2 = ""
	_msg1 = map[string]map[string]map[string][]string{"Api": {"Api": {"Api": {""}}}}
	_param = map[string]string{"Domain": "image15-c.poco.cn.mmycdn.com", "Ip": "61.144.1.1"}

	am := AMQP_Message{
		ID:      0,
		Sender:  "gslb-api",
		Command: "Get",
		Params:  &_param,
		Object:  "Api",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	if err := c.Get(&am); err == nil {
		t.Logf("AMQP API test ok")
	} else {
		t.Errorf("AMQP API test failed")
	}

}

func TCmdGet(t *testing.T) {
	var _param map[string]string
	var _msg1 map[string]map[string]map[string][]string
	var _msg2 string
	var c = new(Cmds)

	if IP.Ipdb == nil {
		init_db()
	}

	_msg2 = ""
	_msg1 = map[string]map[string]map[string][]string{"Api": {"Api": {"Api": {""}}}}
	_param = map[string]string{"Domain": "image15-c.poco.cn.mmycdn.com", "Ip": "61.144.1.1"}

	am := AMQP_Message{
		ID:      0,
		Sender:  "gslb-api",
		Command: "Get",
		Params:  &_param,
		Object:  "Api",
		Msg1:    &_msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	c.Get(&am)

}

func TestBenchAPI(t *testing.T) {
	res := testing.Benchmark(BenchmarkAPI)
	t.Logf("AMQP API gen_time: %f s/q", float64(res.T)/(float64(res.N)*1000000000))
	t.Logf("AMQP API gen_rate: %d q/s", uint64(1.0/(float64(res.T)/(float64(res.N)*1000000000))))
}

func BenchmarkAPI(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		t := &testing.T{}
		for pb.Next() {
			TCmdGet(t)
		}
	})
}
