package grid_amqp

import (
	IP "3grid/ip"
	RT "3grid/route"
	G "3grid/tools/globals"
	"encoding/json"
	"net"
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
			ac := IP.Ipdb.GetAreaCode(ip)
			aaa, ttl, _type, _, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, 0)
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

			dn := p["Domain"]
			ip := net.ParseIP(p["Ip"])
			ac := IP.Ipdb.GetAreaCode(ip)
			aaa, ttl, _type, _, _, _, _ := RT.Rtdb.GetAAA(dn, ac, ip, 0)
			if _type == "" {
				_type = "A"
			}

			_debug_info := []string{}
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

		case "Cover":
			//CT.CN.GD.FS
			n := map[string]map[string]map[string][]uint{}
			r := map[string]map[string]map[string][]string{}

			dn := p["Domain"]
			dr := RT.Rtdb.Read_Domain_Record(dn)

			if dr.RoutePlan != nil {

				for _, rid := range dr.RoutePlan {
					RT.Rtdb.Locks["routes"].RLock()
					for ac, rps := range RT.Rtdb.Routes {
						for _rid, _ := range rps {
							if _rid == rid {
								acs := strings.Split(ac, ".")
								for i, a := range acs {
									if i == 0 && r[a] == nil {
										n[a] = map[string]map[string][]uint{}
										r[a] = map[string]map[string][]string{}
									}
									if i == 2 && r[acs[0]][a] == nil {
										n[acs[0]][a] = map[string][]uint{}
										r[acs[0]][a] = map[string][]string{}
									}
									if i == 3 && r[acs[0]][acs[2]][a] == nil {
										n[acs[0]][acs[2]][a] = []uint{}
										r[acs[0]][acs[2]][a] = []string{}
										if rr := RT.Rtdb.Read_Route_Record(ac, rid); rr.Nodes != nil {
											for nid, _ := range rr.Nodes {
												n[acs[0]][acs[2]][a] = append(n[acs[0]][acs[2]][a], nid)
											}
										}
									}
								}
							}
						}
					}
					RT.Rtdb.Locks["routes"].RUnlock()
				}

				for a, va := range n {
					for b, vb := range va {
						for c, d := range vb {
							msr := make(map[string]int)
							for _, nid := range d {
								nr := RT.Rtdb.Read_Node_Record(nid)
								for _, sid := range nr.ServerList {
									sr := RT.Rtdb.Read_Server_Record(sid)
									msr[sr.ServerIp] = 1
								}
							}
							for sip, _ := range msr {
								r[a][b][c] = append(r[a][b][c], sip)
							}
						}
					}
				}

			}

			G.Outlog3(G.LOG_AMQP, "Cover data: %+v", r)
			_msg1 = r

		case "Source":
		}

		if msg.ID == 0 {
			break
		}

		if err = Sendmsg2("", AMQP_CMD_DATA, &_param, AMQP_OBJ_API,
			&_msg1, "", msg.Sender, msg.ID, false); err != nil {
			G.Outlog3(G.LOG_AMQP, "Error send API data: %s", err)
		} else {
			if G.Debug {
				G.Outlog3(G.LOG_API, "Sent API data: %+v", _msg1)
			}
		}
	}

	return err
}
