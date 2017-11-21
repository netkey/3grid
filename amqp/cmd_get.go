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
			n := make(map[string]map[string]map[string][]uint)   //holds nodes info
			r := make(map[string]map[string]map[string][]string) //holds ip info(return results)

			dn := p["Domain"]
			dr := RT.Rtdb.Read_Domain_Record(dn)

			if dr.RoutePlan == nil {
				break
			} else {
				G.Outlog3(G.LOG_API, "Cover Route Plan: %+v", dr.RoutePlan)
			}

			for _, rid := range dr.RoutePlan {
				//walk through routes
				RT.Rtdb.Locks["routes"].RLock()
				for ac, _ := range RT.Rtdb.Routes {
					rr := RT.Rtdb.Read_Route_Record(ac, rid)
					if rr.Nodes == nil {
						continue
					}
					a, b, c := "", "", ""
					//ac:CTC.CN.HAN.GD  acs:0.1.2.3
					acs := strings.Split(ac, ".")
					la := len(acs)
					if la > 0 {
						a = acs[0]
						if r[a] == nil {
							n[a] = make(map[string]map[string][]uint)
							r[a] = make(map[string]map[string][]string)
							G.Outlog3(G.LOG_API, "Cover: nil a:%s n:%+v", a, n[a])
						}
						if la == 1 || la == 2 {
							b = "*"
							c = "*"
							n[a] = map[string]map[string][]uint{b: {c: {}}}
							r[a] = map[string]map[string][]string{b: {c: {}}}
						}
					}
					if la > 2 {
						b = acs[2]
						if r[a][b] == nil {
							n[a][b] = make(map[string][]uint)
							r[a][b] = make(map[string][]string)
							G.Outlog3(G.LOG_API, "Cover: nil a:%s b:%s n:%+v", a, b, n[a][b])
						}
						if la == 3 {
							c = "*"
							n[a][b] = map[string][]uint{c: {}}
							r[a][b] = map[string][]string{c: {}}
						}
					}
					if la > 3 {
						c = acs[3]
						if r[a][b][c] == nil {
							n[a][b][c] = []uint{}
							r[a][b][c] = []string{}
							G.Outlog3(G.LOG_API, "Cover: nil a:%s b:%s c:%s n:%+v", a, b, c, n[a][b][c])
						} else {
							G.Outlog3(G.LOG_API, "Cover: not nil a:%s b:%s c:%s %+v", a, b, c, n[a][b][c])
							continue
						}
					}
					for nid, _ := range rr.Nodes {
						//append node list
						n[a][b][c] = append(n[a][b][c], nid)
					}
					G.Outlog3(G.LOG_API, "Cover data: rid:%d a:%s b:%s c:%s n:%+v", rid, a, b, c, n[a][b][c])
				}
				RT.Rtdb.Locks["routes"].RUnlock()
			}

			G.Outlog3(G.LOG_API, "Cover N: %+v", n["CMCC"])
			for a, va := range n {
				for b, vb := range va {
					for c, d := range vb {
						msr := make(map[uint]map[string]int)
						for _, nid := range d {
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
						//G.Outlog3(G.LOG_API, "Cover data: a:%s b:%s c:%s data:%+v", a, b, c, r[a][b][c])
					}
				}
			}

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
