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

//main program related
var num_cpus int
var port string
var daemond bool

//gslb related
var myname string
var interval int
var keepalive int
var acprefix string
var cutoff_percent int
var deny_percent int
var ip_cache_ttl int
var rt_cache_ttl int
var ip_cache_size int
var rt_cache_size int

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
		_cutoff_percent := viper.GetInt("gslb.cutoff_percent")
		if _cutoff_percent < 80 {
			cutoff_percent = 80
		} else {
			cutoff_percent = _cutoff_percent
		}
		_deny_percent := viper.GetInt("gslb.deny_percent")
		if _deny_percent < 90 {
			deny_percent = 90
		} else {
			deny_percent = _deny_percent
		}
		_ip_cache_ttl := viper.GetInt("gslb.ip_cache_ttl")
		if _ip_cache_ttl < 60 {
			ip_cache_ttl = 60
		} else {
			ip_cache_ttl = _ip_cache_ttl
		}
		_ip_cache_size := viper.GetInt("gslb.ip_cache_size")
		if _ip_cache_size < 1000 {
			ip_cache_size = 1000
		} else {
			ip_cache_size = _ip_cache_size
		}
		_rt_cache_ttl := viper.GetInt("gslb.rt_cache_ttl")
		if _rt_cache_ttl < 60 {
			rt_cache_ttl = 60
		} else {
			rt_cache_ttl = _rt_cache_ttl
		}
		_rt_cache_size := viper.GetInt("gslb.rt_cache_size")
		if _rt_cache_size < 1000 {
			rt_cache_size = 1000
		} else {
			rt_cache_size = _rt_cache_size
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

	IP.Ipdb = &IP.IP_db{}
	IP.Ipdb.IP_db_init()
	IP.Ip_Cache_TTL = ip_cache_ttl
	IP.Ip_Cache_Size = ip_cache_size

	RT.Rtdb = &RT.Route_db{}
	RT.Rtdb.RT_db_init()
	RT.MyACPrefix = acprefix
	RT.Service_Cutoff_Percent = uint(cutoff_percent)
	RT.Service_Deny_Percent = uint(deny_percent)
	RT.RT_Cache_TTL = rt_cache_ttl
	RT.RT_Cache_Size = int64(rt_cache_size)

	G.GP = G.GSLB_Params{}
	G.GP.Init(keepalive)

	var name, secret string
	for i := 0; i < num_cpus; i++ {
		go D.Working("udp", port, name, secret, i, IP.Ipdb, RT.Rtdb)
	}

	go A.Synchronize(interval, keepalive, myname)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Printf("Signal (%s) received, stopping\n", s)
}
