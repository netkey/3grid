package grid_amqp

import (
	"3grid/ip"
	"3grid/route"
	T "3grid/tools"
	G "3grid/tools/globals"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"
)

var (
	debug        = flag.Bool("amqp-debug", true, "output debug info")
	amqp_uri     = flag.String("amqp-uri", "amqp://gslb:gslb@gslb-amqp.chinamaincloud.com:5672//gslb", "URI")
	exchange     = flag.String("exchange", "gslb-exchange", "Durable, non-auto-deleted AMQP exchange name")
	exchangeType = flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	broadcast    = flag.String("broadcast", "gslb-broadcast", "Durable, non-auto-deleted AMQP exchange name")
	queue_b      = flag.String("queue-b", "gslb-queue-b", "Ephemeral AMQP broadcaster queue name")
	queue_d      = flag.String("queue-d", "gslb-queue-d", "Ephemeral AMQP director queue name")
	routingKey_b = flag.String("key-b", "gslb-key-b", "AMQP routing key of fanout exchange")
	routingKey_d = flag.String("key-d", "gslb-key-d", "AMQP routing key of direct exchange")
	gslb_center  = flag.String("gslb-center", "gslb-center", "AMQP routing key of gslb backends")
)

var myname string
var ka_interval int

var AMQP_B *AMQP_Broadcaster
var AMQP_D *AMQP_Director

var msgid AutoInc

type AutoInc struct {
	id uint
}

func (a *AutoInc) AutoID() uint {
	a.id = a.id + 1
	return a.id
}

func AMQP_D_RECONNECT() {
	var err error
	_myname := myname
	_ka_interval := ka_interval

	keyd := _myname + "-d"
	queue_d = &keyd
	routingKey_d = &_myname

	for {
		AMQP_D, err = NewAMQPDirector(*amqp_uri, *exchange, *exchangeType, *queue_d, *routingKey_d, _myname)
		if err != nil {
			time.Sleep(time.Duration(_ka_interval) * time.Second)
		} else {
			break
		}
	}
}

func AMQP_B_RECONNECT() {
	var err error
	_myname := myname
	_ka_interval := ka_interval

	keyb := _myname + "-d"
	queue_b = &keyb
	routingKey_b = &_myname

	for {
		AMQP_B, err = NewAMQPBroadcaster(*amqp_uri, *broadcast, "fanout", *queue_b, *routingKey_b, _myname)
		if err != nil {
			time.Sleep(time.Duration(_ka_interval) * time.Second)
		} else {
			break
		}
	}
}

func Synchronize(_interval, _ka_interval int, _myname string) {
	//function to synchronize ip & route db
	var err error

	myname = _myname
	ka_interval = _ka_interval

	keyb := _myname + "-b"
	keyd := _myname + "-d"
	queue_b = &keyb
	queue_d = &keyd
	routingKey_b, routingKey_d = &_myname, &_myname

	AMQP_D, err = NewAMQPDirector(*amqp_uri, *exchange, *exchangeType, *queue_d, *routingKey_d, _myname)
	if err != nil {
		log.Fatalf("%s", err)
	} else {
		AMQP_B, err = NewAMQPBroadcaster(*amqp_uri, *broadcast, "fanout", *queue_b, *routingKey_b, _myname)
		if err != nil {
			log.Fatalf("%s", err)
		} else {
			go CheckVersion(_interval)    //Check version of ipdb & routedb
			go Keepalive(_ka_interval)    //keepalive with backend servers
			go State_Notify(_ka_interval) //state notify with backend servers
			for {
				time.Sleep(time.Duration(_interval) * time.Second)
				T.Check_db_versions()
			}
		}
	}

	defer func() {
		G.Outlog(G.LOG_AMQP, fmt.Sprintf("shutting down"))
		if err := AMQP_D.Shutdown(); err != nil {
			log.Fatalf("error during direcror shutdown: %s", err)
		}
		if err := AMQP_B.Shutdown(); err != nil {
			log.Fatalf("error during broadcaster shutdown: %s", err)
		}
	}()
}

func CheckVersion(_interval int) {
	var err error
	var _param = make(map[string]string)
	var _msg1 []string

	for {
		time.Sleep(time.Duration(_interval) * time.Second)
		_param[AMQP_OBJ_IP] = grid_ip.Version
		_param[AMQP_OBJ_ROUTE] = grid_route.RT_Version
		_param[AMQP_OBJ_CMDB] = grid_route.CM_Version
		_param[AMQP_OBJ_DOMAIN] = grid_route.DM_Version
		if err = Sendmsg("", AMQP_CMD_VER, &_param, AMQP_OBJ_CONTROL, &_msg1, "", *gslb_center, 0); err != nil {
			G.Outlog(G.LOG_AMQP, fmt.Sprintf("checkversion: %s", err))
		}
	}
}

func State_Notify(_interval int) {
	var err error
	var _param *map[string]string
	var _msg1 []string

	for {
		time.Sleep(time.Duration(_interval) * time.Second)

		_param = G.PC.Read_Perfcs(G.PERF_DOMAIN)
		if err = Sendmsg("", AMQP_CMD_STATE, _param, AMQP_OBJ_DOMAIN, &_msg1, "", *gslb_center, 0); err != nil {
			G.Outlog(G.LOG_AMQP, fmt.Sprintf("state: %s", err))
		}
	}
}

