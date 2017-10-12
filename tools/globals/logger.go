package grid_globals

import "io"
import "log"
import "os"
import "path/filepath"

const (
	LOG_IP        = "ip"
	LOG_DNS       = "query"
	LOG_ROUTE     = "route"
	LOG_SCHEDULER = "sche"
	LOG_GSLB      = "gslb"
	LOG_DEBUG     = "debug"
	LOG_AMQP      = "amqp"
)

var Log bool
var LogBufSize int
var Logger *Grid_Logger
var LogChan *chan map[string]string

type Grid_Logger struct {
	Workdir string
	Files   map[string]string
	Fds     map[string]io.Writer
	Loggers map[string]*log.Logger
	Chan    chan map[string]string
}

func Outlog(target string, line string) {
	*LogChan <- map[string]string{target: line}
}

func NewLogger() (*Grid_Logger, error) {
	var err error
	var logto = []string{LOG_IP, LOG_DNS, LOG_ROUTE, LOG_SCHEDULER, LOG_GSLB, LOG_DEBUG, LOG_AMQP}
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

	for _, to := range logto {
		lg.Files[to] = lg.Workdir + "/logs/" + to + ".log"

		if lg.Fds[to], err = os.OpenFile(lg.Files[to],
			os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
			if Debug {
				log.Printf("Error openning log file %s: %s", err)
			}
		} else {
			lg.Loggers[to] = log.New(lg.Fds[to], "", log.LstdFlags)
		}
	}

	lg.Chan = make(chan map[string]string, LogBufSize)

	return &lg, err
}

func (lg *Grid_Logger) Output() {
	for {
		lm := <-lg.Chan
		for to, line := range lm {
			if err := lg.Loggers[to].Output(1, line); err != nil {
				log.Printf("Error output lines to %s: %s", lg.Files[to], err)
			}
		}
	}
}
