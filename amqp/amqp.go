package grid_amqp

import (
	G "3grid/tools/globals"
	"fmt"
	"github.com/streadway/amqp"
	"reflect"
)

const (
	AMQP_CMD_KA      = "Ka"      //Keepalive
	AMQP_CMD_ONLINE  = "Online"  //Online notify
	AMQP_CMD_OFFLINE = "Offline" //Offline notify
	AMQP_CMD_STATE   = "State"   //State notify
	AMQP_CMD_VER     = "Ver"     //Version Update
	AMQP_CMD_ADD     = "Add"     //Add data
	AMQP_CMD_DEL     = "Del"     //Delete data
	AMQP_CMD_UPDATE  = "Update"  //Update data
	AMQP_CMD_GET     = "Get"     //Get data
	AMQP_CMD_DATA    = "Data"    //Data requested by Get command
	AMQP_CMD_ACK     = "Ack"     //Confirm msg
	AMQP_PARAM_ZIP   = "Gzip"    //Whether msg is zipped
	AMQP_PARAM_ACK   = "Ack"     //Whether msg is being required to confirm
	AMQP_OBJ_IP      = "Ipdb"    //Object ip db
	AMQP_OBJ_ROUTE   = "Routedb" //Object route db
	AMQP_OBJ_CONTROL = "Control" //Object control db
	AMQP_OBJ_CMDB    = "Cmdb"    //Object cm db
	AMQP_OBJ_DOMAIN  = "Domain"  //Object domain db
	AMQP_OBJ_API     = "Api"     //API request
	AMQP_OBJ_NET     = "Net"     //Object net probe
)

type AMQP_Message struct {
	ID      uint
	Sender  string
	Command string
	Params  *map[string]string
	Object  string
	Msg1    *map[string]map[string]map[string][]string
	Msg2    string
	Gzip    bool
	Ack     bool
}

type AMQP_Broadcaster struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	exc     string
	rkey    string
	tag     string
	myname  string
	done    chan error
}

func NewAMQPBroadcaster(amqpURI, exchange, exchangeType, queueName, routingKey, myname string) (*AMQP_Broadcaster, error) {
	c := &AMQP_Broadcaster{
		conn:    nil,
		channel: nil,
		exc:     exchange,
		rkey:    routingKey,
		tag:     routingKey,
		myname:  myname,
		done:    make(chan error),
	}

	var err error

	G.Outlog3(G.LOG_AMQP, "dialing %q", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		G.Outlog3(G.LOG_AMQP, "closing: %s, re-dialing..", <-c.conn.NotifyClose(make(chan *amqp.Error)))
		AMQP_B_RECONNECT()
	}()

	G.Outlog3(G.LOG_AMQP, "got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "got Channel, declaring %q Exchange (%q)", exchangeType, exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "declared Exchange, declaring Queue %q", queueName)
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Declare: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)", queue.Name, queue.Messages, queue.Consumers, c.rkey)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		"",         // routingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "Queue bound to Exchange, starting Broadcast (broadcaster tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		true,       // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go c.amqp_handle_b(deliveries, c.done)

	return c, nil
}

func (c *AMQP_Broadcaster) Publish(body []byte) error {
	var err error

	//G.Outlog3(G.LOG_AMQP, "broadcasting %dB body to %s", len(body), c.exc)
	if err = c.channel.Publish(
		c.exc, // publish to an exchange
		"",    // routing to 0 or more queues
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "application/json",
			ContentEncoding: "UTF-8",
			Body:            body,
			DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:        0,              // 0-9
		},
	); err != nil {
		return fmt.Errorf("Exchange Publish: %s", err)
	}

	return nil
}

func (c *AMQP_Broadcaster) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Broadcaster cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer G.Outlog3(G.LOG_AMQP, "AMQP_Broadcaster shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func (c *AMQP_Broadcaster) amqp_handle_b(deliveries <-chan amqp.Delivery, done chan error) {
	var err error
	var msg AMQP_Message
	var cmds *Cmds
	var params []reflect.Value
	var _param map[string]string
	var _msg1 []string

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic amqp_handle_b: %s", pan)
		}
	}()

	for d := range deliveries {

		//for concurrently handling message, msg&params&cmds must be separated
		msg = AMQP_Message{}
		cmds = &Cmds{}
		params = make([]reflect.Value, 2)
		_param = make(map[string]string)
		_msg1 = []string{}

		if err = Transmsg(d.Body, &msg); err != nil {
			G.Outlog3(G.LOG_AMQP, "transmsg: %s", err)
			continue
		}
		if msg.Sender == c.myname {
			continue
		}
		//call msg handler functions
		fn, r := reflect.TypeOf(cmds).MethodByName(msg.Command)
		if r == false {
			G.Outlog3(G.LOG_AMQP, "error reflect cmds: %s", msg.Command)
		} else {
			params[0] = reflect.ValueOf(cmds)
			params[1] = reflect.ValueOf(&msg)
			go fn.Func.Call(params) //handle messages concurrently
			/*
				v := fn.Func.Call(params)
				if v[0].IsNil() == false {
					G.Outlog3(G.LOG_AMQP, "error handing msg: %+v", v[0].Interface())
				}
			*/
		}
		//need ack?
		if msg.Ack == true {
			if err = Sendmsg("", AMQP_CMD_ACK, &_param, msg.Object, &_msg1, "", msg.Sender, msg.ID); err != nil {
				G.Outlog3(G.LOG_AMQP, "error sending ack msg: %s", err)
			}
		}
	}

	G.Outlog3(G.LOG_AMQP, "handle: deliveries channel closed")

	done <- nil
}

