package grid_globals

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	LOG_IP        = "ip"
	LOG_DNS       = "query"
	LOG_ROUTE     = "route"
	LOG_SCHEDULER = "sche"
	LOG_GSLB      = "gslb"
	LOG_DEBUG     = "debug"
	LOG_AMQP      = "amqp"
	LOG_TEST      = "test"
	LOG_API       = "api"
)

var Log bool
var LogBufSize int
var Logger *Grid_Logger
var LogChan *chan map[string]string
var LogChan3 *chan map[string][]interface{}

var Apilog *ApiLog

type ApiLog struct {
	Goid int
	Chan *chan string
	Lock *sync.RWMutex
}

type Grid_Logger struct {
	Workdir string
	Files   map[string]string
	Fds     map[string]io.Writer
	Loggers map[string]*log.Logger
	Chan    chan map[string]string
	Chan3   chan map[string][]interface{}
	Locks   map[string]*sync.RWMutex
}

func Outlog(target string, line string) {
	if Log {
		*LogChan <- map[string]string{target: line}
	} else {
		if Debug {
			log.Printf(line)
		}
	}
}

func Outlog2(lines *map[string]string) {
	if Log && lines != nil {
		*LogChan <- *lines
	}
}

func Outlog3(target string, a ...interface{}) {
	if Log {
		*LogChan3 <- map[string][]interface{}{target: a}
	} else {
		if Debug {
			log.Printf(a[0].(string), a[1:]...)
		}
	}
}

func OutDebug(a ...interface{}) {
	if Debug {
		*LogChan3 <- map[string][]interface{}{LOG_DEBUG: a}
	}
}

func OutDebug2(target string, a ...interface{}) {
	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic logger OutDebug2: %s", pan)
		}
	}()

	if Debug {
		*LogChan3 <- map[string][]interface{}{target: a}
	}
	if Apilog != nil {
		if Apilog.Chan != nil {
			if len(*Apilog.Chan) < cap(*Apilog.Chan) {
				_go_id := GoID()
				if _go_id == Apilog.Goid {
					*Apilog.Chan <- fmt.Sprintf(a[0].(string), a[1:]...)
				}
			}
		}
	}
}

func NewLogger(path *string) (*Grid_Logger, error) {
	var err error
	var logto = []string{LOG_IP, LOG_DNS, LOG_ROUTE, LOG_SCHEDULER,
		LOG_GSLB, LOG_DEBUG, LOG_AMQP, LOG_TEST, LOG_API}
	var lg = Grid_Logger{}
	var O_FLAGS int

	if path == nil {
		if lg.Workdir, err = filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
			if Debug {
				log.Printf("Error getting workdir: %s", err)
			}
			return nil, err
		}
	} else {
		lg.Workdir = *path
	}

	lg.Files = make(map[string]string)
	lg.Fds = make(map[string]io.Writer)
	lg.Loggers = make(map[string]*log.Logger)
	lg.Locks = make(map[string]*sync.RWMutex)

	//if no logs/ dir, create it
	logs_path := lg.Workdir + "/logs"
	if _, err = os.Stat(logs_path); os.IsNotExist(err) {
		os.Mkdir(logs_path, 0755)
	}

	for _, to := range logto {
		lg.Files[to] = lg.Workdir + "/logs/" + to + ".log"

		if Test {
			O_FLAGS = os.O_TRUNC | os.O_WRONLY | os.O_CREATE
		} else {
			O_FLAGS = os.O_APPEND | os.O_WRONLY | os.O_CREATE
		}

		if lg.Fds[to], err = os.OpenFile(lg.Files[to], O_FLAGS, 0644); err != nil {
			if Debug {
				log.Printf("Error open log file %s: %s", err)
			}
		} else {
			lg.Loggers[to] = log.New(lg.Fds[to], "", log.LstdFlags)
		}

		lg.Locks[to] = new(sync.RWMutex)
	}

	lg.Chan = make(chan map[string]string, LogBufSize)
	lg.Chan3 = make(chan map[string][]interface{}, LogBufSize)

	return &lg, err
}

func (lg *Grid_Logger) Output() {
	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic logger ouput: %s", pan)
		}
	}()

	for {
		lmap := <-lg.Chan
		for to, line := range lmap {
			lg.Locks[to].Lock()
			lg.Loggers[to].Output(2, line)
			lg.Locks[to].Unlock()
		}
	}
}

func (lg *Grid_Logger) Output3() {
	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic logger ouput3: %s", pan)
		}
	}()

	for {
		lmap := <-lg.Chan3
		for to, lines := range lmap {
			lg.Locks[to].Lock()
			lg.Loggers[to].Printf(lines[0].(string), lines[1:]...)
			lg.Locks[to].Unlock()
		}
	}
}

func (lg *Grid_Logger) Checklogs() {
	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic logger checklogs: %s", pan)
		}
	}()

	for {
		time.Sleep(time.Duration(15) * time.Second)
		for n, f := range lg.Files {
			if _, err := os.Stat(f); err != nil {
				if Debug {
					log.Printf("Logfile %s, reopen it", err)
				}
				lg.Locks[n].Lock()
				if lg.Fds[n], err = os.OpenFile(lg.Files[n],
					os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
					if Debug {
						log.Printf("Error reopen log file %s: %s", err)
					}
				} else {
					lg.Loggers[n] = log.New(lg.Fds[n], "", log.LstdFlags)
				}
				lg.Locks[n].Unlock()
			}
		}
	}
}
