package grid_amqp

import (
	RT "3grid/route"
	G "3grid/tools/globals"
	"encoding/json"
)

//Get node&server data from gslb to gslb-center
func (c *Cmds) Get(msg *AMQP_Message) error {
	var err error
	var _msg1 = make(map[string]map[string]map[string][]string)
	var _param = make(map[string]string)

	switch msg.Object {
	case AMQP_OBJ_DOMAIN:
	case AMQP_OBJ_CMDB:
	case AMQP_OBJ_ROUTE:
		routes_json := RT.Rtdb.Read_Route_Record_All_JSON()

		if err = json.Unmarshal(routes_json, &_msg1); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error unmarshal routes data: %s", err)
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_ROUTE,
			&_msg1, "", *gslb_center, msg.ID); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send routes data: %s", err)
		}

	}

	return err
}
