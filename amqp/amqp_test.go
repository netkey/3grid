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
	RT.RT_Cache_Size = 1000

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

func TestUpdateRtdb(t *testing.T) {
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
