package grid_globals

import "io"
import "log"
import "os"
import "path/filepath"
import "sync"
import "time"

const (
	LOG_IP        = "ip"
	LOG_DNS       = "query"
	LOG_ROUTE     = "route"
	LOG_SCHEDULER = "sche"
	LOG_GSLB      = "gslb"
	LOG_DEBUG     = "debug"
	LOG_AMQP      = "amqp"
	LOG_TEST      = "test"
)

var Log bool
var LogBufSize int
var Logger *Grid_Logger
var LogChan *chan map[string]string
var LogChan3 *chan map[string][]interface{}

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

func NewLogger() (*Grid_Logger, error) {
	var err error
	var logto = []string{LOG_IP, LOG_DNS, LOG_ROUTE, LOG_SCHEDULER, LOG_GSLB, LOG_DEBUG, LOG_AMQP, LOG_TEST}
	var lg = Grid_Logger{}

	if lg.Workdir, err = filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
		if Debug {
			log.Printf("Error getting workdir: %s", err)
		}
		return nil, err
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

		if lg.Fds[to], err = os.OpenFile(lg.Files[to],
			os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
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
