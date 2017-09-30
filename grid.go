package main

import (
	A "3grid/amqp"
	D "3grid/dns"
	IP "3grid/ip"
	RT "3grid/route"
	T "3grid/tools"
	G "3grid/tools/globals"
	"flag"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var debug = flag.Bool("debug", true, "output debug info")

var num_cpus int
var port string
var daemond bool
var interval int
var keepalive int
var myname string
var acprefix string

func read_conf() {
	viper.SetConfigFile("grid.conf")
	viper.SetConfigType("toml")

	viper.AddConfigPath("/etc")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		log.Printf("Error reading config file: %s\n", err)
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
		_debug := viper.GetBool("server.debug")
		if _debug == false {
			*debug = false
		} else {
			*debug = true
		}
		_interval := viper.GetInt("gslb.interval")
		if _interval < 30 {
			interval = 30
		} else {
			interval = _interval
		}
		_keepalive := viper.GetInt("gslb.keepalive")
		if _keepalive < 30 {
			keepalive = 30
		} else {
			keepalive = _keepalive
		}
		_myname := viper.GetString("gslb.myname")
		if _myname == "" {
			myname = "3grid"
		} else {
			myname = _myname
		}
		_acprefix := viper.GetString("gslb.acprefix")
		if _acprefix == "" {
			acprefix = "MMY"
		} else {
			acprefix = _acprefix
		}
		if *debug {
			log.Printf("grid running - cpus:%d port:%s daemon:%t debug:%t interval:%d keepalive:%d myname:%s", num_cpus, port, daemond, *debug, interval, keepalive, myname)
		}
	}
}

func main() {

	flag.Parse()
	read_conf()

	G.Debug = *debug
	T.Check_db_versions()

	if daemond {
		context := new(daemon.Context)
		child, _ := context.Reborn()

		if child != nil {
			os.Exit(0)
		}

		defer context.Release()
	}

	//after fork as daemon, go on working
	runtime.GOMAXPROCS(num_cpus)

	ipdb := IP.IP_db{}
	ipdb.IP_db_init()

	rtdb := RT.Route_db{}
	rtdb.RT_db_init()
	RT.MyACPrefix = acprefix

	G.GP = G.GSLB_Params{}
	G.GP.Init(keepalive)

	var name, secret string
	for i := 0; i < num_cpus; i++ {
		go D.Working("udp", port, name, secret, i, &ipdb, &rtdb)
	}

	go A.Synchronize(interval, keepalive, myname)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Printf("Signal (%s) received, stopping\n", s)
}
