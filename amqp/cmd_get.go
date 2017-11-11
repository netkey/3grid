package grid_amqp

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"encoding/json"
	"net"
	"strconv"
	"time"
)

//Get node&server data from gslb to gslb-center
func (c *Cmds) Get(msg *AMQP_Message) error {
	var err error
	var _msg1 = make(map[string]map[string]map[string][]string)
	var _param = make(map[string]string)
	var logs string

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic amqp cmd_get: %s", pan)
		}
	}()

	switch msg.Object {
	case AMQP_OBJ_DOMAIN:
	case AMQP_OBJ_CMDB:
		cmdb_json := RT.Rtdb.Read_Cmdb_Record_All_JSON()

		if err = json.Unmarshal(cmdb_json, &_msg1); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error unmarshal cmdb data: %s", err)
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_CMDB,
			&_msg1, "", msg.Sender, msg.ID); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send cmdb data: %s", err)
		}

	case AMQP_OBJ_ROUTE:
		routes_json := RT.Rtdb.Read_Route_Record_All_JSON()

		if err = json.Unmarshal(routes_json, &_msg1); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error unmarshal routes data: %s", err)
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_ROUTE,
			&_msg1, "", msg.Sender, msg.ID); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send routes data: %s", err)
		}

	case AMQP_OBJ_API:
		p := *msg.Params
		api_type := p["Type"]
		switch api_type {
		case "Dns":
			dn := p["Domain"]
			ip := net.ParseIP(p["Ip"])
			ac := IP.Ipdb.GetAreaCode(ip)
			aaa, ttl, _type, _, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, 0)
			if _type == "" {
				_type = "A"
			}
			_dns_info := []string{_type, strconv.Itoa(int(ttl))}
			_dns_info = append(_dns_info, aaa...)
			_msg1 = map[string]map[string]map[string][]string{"Dns": {p["Domain"]: {p["Ip"]: _dns_info}}}
		case "Dns_debug":
			dn := p["Domain"]
			ip := net.ParseIP(p["Ip"])
			ac := IP.Ipdb.GetAreaCode(ip)

			_debug_info := []string{}

			_my_goid := G.GoID()

			G.Apilog.Lock.Lock()
			_chan := make(chan string, 100)
			G.Apilog.Chan = &_chan
			G.Apilog.Goid = _my_goid

			defer func() {
				close(_chan)
				G.Apilog.Goid = 0
				G.Apilog.Chan = nil
				G.Apilog.Lock.Unlock()
			}()

			aaa, ttl, _type, _, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, 0)
			if _type == "" {
				_type = "A"
			}

			_time_out := false
			for {
				select {
				case logs = <-*G.Apilog.Chan:
					_debug_info = append(_debug_info, logs)
				case <-time.After(500 * time.Millisecond):
					_time_out = true
				}
				if _time_out {
					break
				}
			}

			_dns_info := []string{_type, strconv.Itoa(int(ttl))}
			_dns_info = append(_dns_info, aaa...)
			_msg1 = map[string]map[string]map[string][]string{"Dns": {p["Domain"]: {p["Ip"]: _dns_info}}}

			_msg1["Dns_debug"] = map[string]map[string][]string{p["Domain"]: {p["Ip"]: _debug_info}}

			//_msg1["Dns_debug"]["Domain"]["Ip"] = _debug_info //bug here
			//_msg := fmt.Sprintf("查询域名%s匹配%s记录，TTL设置：%d，返回A地址数：%d\n路由方案：%+v，禁止解析区域：%s\nIP区域码（AC）：%s，实际匹配区域码：%s\n，可用节点：%+v，选择节点：%s\n节点信息：%+v")

		case "Cover":
		case "Source":
		}

		if msg.ID == 0 {
			break
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_API,
			&_msg1, "", msg.Sender, msg.ID); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send API data: %s", err)
		} else {
			if G.Debug {
				G.Outlog3(G.LOG_API, "Sent API data: %+v", _msg1)
			}
		}
	}

	return err
}
