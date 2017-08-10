package main

import (
	"3grid/amqp"
	"3grid/dns"
	"3grid/ip"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var (
	debug        = flag.Bool("debug", true, "output debug info")
	uri          = flag.String("uri", "amqp://gslb:gslb@gslb-amqp.chinamaincloud.com:5672//gslb", "AMQP URI")
	exchange     = flag.String("exchange", "gslb-exchange", "Durable, non-auto-deleted AMQP exchange name")
	exchangeType = flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	queue_c      = flag.String("queue_c", "gslb-queue-c", "Ephemeral AMQP consumer queue name")
	queue_p      = flag.String("queue_p", "gslb-queue-p", "Ephemeral AMQP producer queue name")
	bindingKey_c = flag.String("key_c", "gslb-key-c", "AMQP binding key")
	bindingKey_p = flag.String("key_p", "gslb-key-p", "AMQP binding key")
	consumerTag  = flag.String("consumer-tag", "gslb-3grid", "AMQP consumer tag (should not be blank)")
	producerTag  = flag.String("producer-tag", "gslb-3grid", "AMQP producer tag (should not be blank)")
	lifetime     = flag.Duration("lifetime", 0*time.Second, "lifetime of process before shutdown (0s=infinite)")
)

var amqp_c *grid_amqp.AMQP_Consumer

//var amqp_p *grid_amqp.AMQP_Producer

func synchronize(_interval int) {
	//function to synchronize ip & route db
	var err error
	amqp_c, err = grid_amqp.NewAMQPConsumer(*uri, *exchange, *exchangeType, *queue_c, *bindingKey_c, *consumerTag)

	if err != nil {
		log.Fatalf("%s", err)
	} else {
		//amqp_p, err = grid_amqp.NewAMQPProducer(*uri, *exchange, *exchangeType, *queue_p, *bindingKey_p, *producerTag)
		for {
			if err = amqp_c.Publish("Hello, GSLB", *bindingKey_p); err != nil {
				log.Fatalf("%s", err)
			}
			time.Sleep(time.Duration(_interval) * time.Second)
		}
	}

	defer func() {
		log.Printf("shutting down")
		if err := amqp_c.Shutdown(); err != nil {
			log.Fatalf("error during shutdown: %s", err)
		}
		/*if err := amqp_p.Shutdown(); err != nil {
			log.Fatalf("error during shutdown: %s", err)
		}*/
	}()
}

func serve(net, name, secret string, num int) {
	ipdb := grid_ip.IP_db{}
	ipdb.IP_db_init()

	worker := grid_dns.DNS_worker{}
	worker.Id = num
	worker.Ipdb = &ipdb

	switch name {
	case "":
		worker.Server = &dns.Server{Addr: ":" + port, Net: net, TsigSecret: nil}
		worker.Server.Handler = &worker
		if err := worker.Server.ListenAndServe(); err != nil {
			fmt.Printf("Failed to setup the "+net+" server: %s\n", err.Error())
		}
	default:
		worker.Server = &dns.Server{Addr: ":" + port, Net: net, TsigSecret: map[string]string{name: secret}}
		worker.Server.Handler = &worker
		if err := worker.Server.ListenAndServe(); err != nil {
			fmt.Printf("Failed to setup the "+net+" server: %s\n", err.Error())
		}
	}
}

func read_conf() {
	viper.SetConfigName("grid")
	viper.AddConfigPath("/etc")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
	} else {
		_num_cpus := viper.GetInt("server.cpus")
		if _num_cpus == 0 {
			num_cpus = runtime.NumCPU()
		} else {
			num_cpus = _num_cpus
		}
		_port := viper.GetString("server.port")
		if _port == "" {
			port = "53"
		} else {
			port = _port
		}
		_daemon := viper.GetBool("server.daemon")
		if _daemon == false {
			daemond = false
		} else {
			daemond = true
		}
		_interval := viper.GetInt("server.interval")
		if _interval < 30 {
			interval = 30
		} else {
			interval = _interval
		}
		_logger := viper.GetString("server.log")
		if _logger == "" {
			logger = "3grid.log"
		} else {
			logger = _logger
		}
		if *debug {
			fmt.Printf("cpus:%d\n", num_cpus)
			fmt.Printf("port:%s\n", port)
			fmt.Printf("daemon:%t\n", daemond)
			fmt.Printf("interval:%d\n", interval)
			fmt.Printf("log:%s\n", logger)
		}
	}
}

var num_cpus int
var port string
var daemond bool
var interval int
var logger string

func main() {

	read_conf()

	if daemond {
		context := new(daemon.Context)
		child, _ := context.Reborn()

		if child != nil {
			os.Exit(0)
		}

		defer context.Release()
	}

	//after fork as daemon, go on working
	runtime.GOMAXPROCS(num_cpus + 1)

	var name, secret string
	for i := 0; i < num_cpus; i++ {
		go serve("udp", name, secret, i)
	}

	go synchronize(interval)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Printf("Signal (%s) received, stopping\n", s)
}
