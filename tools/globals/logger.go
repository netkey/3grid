package grid_globals

import "io"
import "log"
import "os"
import "path/filepath"

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

func NewLogger() (*Grid_Logger, error) {
	var err error
	var logto = []string{"ip", "query", "route", "sche", "gslb"}
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
			lg.Loggers[to] = log.New(lg.Fds[to], "", log.Lshortfile)
		}
	}

	lg.Chan = make(chan map[string]string, LogBufSize)

	return &lg, err
}

func (lg *Grid_Logger) Out() {
	for {
		lm := <-lg.Chan
		for to, line := range lm {
			lg.Loggers[to].Printf("%s", line)
		}
	}
}
