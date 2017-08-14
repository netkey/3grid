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
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	debug = flag.Bool("debug", true, "output debug info")
)

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
		_myname := viper.GetString("server.myname")
		if _myname == "" {
			myname = "3grid"
		} else {
			myname = _myname
		}
		if *debug {
			fmt.Printf("cpus:%d\n", num_cpus)
			fmt.Printf("port:%s\n", port)
			fmt.Printf("daemon:%t\n", daemond)
			fmt.Printf("interval:%d\n", interval)
			fmt.Printf("myname:%s\n", myname)
		}
	}
}

var num_cpus int
var port string
var daemond bool
var interval int
var myname string

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

	go grid_amqp.Synchronize(interval, myname)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Printf("Signal (%s) received, stopping\n", s)
}
