package grid_amqp

import (
	"encoding/json"
	"flag"
	"log"
	"time"
)

var (
	amqp_uri       = flag.String("amqp-uri", "amqp://gslb:gslb@gslb-amqp.chinamaincloud.com:5672//gslb", "AMQP URI")
	exchange       = flag.String("exchange", "gslb-exchange", "Durable, non-auto-deleted AMQP exchange name")
	exchangeType   = flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	broadcast      = flag.String("broadcast", "gslb-broadcast", "Durable, non-auto-deleted AMQP exchange name")
	queue_b        = flag.String("queue-b", "gslb-queue-b", "Ephemeral AMQP consumer queue name")
	queue_c        = flag.String("queue-c", "gslb-queue-c", "Ephemeral AMQP consumer queue name")
	queue_p        = flag.String("queue-p", "gslb-queue-p", "Ephemeral AMQP producer queue name")
	bindingKey_b   = flag.String("key-b", "gslb-key-b", "AMQP binding key")
	bindingKey_c   = flag.String("key-c", "gslb-key-c", "AMQP binding key")
	bindingKey_p   = flag.String("key-p", "gslb-key-p", "AMQP binding key")
	broadcasterTag = flag.String("broadcaster-tag", "gslb-3grid", "AMQP broadcaster tag (should not be blank)")
	consumerTag    = flag.String("consumer-tag", "gslb-3grid", "AMQP consumer tag (should not be blank)")
	producerTag    = flag.String("producer-tag", "gslb-3grid", "AMQP producer tag (should not be blank)")
)

var AMQP_B *AMQP_Broadcaster
var AMQP_C *AMQP_Consumer
var AMQP_P *AMQP_Producer

func Synchronize(_interval int, _myname string) {
	//function to synchronize ip & route db
	var err error

	keyb := _myname + "-b"
	keyc := _myname + "-c"
	keyp := _myname + "-p"
	bindingKey_b, queue_b, broadcasterTag = &keyb, &keyb, &keyb
	bindingKey_c, queue_c, consumerTag = &keyc, &keyc, &keyc
	bindingKey_p, queue_p, producerTag = &keyp, &keyp, &keyp

	AMQP_C, err = NewAMQPConsumer(*amqp_uri, *exchange, *exchangeType, *queue_c, *bindingKey_c, *consumerTag)

	if err != nil {
		log.Fatalf("%s", err)
	} else {
		AMQP_P, err = NewAMQPProducer(*amqp_uri, *exchange, *exchangeType, *queue_p, *bindingKey_p, *producerTag)
		if err != nil {
			log.Fatalf("%s", err)
		} else {
			AMQP_B, err = NewAMQPBroadcaster(*amqp_uri, *broadcast, "fanout", *queue_b, *bindingKey_b, *broadcasterTag, _myname)
			if err != nil {
				log.Fatalf("%s", err)
			} else {
				go keepalive(60, _myname) //keepalive with backend
				for {
					time.Sleep(time.Duration(_interval) * time.Second)
				}
			}
		}
	}

	defer func() {
		log.Printf("shutting down")
		if err := AMQP_C.Shutdown(); err != nil {
			log.Fatalf("error during shutdown: %s", err)
		}
		if err := AMQP_P.Shutdown(); err != nil {
			log.Fatalf("error during shutdown: %s", err)
		}
	}()
}

func keepalive(_interval int, _myname string) {
	var err error
	for {
		if err = Sendmsg("", _myname, AMQP_CM_KA, "", "", ""); err != nil {
			log.Printf("keepalive: %s", err)
		}
		time.Sleep(time.Duration(_interval) * time.Second)
	}
}

func Sendmsg(_type, _myname, _command, _param, _obj, _msg string) error {
	var err error
	var jam []byte

	am := AMQP_Message{
		Sender:  _myname,
		Command: _command,
		Params:  _param,
		Object:  _obj,
		Content: _msg,
		Zip:     false,
		Confirm: false,
	}

	if jam, err = json.Marshal(am); err != nil {
		log.Fatalf("%s", err)
		return err
	}

	switch _type {
	case "broadcast":
		if err = AMQP_B.Publish(jam); err != nil {
			log.Fatalf("%s", err)
			return err
		}
	default:
		if err = AMQP_P.Publish(jam); err != nil {
			log.Fatalf("%s", err)
			return err
		}
	}

	return nil
}

func Transmsg(_msg []byte, _am *AMQP_Message) error {
	var err error

	if err = json.Unmarshal(_msg, _am); err != nil {
		log.Fatalf("%s", err)
		return err
	}

	return nil
}