func Keepalive(_interval int) {
	var err error
	var _param = make(map[string]string)
	var _msg1 []string
	var _first bool = true

	for {
		if _first {
			if err = Sendmsg("", AMQP_CMD_ONLINE, &_param, "", &_msg1, "", *gslb_center, 0); err != nil {
				G.Outlog(G.LOG_AMQP, fmt.Sprintf("online: %s", err))
			}
			_first = false
		} else {
			_param["Qps"] = strconv.FormatUint(G.GP.Read_Qps(), 10)
			_param["Load"] = strconv.FormatUint(G.GP.Read_Load(), 10)
			if err = Sendmsg("", AMQP_CMD_KA, &_param, "", &_msg1, "", *gslb_center, 0); err != nil {
				G.Outlog(G.LOG_AMQP, fmt.Sprintf("keepalive: %s", err))
			}
			//G.Outlog3(G.LOG_AMQP, "GP Perf: %+v", _param)
		}

		time.Sleep(time.Duration(_interval) * time.Second)
	}
}

func Sendmsg(_type, _command string, _param *map[string]string, _obj string, _msg1 *[]string, _msg2 string, _replyto string, _replyid uint) error {
	var err error
	var jam []byte
	var target, exchange string
	var _mid uint

	if _replyid != 0 {
		_mid = _replyid
	} else {
		_mid = msgid.AutoID()
	}

	am := AMQP_Message{
		ID:      _mid,
		Sender:  "",
		Command: _command,
		Params:  _param,
		Object:  _obj,
		//Msg1:    &map[string][]string{"": *_msg1},
		Msg1: &map[string]map[string]map[string][]string{"": {"": {"": *_msg1}}},
		Msg2: _msg2,
		Gzip: false,
		Ack:  false,
	}

	switch _type {
	case "broadcast":
		target = AMQP_B.rkey
		exchange = AMQP_B.exc
		am.Sender = AMQP_B.myname
		if jam, err = json.Marshal(am); err != nil {
			return err
		}
		if err = AMQP_B.Publish(jam); err != nil {
			return err
		}
	default:
		if _replyto == "" {
			target = AMQP_D.rkey
		} else {
			target = _replyto
		}
		exchange = AMQP_D.exc
		am.Sender = AMQP_D.myname
		if jam, err = json.Marshal(am); err != nil {
			return err
		}
		if err = AMQP_D.Publish(jam, target); err != nil {
			return err
		}
	}

	G.OutDebug(
		"msg to [%s] of [%s]: size [%v], msgid [%d], value: [%+v]",
		target,
		exchange,
		len(jam),
		am.ID,
		am,
	)

	return nil
}

func Transmsg(_msg []byte, _am *AMQP_Message) error {
	var err error

	if err = json.Unmarshal(_msg, _am); err != nil {
		G.Outlog(G.LOG_AMQP, fmt.Sprintf("trans msg: %s", _msg))
		return err
	}

	if _am.Gzip == true {
		var mbuf bytes.Buffer

		buf := bytes.NewBufferString(_am.Msg2)
		br := base64.NewDecoder(base64.StdEncoding, buf)

		zr, err := gzip.NewReader(br)
		if err != nil {
			return err
		}
		zw := bufio.NewWriter(&mbuf)

		if _, err := io.Copy(zw, zr); err != nil {
			return err
		}
		if err := zr.Close(); err != nil {
			return err
		}
		zw.Flush()

		if err = json.Unmarshal(mbuf.Bytes(), _am.Msg1); err != nil {
			return err
		}
	}

	G.OutDebug(
		"msg from [%s]: size [%v], msgid [%d], msg: [%+v] ",
		_am.Sender,
		len(_msg),
		_am.ID,
		_am,
	)

	return nil
}

func Sendmsg2(_type, _command string, _param *map[string]string, _obj string, _msg1 *map[string]map[string]map[string][]string, _msg2 string, _replyto string, _replyid uint) error {
	var err error
	var jam []byte
	var target, exchange string
	var _mid uint

	if _replyid != 0 {
		_mid = _replyid
	} else {
		_mid = msgid.AutoID()
	}

	am := AMQP_Message{
		ID:      _mid,
		Sender:  "",
		Command: _command,
		Params:  _param,
		Object:  _obj,
		Msg1:    _msg1,
		Msg2:    _msg2,
		Gzip:    false,
		Ack:     false,
	}

	switch _type {
	case "broadcast":
		target = AMQP_B.rkey
		exchange = AMQP_B.exc
		am.Sender = AMQP_B.myname
		if jam, err = json.Marshal(am); err != nil {
			return err
		}
		if err = AMQP_B.Publish(jam); err != nil {
			return err
		}
	default:
		if _replyto == "" {
			target = AMQP_D.rkey
		} else {
			target = _replyto
		}
		exchange = AMQP_D.exc
		am.Sender = AMQP_D.myname
		if jam, err = json.Marshal(am); err != nil {
			return err
		}
		if err = AMQP_D.Publish(jam, target); err != nil {
			return err
		}
	}

	G.OutDebug(
		"msg to [%s] of [%s]: size [%v], msgid [%d], value: [%+v]",
		target,
		exchange,
		len(jam),
		am.ID,
		am,
	)

	return nil
}
