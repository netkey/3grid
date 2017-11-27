package grid_amqp

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"encoding/json"
	"net"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//Get node&server data from gslb to gslb-center
func (c *Cmds) Get(msg *AMQP_Message) error {
	var err error
	var _msg1 = make(map[string]map[string]map[string][]string)
	var _param = make(map[string]string)
	var logs string
	var _chan chan string

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
			&_msg1, "", msg.Sender, msg.ID, false); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send cmdb data: %s", err)
		}

	case AMQP_OBJ_ROUTE:
		routes_json := RT.Rtdb.Read_Route_Record_All_JSON()

		if err = json.Unmarshal(routes_json, &_msg1); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error unmarshal routes data: %s", err)
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_ROUTE,
			&_msg1, "", msg.Sender, msg.ID, false); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send routes data: %s", err)
		}

	case AMQP_OBJ_API:
		p := *msg.Params
		api_type := p["Type"]
		switch api_type {
		case "Dns":
			dn := p["Domain"]
			ip := net.ParseIP(p["Ip"])
			ac := IP.Ipdb.GetAreaCode(ip, p["Ip"])
			aaa, ttl, _type, _, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, p["Ip"], 0)
			if _type == "" {
				_type = "A"
			}
			_dns_info := []string{_type, strconv.Itoa(int(ttl))}
			_dns_info = append(_dns_info, aaa...)
			_msg1 = map[string]map[string]map[string][]string{"Dns": {p["Domain"]: {p["Ip"]: _dns_info}}}
		case "Dns_debug":
			_my_goid := runtime.Goid()

			//only one Dns_debug request can be run at a same time
			G.Apilog_Lock.Lock()

			_chan = make(chan string, 100)
			defer func() {
				G.ApilogS_Lock.Lock()
				G.ApilogS = false
				G.ApilogS_Lock.Unlock()

				G.Apilog.Clock.Lock()
				close(_chan)
				G.Apilog.Goid = 0
				G.Apilog.Chan = nil
				G.Apilog.Clock.Unlock()

				G.Apilog_Lock.Unlock()
			}()

			G.Apilog.Clock.Lock()
			G.Apilog.Chan = &_chan
			G.Apilog.Goid = _my_goid
			G.Apilog.Clock.Unlock()

			G.ApilogS_Lock.Lock()
			G.ApilogS = true
			G.ApilogS_Lock.Unlock()

			dn := p["Domain"]
			ip := net.ParseIP(p["Ip"])
			ac := IP.Ipdb.GetAreaCode(ip, p["Ip"])
			aaa, ttl, _type, _, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, p["Ip"], 0)
			if _type == "" {
				_type = "A"
			}

			_debug_info := []string{}
			_time_out := false
			for {
				select {
				case logs = <-*G.Apilog.Chan:
					//get my debug logs
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

		case "Cover":
			var rr RT.Route_List_Record
			var rid uint
			var ac, a, b, c string

			n := make(map[string]map[string]map[string]map[uint]int)
			r := make(map[string]map[string]map[string][]string)

			dn := p["Domain"]
			dr := RT.Rtdb.Read_Domain_Record(dn)

			if dr.RoutePlan == nil {
				break
			} else {
				//G.Outlog3(G.LOG_API, "Cover Route Plan: %+v", dr.RoutePlan)
			}

			RT.Rtdb.Locks["routes"].RLock()
			ac_keys := reflect.ValueOf(RT.Rtdb.Routes).MapKeys()
			RT.Rtdb.Locks["routes"].RUnlock()

			//walk through routes with acs
			for _, ac_key := range ac_keys {
				ac = ac_key.String()
				find := false
				for _, _rid := range dr.RoutePlan {
					rid = _rid
					rr = RT.Rtdb.Read_Route_Record(ac, rid)
					if rr.Nodes != nil && len(rr.Nodes) > 0 {
						find = true
						break
					}
				}

				if !find {
					continue
				}

				a = ""
				b = ""
				c = ""

				//ac:CTC.CN.HAN.GD  acs:0.1.2.3
				//ac:CTC.CN.HAN
				//ac:CTC.CN
				acs := strings.Split(ac, ".")
				la := len(acs)
				a = acs[0]

				switch la {
				case 1, 2:
					b = "*"
					c = "*"
				case 3:
					b = acs[2]
					c = "*"
				case 4:
					b = acs[2]
					c = acs[3]
				}

				if n[a] == nil {
					n[a] = make(map[string]map[string]map[uint]int)
					r[a] = make(map[string]map[string][]string)
				}
				if n[a][b] == nil {
					n[a][b] = make(map[string]map[uint]int)
					r[a][b] = make(map[string][]string)
				}
				if n[a][b][c] == nil {
					n[a][b][c] = make(map[uint]int)
					r[a][b][c] = []string{}
				}

				for nid, _ := range rr.Nodes {
					n[a][b][c][nid] = 1
				}

				//G.Outlog3(G.LOG_API, "Cover 4: ac:%s rid:%d a:%s b:%s c:%s n:%+v",
				//ac, rid, a, b, c, n[a][b][c])
			}

			//G.Outlog3(G.LOG_API, "Cover N: CMCC.HAN %+v", n["CMCC"]["HAN"])

			for a, va := range n {
				for b, vb := range va {
					for c, vc := range vb {
						msr := make(map[uint]map[string]int)
						for nid, _ := range vc {
							nr := RT.Rtdb.Read_Node_Record(nid)
							for _, sid := range nr.ServerList {
								sr := RT.Rtdb.Read_Server_Record(sid)
								if sr.ServerId != 0 {
									//get rid of duplicate servers
									if msr[nid] == nil {
										msr[nid] = make(map[string]int)
									}
									msr[nid][sr.ServerIp] = 1
								}
							}
						}

						for _, sss := range msr {
							for sip, _ := range sss {
								//append server list
								r[a][b][c] = append(r[a][b][c], sip)
							}
						}
					}
				}
			}

			r["Z"] = map[string]map[string][]string{"QueryDomain": {"*": {dn}}}

			_msg1 = r

		case "Source":
		}

		if msg.ID == 0 {
			break
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_API,
			&_msg1, "", msg.Sender, msg.ID, true); err != nil {
			if G.Debug {
				G.Outlog3(G.LOG_API, "Error send API data(id:%d): %s", msg.ID, err)
			}
		} else {
			if G.Debug {
				G.Outlog3(G.LOG_API, "Sent API Cover data: to:%s id:%d", msg.Sender, msg.ID)
			}
		}
	}

	return err
}
