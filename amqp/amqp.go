package grid_amqp

import (
	"fmt"
	"github.com/streadway/amqp"
	"log"
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
	AMQP_CMD_ACK     = "Ack"     //Confirm msg
	AMQP_PARAM_ZIP   = "Gzip"    //Whether msg is zipped
	AMQP_PARAM_ACK   = "Ack"     //Whether msg is being required to confirm
	AMQP_OBJ_IP      = "Ipdb"    //Object ip db
	AMQP_OBJ_ROUTE   = "Routedb" //Object route db
	AMQP_OBJ_CONTROL = "Control" //Object control db
	AMQP_OBJ_CMDB    = "Cmdb"    //Object cm db
	AMQP_OBJ_DOMAIN  = "Domain"  //Object domain db
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

	log.Printf("dialing %q", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		log.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	log.Printf("got Channel, declaring %q Exchange (%q)", exchangeType, exchange)
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

	log.Printf("declared Exchange, declaring Queue %q", queueName)
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

	log.Printf("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
		queue.Name, queue.Messages, queue.Consumers, c.rkey)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		"",         // routingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	log.Printf("Queue bound to Exchange, starting Broadcast (broadcaster tag %q)", c.tag)
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

	//log.Printf("broadcasting %dB body to %s", len(body), c.exc)
	if err = c.channel.Publish(
		c.exc, // publish to an exchange
		"",    // routing to 0 or more queues
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "application/json",
			ContentEncoding: "",
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

	defer log.Printf("AMQP_Broadcaster shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func (c *AMQP_Broadcaster) amqp_handle_b(deliveries <-chan amqp.Delivery, done chan error) {
	var err error
	var msg AMQP_Message
	var cmds = &Cmds{}
	var params = make([]reflect.Value, 2)
	var _param = make(map[string]string)
	var _msg1 []string

	for d := range deliveries {
		if err = Transmsg(d.Body, &msg); err != nil {
			log.Printf("transmsg: %s", err)
			continue
		}
		if msg.Sender == c.myname {
			continue
		}
		//call msg handler functions
		fn, r := reflect.TypeOf(cmds).MethodByName(msg.Command)
		if r == false {
			log.Printf("error reflect cmds: %s", msg.Command)
		} else {
			params[0] = reflect.ValueOf(cmds)
			params[1] = reflect.ValueOf(&msg)
			v := fn.Func.Call(params)
			if v[0].IsNil() == false {
				log.Printf("error handing msg: %+v", v[0].Interface())
			}
		}
		//need ack?
		if msg.Ack == true {
			if err = Sendmsg("", AMQP_CMD_ACK, &_param, msg.Object, &_msg1, "", msg.Sender, msg.ID); err != nil {
				log.Printf("error sending ack msg: %s", err)
			}
		}
	}
	log.Printf("handle: deliveries channel closed")
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

	log.Printf("dialing %q", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		log.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	log.Printf("got Channel, declaring %q Exchange (%q)", exchangeType, exchange)
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

	log.Printf("declared Exchange, declaring Queue %q", queueName)
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

	log.Printf("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
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

	log.Printf("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
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

	//log.Printf("publishing %dB body to %s of %s", len(body), c.rkey, c.exc)
	if err = c.channel.Publish(
		c.exc, // publish to an exchange
		key,   // routing to 0 or more queues
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "application/json",
			ContentEncoding: "",
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

	defer log.Printf("AMQP_Director shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func (c *AMQP_Director) amqp_handle_d(deliveries <-chan amqp.Delivery, done chan error) {
	var err error
	var msg AMQP_Message
	var cmds = &Cmds{}
	var params = make([]reflect.Value, 2)
	var _param = make(map[string]string)
	var _msg1 []string

	for d := range deliveries {
		if err = Transmsg(d.Body, &msg); err != nil {
			log.Printf("transmsg: %s", err)
			continue
		}
		if msg.Sender == c.myname {
			continue
		}
		//call msg handler functions
		fn, r := reflect.TypeOf(cmds).MethodByName(msg.Command)
		if r == false {
			log.Printf("error reflect cmds: %s", msg.Command)
		} else {
			params[0] = reflect.ValueOf(cmds)
			params[1] = reflect.ValueOf(&msg)
			v := fn.Func.Call(params)
			if v[0].IsNil() == false {
				log.Printf("error handing msg: %s", v[0].Interface())
			}
		}
		//need ack?
		if msg.Ack == true {
			if err = Sendmsg("", AMQP_CMD_ACK, &_param, msg.Object, &_msg1, "", msg.Sender, msg.ID); err != nil {
				log.Printf("error sending ack msg: %s", err)
			}
		}
	}
	log.Printf("handle: deliveries channel closed")
	done <- nil
}
