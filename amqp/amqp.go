package grid_amqp

import (
	"fmt"
	"github.com/streadway/amqp"
	"log"
)

type AMQP_Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	exc     string
	rkey    string
	tag     string
	done    chan error
}

func NewAMQPConsumer(amqpURI, exchange, exchangeType, queueName, key, ctag string) (*AMQP_Consumer, error) {
	c := &AMQP_Consumer{
		conn:    nil,
		channel: nil,
		exc:     exchange,
		rkey:    key,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	log.Printf("dialing %q", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		fmt.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
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
		queue.Name, queue.Messages, queue.Consumers, key)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		key,        // bindingKey
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
		false,      // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go amqp_handle(deliveries, c.done)

	return c, nil
}

func (c *AMQP_Consumer) Publish(body, key string) error {
	var err error

	log.Printf("publishing %dB body (%q) to %s of %s", len(body), body, key, c.exc)
	if err = c.channel.Publish(
		c.exc, // publish to an exchange
		key,   // routing to 0 or more queues
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "text/plain",
			ContentEncoding: "",
			Body:            []byte(body),
			DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:        0,              // 0-9
		},
	); err != nil {
		log.Printf("Exchange Publish: %s", err)
		return fmt.Errorf("Exchange Publish: %s", err)
	}

	return nil
}

func (c *AMQP_Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer log.Printf("AMQP_Consumer shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func amqp_handle(deliveries <-chan amqp.Delivery, done chan error) {
	for d := range deliveries {
		log.Printf(
			"got %dB delivery: [%v] %q",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)
		d.Ack(false)
	}
	log.Printf("handle: deliveries channel closed")
	done <- nil
}

type AMQP_Producer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	exc     string
	rkey    string
	tag     string
	done    chan error
}

func NewAMQPProducer(amqpURI, exchange, exchangeType, queueName, key, ctag string) (*AMQP_Producer, error) {
	c := &AMQP_Producer{
		conn:    nil,
		channel: nil,
		exc:     exchange,
		rkey:    key,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	// This function dials, connects, declares, publishes, and tears down,
	// all in one go. In a real service, you probably want to maintain a
	// long-lived connection as state, and publish against that.

	log.Printf("dialing %q", amqpURI)
	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		log.Printf("Dial: %s", err)
		return nil, fmt.Errorf("Dial: %s", err)
	}

	defer c.conn.Close()

	log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		log.Printf("Channel: %s", err)
		return nil, fmt.Errorf("Channel: %s", err)
	}

	log.Printf("got Channel, declaring %q Exchange (%q)", exchangeType, exchange)
	if err := c.channel.ExchangeDeclare(
		exchange,     // name
		exchangeType, // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		log.Printf("Exchange Declare: %s", err)
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	log.Printf("declared Exchange")

	return c, nil
}

func (c *AMQP_Producer) Publish(body string) error {
	var err error

	log.Printf("publishing %dB body (%q) to %s of %s", len(body), body, c.rkey, c.exc)
	if err = c.channel.Publish(
		c.exc,  // publish to an exchange
		c.rkey, // routing to 0 or more queues
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "text/plain",
			ContentEncoding: "",
			Body:            []byte(body),
			DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:        0,              // 0-9
		},
	); err != nil {
		log.Printf("Exchange Publish: %s", err)
		return fmt.Errorf("Exchange Publish: %s", err)
	}

	return nil
}

func (c *AMQP_Producer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Producer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer log.Printf("AMQP_Producer shutdown OK")

	// wait for handle() to exit
	return <-c.done
}