type AMQP_Director struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	exc     string
	rkey    string
	tag     string
	myname  string
	done    chan error
}

func NewAMQPDirector(amqpURI, exchange, exchangeType, queueName, routingKey, myname string) (*AMQP_Director, error) {
	c := &AMQP_Director{
		conn:    nil,
		channel: nil,
		exc:     exchange,
		rkey:    routingKey,
		tag:     routingKey,
		myname:  myname,
		done:    make(chan error),
	}

	var err error

	G.Outlog3(G.LOG_AMQP, "dialing %q", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		G.Outlog3(G.LOG_AMQP, "closing: %s, re-dialing..", <-c.conn.NotifyClose(make(chan *amqp.Error)))
		AMQP_D_RECONNECT()
	}()

	G.Outlog3(G.LOG_AMQP, "got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "got Channel, declaring %q Exchange (%q)", exchangeType, exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "declared Exchange, declaring Queue %q", queueName)
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Declare: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
		queue.Name, queue.Messages, queue.Consumers, c.rkey)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		c.rkey,     // bindingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	G.Outlog3(G.LOG_AMQP, "Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		true,       // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go c.amqp_handle_d(deliveries, c.done)

	return c, nil
}

func (c *AMQP_Director) Publish(body []byte, _key string) error {
	var err error
	var key string

	if _key == "" {
		key = c.rkey
	} else {
		key = _key
	}

	//G.Outlog3(G.LOG_AMQP, "publishing %dB body to %s of %s", len(body), c.rkey, c.exc)
	if err = c.channel.Publish(
		c.exc, // publish to an exchange
		key,   // routing to 0 or more queues
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "application/json",
			ContentEncoding: "UTF-8",
			Body:            body,
			DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:        0,              // 0-9
		},
	); err != nil {
		return fmt.Errorf("Exchange Publish: %s", err)
	}

	return nil
}

func (c *AMQP_Director) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Director cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer G.Outlog3(G.LOG_AMQP, "AMQP_Director shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func (c *AMQP_Director) amqp_handle_d(deliveries <-chan amqp.Delivery, done chan error) {
	var err error
	var msg AMQP_Message
	var cmds *Cmds
	var params []reflect.Value
	var _param map[string]string
	var _msg1 []string

	defer func() {
		if pan := recover(); pan != nil {
			G.Outlog3(G.LOG_GSLB, "Panic amqp_handle_d: %s", pan)
		}
	}()

	for d := range deliveries {

		//for concurrently handling message, msg&params&cmds must be separated
		msg = AMQP_Message{}
		cmds = &Cmds{}
		params = make([]reflect.Value, 2)
		_param = make(map[string]string)
		_msg1 = []string{}

		if err = Transmsg(d.Body, &msg); err != nil {
			G.Outlog3(G.LOG_AMQP, "transmsg: %s", err)
			continue
		}
		if msg.Sender == c.myname {
			continue
		}
		//call msg handler functions
		fn, r := reflect.TypeOf(cmds).MethodByName(msg.Command)
		if r == false {
			G.Outlog3(G.LOG_AMQP, "error reflect cmds: %s", msg.Command)
		} else {
			params[0] = reflect.ValueOf(cmds)
			params[1] = reflect.ValueOf(&msg)

			go fn.Func.Call(params) //handle messages concurrently
			/*
				v := fn.Func.Call(params)
				if v[0].IsNil() == false {
					G.Outlog3(G.LOG_AMQP, "error handing msg: %s", v[0].Interface())
				}
			*/
		}
		//need ack?
		if msg.Ack == true {
			if err = Sendmsg("", AMQP_CMD_ACK, &_param, msg.Object, &_msg1, "", msg.Sender, msg.ID); err != nil {
				G.Outlog3(G.LOG_AMQP, "error sending ack msg: %s", err)
			}
		}
	}

	G.Outlog3(G.LOG_AMQP, "handle: deliveries channel closed")

	done <- nil
}
