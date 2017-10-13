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
			if G.Debug {
				G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Updating domain status: %s", k))
			}
		}

	case AMQP_OBJ_CMDB:

		for k, v := range *msg.Params {
			if strings.Contains(k, ".") {
				//it's a server
				x, _ := strconv.Atoi(k)
				sr := RT.Rtdb.Read_Server_Record(uint(x))
				RT.Rtdb.Convert_Server_Record(map[string][]string{strconv.Itoa(int(sr.ServerId)): strings.Split(v, ",")})
				if G.Debug {
					G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Updating server status: %s", k))
				}
			} else {
				//it's a node
				RT.Rtdb.Convert_Node_Record(map[string][]string{k: strings.Split(v, ",")})
				if G.Debug {
					G.Outlog(G.LOG_DEBUG, fmt.Sprintf("Updating node status: %s", k))
				}

			}
		}

	case AMQP_OBJ_IP:

	case AMQP_OBJ_CONTROL:

	}

	return err
}
