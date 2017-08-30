package main

import (
	"3grid/amqp"
	"3grid/dns"
	"3grid/ip"
	"3grid/tools"
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
		_interval := viper.GetInt("server.interval")
		if _interval < 30 {
			interval = 30
		} else {
			interval = _interval
		}
		_keepalive := viper.GetInt("server.keepalive")
		if _keepalive < 30 {
			keepalive = 30
		} else {
			keepalive = _keepalive
		}
		_myname := viper.GetString("server.myname")
		if _myname == "" {
			myname = "3grid"
		} else {
			myname = _myname
		}
		if *debug {
			log.Printf("grid running - cpus:%d port:%s daemon:%t debug:%t interval:%d keepalive:%d myname:%s", num_cpus, port, daemond, *debug, interval, keepalive, myname)
		}
	}
}

func main() {

	var err error

	flag.Parse()
	read_conf()

	grid_ip.Ver_Major, grid_ip.Ver_Minor, grid_ip.Ver_Patch, grid_ip.Version, grid_ip.Db_file, err = grid_tools.Check_db_version("ip")

	if err != nil {
		log.Printf("Check db version error: %s", err)
	} else {
		if *debug {
			log.Printf("IP db version:%s, major:%d, minor:%d, patch:%d, file_patch:%s", grid_ip.Version, grid_ip.Ver_Major, grid_ip.Ver_Minor, grid_ip.Ver_Patch, grid_ip.Db_file)
		}
	}

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

	var name, secret string
	for i := 0; i < num_cpus; i++ {
		go grid_dns.Working("udp", port, name, secret, i)
	}

	go grid_amqp.Synchronize(interval, keepalive, myname)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Printf("Signal (%s) received, stopping\n", s)
}
