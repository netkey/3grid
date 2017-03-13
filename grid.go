package main

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/viper"
	"grid/dns"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

func serve(net, name, secret string, num int) {
	worker := grid.DNS_worker{}
	worker.Id = num
	worker.Ipcache = make(map[string]string)
	worker.Ipdb, _ = geoip2.Open("ip/GeoLite2-City.mmdb")
	worker.Lock = new(sync.RWMutex)

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
		/*
			fmt.Printf("cpus:%d\n", num_cpus)
			fmt.Printf("port:%s\n", port)
			fmt.Printf("daemon:%t\n", daemond)
		*/
	}
}

var num_cpus int
var port string
var daemond bool

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

	runtime.GOMAXPROCS(num_cpus)

	var name, secret string
	for i := 0; i < num_cpus; i++ {
		go serve("udp", name, secret, i)
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Printf("Signal (%s) received, stopping\n", s)
}
