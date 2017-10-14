package grid_amqp

import (
	RT "3grid/route"
	G "3grid/tools/globals"
	"fmt"
	"strconv"
	"strings"
)

//update node&server states via gslb-center
func (c *Cmds) State(msg *AMQP_Message) error {
	var err error

	switch msg.Object {
	case AMQP_OBJ_DOMAIN:

		for k, v := range *msg.Params {
			RT.Rtdb.Convert_Domain_Record(map[string][]string{k: strings.Split(v, ",")})
		}

		if G.Debug {
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Updating domain status: %+v", *msg.Params))
		}

	case AMQP_OBJ_CMDB:

		if G.Debug {
			G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Updating node/server: %+v", *msg.Params))
		}

		for k, v := range *msg.Params {
			if strings.Contains(k, ".") {
				//it's a server
				sid := RT.Rtdb.Ips[k].ServerId
				nid := RT.Rtdb.Ips[k].NodeId
				r := append(strings.Split(v, ","), strconv.Itoa(int(nid)))
				m := map[string][]string{strconv.Itoa(int(sid)): r}
				if G.Debug {
					//G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Server id: %d, %+v", sid, m))
				}
				RT.Rtdb.Convert_Server_Record(m)
			} else {
				//it's a node
				x, _ := strconv.Atoi(k)
				nid := uint(x)
				server_list := ""
				for _, sid := range RT.Rtdb.Nodes[nid].ServerList {
					server_list = server_list + strconv.Itoa(int(sid)) + ","
				}
				r := append(strings.Split(v, ","), server_list)
				if G.Debug {
					//G.Outlog(G.LOG_DEBUG, fmt.Sprintf("node id: %s, %+v", k, RT.Rtdb.Nodes[nid].ServerList))
				}
				RT.Rtdb.Convert_Node_Record(map[string][]string{k: r})
			}
		}

	case AMQP_OBJ_IP:

	case AMQP_OBJ_CONTROL:

	}

	return err
}
