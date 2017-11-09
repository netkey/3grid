package grid_amqp

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"encoding/json"
	"net"
)

//Get node&server data from gslb to gslb-center
func (c *Cmds) Get(msg *AMQP_Message) error {
	var err error
	var _msg1 = make(map[string]map[string]map[string][]string)
	var _param = make(map[string]string)

	switch msg.Object {
	case AMQP_OBJ_DOMAIN:
	case AMQP_OBJ_CMDB:
		cmdb_json := RT.Rtdb.Read_Cmdb_Record_All_JSON()

		if err = json.Unmarshal(cmdb_json, &_msg1); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error unmarshal cmdb data: %s", err)
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_CMDB,
			&_msg1, "", *gslb_center, msg.ID); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send cmdb data: %s", err)
		}

	case AMQP_OBJ_ROUTE:
		routes_json := RT.Rtdb.Read_Route_Record_All_JSON()

		if err = json.Unmarshal(routes_json, &_msg1); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error unmarshal routes data: %s", err)
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_ROUTE,
			&_msg1, "", *gslb_center, msg.ID); err != nil {
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
			if aaa, _, _, ok, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, 0); ok {
				_msg1["Dns"][p["Domain"]][p["Ip"]] = aaa
			}
		case "Dns_debug":
			dn := p["Domain"]
			ip := net.ParseIP(p["Ip"])
			ac := IP.Ipdb.GetAreaCode(ip)
			if aaa, _, _, ok, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, 0); ok {
				_msg1["Dns"][p["Domain"]][p["Ip"]] = aaa
				/*
				   DN:image227-c.poco.cn.mmycdn.com matched for image227-c.poco.cn.mmycdn.com, {Name:image227-c.poco.cn Type: Value:image227-c.poco.cn.mmycdn.com Priority:10 ServerGroup:1 Records:3 TTL:300 RoutePlan:[8 70] Status:1 Forbidden:map[]}
				   AC:* matched in route plan 8, {Nodes:map[76:{PW:[1 100]} 71:{PW:[1 100]} 72:{PW:[1 100]}]}
				   Looking at node:CN-CT-ZJ-JH-C1(76), p:1 w:100 u:33 c:0 s:true ac:CN-CT-ZJ-JH-C1
				    CN-CT-ZJ-JH-C1(76) has made default
				   Looking at node:CN-CHU-SD-YT-C1(71), p:1 w:100 u:54 c:0 s:true ac:CUC.CN.HAB.SD.YT
				   CN-CHU-SD-YT-C1(71)() is nearby client(*.CN.HAD.SH), use it
				   Looking at node:CN-CT-GD-FS-C1(72), p:1 w:100 u:34 c:0 s:true ac:CTC.CN.HAN.GD.FS
				   CN-CT-GD-FS-C1(72) is more idle(66.000000>46.000000), use it
				   Chosen node:CN-CT-GD-FS-C1(72) p:1 w:100 u:34 c:0 s:true, second node:CN-CHU-SD-YT-C1(71) u:54 c:0 s:true, for ac:*.CN.HAD.SH
				*/
				//_msg := fmt.Sprintf("查询域名%s匹配%s记录，TTL设置：%d，返回A地址数：%d\n路由方案：%+v，禁止解析区域：%s\nIP区域码（AC）：%s，实际匹配区域码：%s\n，可用节点：%+v，选择节点：%s\n节点信息：%+v")

				_msg1["Dns_debug"][p["Domain"]][p["Ip"]] = []string{""}
			}
		case "Cover":
		case "Source":
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_API,
			&_msg1, "", *gslb_center, msg.ID); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send API data: %s", err)
		}
	}

	return err
}
