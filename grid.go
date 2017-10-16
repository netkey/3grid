package main

import (
	A "3grid/amqp"
	D "3grid/dns"
	IP "3grid/ip"
	RT "3grid/route"
	T "3grid/tools"
	G "3grid/tools/globals"
	"flag"
	"fmt"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

var debug = flag.Bool("debug", true, "output debug info")
var worker = flag.Bool("worker", false, "worker mode, no daemon mode")

//main program related
var num_cpus int
var port string
var daemond bool
var debug_info string
var master bool
var workdir string

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
var log_buf_size int
var log_enable bool

func read_conf() {
	viper.SetConfigFile("grid.conf")
	viper.SetConfigType("toml")

	viper.AddConfigPath("/etc")
	viper.AddConfigPath(workdir)
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
		_master := viper.GetBool("server.master")
		if _master == false {
			master = false
		} else {
			master = true
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
		_log_buf_size := viper.GetInt("gslb.log_buf_size")
		if _log_buf_size < 1000 {
			log_buf_size = 1000
		} else {
			log_buf_size = _log_buf_size
		}
		_log_enable := viper.GetBool("gslb.log_enable")
		if _log_enable == false {
			log_enable = false
		} else {
			log_enable = true
		}
		if *debug {
			debug_info = fmt.Sprintf("%s running - cpus:%d port:%s daemon:%t debug:%t interval:%d keepalive:%d myname:%s", myname, num_cpus, port, daemond, *debug, interval, keepalive, myname)
		}
	}
}

func main() {
	var err error

	workdir, _ = filepath.Abs(filepath.Dir(os.Args[0]))

	flag.Parse()
	read_conf()

	runtime.GOMAXPROCS(num_cpus)

	G.Debug = *debug

	if daemond {
		context := daemon.Context{}
		child, _ := context.Reborn()

		if child != nil {
			os.Exit(0)
		} else {
			defer context.Release()
		}
	}

	if master && !*worker {
		//I am master, fork worker and guard it
		env := os.Environ()
		attr := &os.ProcAttr{
			Env: env,
			Files: []*os.File{
				os.Stdin,
				os.Stdout,
				os.Stderr,
			},
		}

		child, _ := os.StartProcess("3grid", []string{os.Args[0], "-worker=1"}, attr)

		go guard_child(child)

		signal_loop(child)
	} else {
		//after fork as worker, go on working

		{

			//force enable log when debug mode
			if G.Debug && log_enable == false {
				log_enable = true
			}

			//init logger
			if log_enable {
				G.Log = true
				G.LogBufSize = log_buf_size
				if G.Logger, err = G.NewLogger(); err != nil {
					if G.Debug {
						log.Printf("Error making logger: %s", err)
					}
				} else {
					G.LogChan = &G.Logger.Chan
					go G.Logger.Output()
					go G.Logger.Checklogs()
					if G.Debug {
						log.SetOutput(G.Logger.Fds[G.LOG_DEBUG])
						if debug_info != "" {
							G.Outlog(G.LOG_DEBUG, fmt.Sprintf("%s", debug_info))
							G.Outlog(G.LOG_GSLB, fmt.Sprintf("%s", debug_info))
						}
					}
				}
			}
		}

		{
			//global perf counters
			G.GP = G.Perf_Counter{}
			G.GP.Init(keepalive, true)

			//specific oerf counters
			G.PC = G.Perfcs{}
			G.PC.Init(keepalive)

			T.Check_db_versions()
		}

		{
			//init ip db
			IP.Ipdb = &IP.IP_db{}
			IP.Ipdb.IP_db_init()
			IP.Ip_Cache_TTL = ip_cache_ttl
			IP.Ip_Cache_Size = ip_cache_size
		}

		{
			//init route/domain/cm db
			RT.Rtdb = &RT.Route_db{}
			RT.Rtdb.RT_db_init()
			RT.MyACPrefix = acprefix
			RT.Service_Cutoff_Percent = uint(cutoff_percent)
			RT.Service_Deny_Percent = uint(deny_percent)
			RT.RT_Cache_TTL = rt_cache_ttl
			RT.RT_Cache_Size = int64(rt_cache_size)
		}

		{
			//init dns workers
			var name, secret string
			for i := 0; i < num_cpus; i++ {
				go D.Working("udp", port, name, secret, i, IP.Ipdb, RT.Rtdb)
			}
		}

		{
			//init amqp synchronize routine
			go A.Synchronize(interval, keepalive, myname)
		}

		sig := make(chan os.Signal)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		s := <-sig
		log.Printf("%s stopping - signal (%s) received", myname, s)
	}
}

//waiting for signal, reload child if neccessary
func signal_loop(child *os.Process) {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		s := <-sig
		switch s {

		case syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM:
			os.Exit(0)

		case syscall.SIGHUP:
			child.Signal(syscall.SIGTERM)
		}
	}
}

//wait the child process to end, handle it
func guard_child(child *os.Process) {

	for {
		child.Wait()

		env := os.Environ()
		attr := &os.ProcAttr{
			Env: env,
			Files: []*os.File{
				os.Stdin,
				os.Stdout,
				os.Stderr,
			},
		}

		child, _ = os.StartProcess("3grid", []string{os.Args[0], "-worker=1"}, attr)
	}
}
