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
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var debug = flag.Bool("debug", true, "output debug info")
var worker = flag.Bool("worker", false, "worker mode, no daemon mode")

//main program related
var num_cpus int
var port string
var daemond bool
var debug_info string
var workdir string
var progname string

//master/worker related
var master bool
var num_childs int
var childs []*os.Process
var child_lock *sync.RWMutex
var show_workpath bool

//gslb related
var mydomain string
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
var state_recv bool
var dn_spliter string

func read_conf() {
	viper.SupportedExts = append(viper.SupportedExts, "conf")
	viper.SetConfigName("grid")
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
		_num_childs := viper.GetInt("server.childs")
		if _num_childs == 0 {
			num_childs = 1
		} else {
			num_childs = _num_childs
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
		_show_workpath := viper.GetBool("server.show_workpath")
		if _show_workpath == false {
			show_workpath = false
		} else {
			show_workpath = true
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
		_mydomain := viper.GetString("gslb.mydomain")
		if _mydomain == "" {
			mydomain = "mmycdn.com"
		} else {
			mydomain = _mydomain
		}
		_dn_spliter := viper.GetString("gslb.domain_spliter")
		if _dn_spliter == "" {
			dn_spliter = "#.#"
		} else {
			dn_spliter = _dn_spliter
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
		_state_recv := viper.GetBool("gslb.state_recv")
		if _state_recv == false {
			state_recv = false
		} else {
			state_recv = true
		}
		if *debug {
			debug_info = fmt.Sprintf("%s running - cpus:%d port:%s daemon:%t debug:%t interval:%d keepalive:%d myname:%s", myname, num_cpus, port, daemond, *debug, interval, keepalive, myname)
		}
	}
}

func main() {
	var err error

	workdir, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	progname = filepath.Base(os.Args[0])

	flag.Parse()
	read_conf()

	runtime.GOMAXPROCS(num_cpus)

	G.Debug = *debug

	if len(os.Args) > 1 && os.Args[1] != "" {
		if s := os.Args[1]; strings.Contains(s, "worker:") {
			*worker = true
		}
	}

	if daemond && !*worker {
		context := daemon.Context{}

		//child's os.Args[0]==progname, os.Args[1]=="master"
		context.Args = append(context.Args, progname)
		if master {
			context.Args = append(context.Args, "master")
			if show_workpath {
				context.Args = append(context.Args, "@"+workdir)
			}
		}

		//chdir to make sure os.StartProcess can start
		os.Chdir(workdir)

		child, _ := context.Reborn()

		if child != nil {
			//daemonize myself
			os.Exit(0)
		} else {
			defer context.Release()
		}
	}

	if master && !*worker {
		//I am master, go fork worker and enter signal loop
		childs = make([]*os.Process, num_childs)
		child_lock = new(sync.RWMutex)

		for ci, chd := range childs {
			go guard_child(ci, chd)
		}

		signal_loop()

	} else {
		//after fork as worker, go on working
		{
			//multi workers
			if *worker && len(os.Args) > 1 {
				myname = myname + os.Args[1][strings.LastIndex(os.Args[1], ":"):]
			}

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
					G.LogChan3 = &G.Logger.Chan3

					go G.Logger.Output()
					go G.Logger.Output3()
					go G.Logger.Checklogs()

					if G.Debug {
						log.SetOutput(G.Logger.Fds[G.LOG_DEBUG])
						if debug_info != "" {
							G.Outlog3(G.LOG_GSLB, "%s", debug_info)
							G.OutDebug("%s", debug_info)
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
			D.DN = mydomain
			D.DN_Spliter = dn_spliter

			//init dns workers
			var name, secret string
			for i := 0; i < num_cpus; i++ {
				go D.Working("udp", port, name, secret, i, IP.Ipdb, RT.Rtdb)
			}
		}

		{
			A.State_Recv = state_recv

			//init amqp synchronize routine
			go A.Synchronize(interval, keepalive, myname)
		}

		G.OutDebug("%s worker launched", progname)

		sig := make(chan os.Signal)

		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		s := <-sig

		G.OutDebug("%s stopping - signal (%s) received", myname, s)
	}
}

//waiting for signal, reload child if neccessary
func signal_loop() {
	sig := make(chan os.Signal)

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		s := <-sig
		switch s {
		case syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM:
			//die, let child survive
			os.Exit(0)
		case syscall.SIGHUP:
			child_lock.Lock()
			if childs != nil {
				log.Printf("%+v", childs)
				//signal childs to exit(reload)
				for _, child := range childs {
					child.Signal(syscall.SIGTERM)
				}
			}
			child_lock.Unlock()
		}
	}
}

//wait the child process to end, handle it
func guard_child(index int, child *os.Process) {
	var _child *os.Process = child

	for {
		if _child != nil {
			_child.Wait()
		}

		_child, _ = fork_process(index)

		child_lock.Lock()
		childs[index] = _child
		child_lock.Unlock()
	}
}

//start child/worker process
func fork_process(workerid int) (*os.Process, error) {
	env := os.Environ()
	attr := &os.ProcAttr{
		Env: env,
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		},
	}

	//os.Args[0]==progname, os.Args[1]=="-worker=1"
	return os.StartProcess(progname, []string{os.Args[0], "worker:" + strconv.Itoa(workerid)}, attr)
}
