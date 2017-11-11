package grid_amqp

import (
	RT "3grid/route"
	G "3grid/tools/globals"
	"strconv"
	"strings"
)

//update node&server states via gslb-center
func (c *Cmds) State(msg *AMQP_Message) error {
	var err error

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic amqp cmd_state: %s", pan)
		}
	}()

	if !State_Recv {
		return nil
	}

	switch msg.Object {
	case AMQP_OBJ_DOMAIN:

		for k, v := range *msg.Params {
			ds := strings.Split(v, ",")
			if len(ds) > 8 {
				if strings.Contains(ds[6], ":") {
					ds[6] = strings.Replace(ds[6], ":", ",", -1)
				}
				if strings.Contains(ds[8], ":") {
					ds[8] = strings.Replace(ds[8], ":", ",", -1)
				}
				RT.Rtdb.Convert_Domain_Record(map[string][]string{k: ds})
				G.OutDebug("Domain state: %s %+v", k, ds)
			}
		}

	case AMQP_OBJ_CMDB:

		for k, v := range *msg.Params {
			if strings.Contains(k, ".") {
				//it's a server
				ir := RT.Rtdb.Read_IP_Record(k)
				sid := ir.ServerId
				nid := ir.NodeId

				if nid > 0 {
					r := append(strings.Split(v, ","), strconv.Itoa(int(nid)))
					m := map[string][]string{strconv.Itoa(int(sid)): r}
					RT.Rtdb.Convert_Server_Record(m)
					G.OutDebug("Server state: %+v", m)
				}
			} else {
				//it's a node
				x, _ := strconv.Atoi(k)
				nid := uint(x)
				server_list := ""
				nr := RT.Rtdb.Read_Node_Record(nid)

				if nr.NodeId > 0 {
					for _, sid := range nr.ServerList {
						server_list = server_list + strconv.Itoa(int(sid)) + ","
					}
					r := append(strings.Split(v, ","), server_list)
					RT.Rtdb.Convert_Node_Record(map[string][]string{k: r})
					G.OutDebug("Node state: %s %+v", k, r)
				}
			}
		}

	case AMQP_OBJ_IP:

	case AMQP_OBJ_CONTROL:

	}

	return err
}
