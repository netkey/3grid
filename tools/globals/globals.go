package grid_globals

import (
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"
)

var Debug bool //global debug flag
var Test bool  //global test flag

var GP Perf_Counter //global perf counter
var PC Perfcs       //specific perf counter

var VerLock *sync.RWMutex

const (
	PERF_DOMAIN = "domain"
)

type Perfcs struct {
	Interval uint64
	Pcs      map[string]map[string]*Perf_Counter //string for type, string for name/id
	Chan     chan map[string]map[string]uint64
	Lock     *sync.RWMutex
}

func (pcs *Perfcs) Init(interval int) {
	pcs.Interval = uint64(interval)
	pcs.Pcs = make(map[string]map[string]*Perf_Counter)
	pcs.Chan = make(chan map[string]map[string]uint64, 1000)
	pcs.Lock = new(sync.RWMutex)

	go pcs.update_pcs()
	go pcs.keeper()
}

func (pcs *Perfcs) Read_Perfcs(perf_type string) *map[string]string {
	var qps uint64

	pfcs := make(map[string]string)

	switch perf_type {
	case PERF_DOMAIN:
		pcs.Lock.RLock()
		defer pcs.Lock.RUnlock()
		for k, v := range pcs.Pcs[perf_type] {
			qps = v.Read_Qps()
			pfcs[k] = strconv.Itoa(int(qps))
		}
	}

	return &pfcs
}

func (pcs *Perfcs) update_pcs() error {
	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic pcs update_pcs: %s", pan)
		}
	}()

	for {
		if pcm := <-pcs.Chan; pcm != nil {
			for _type, _values := range pcm {
				for k, v := range _values {

					pcs.Lock.Lock()
					if pcs.Pcs[_type] == nil {
						pcs.Pcs[_type] = make(map[string]*Perf_Counter)
					}
					if pcs.Pcs[_type][k] == nil {
						_pc := &Perf_Counter{}
						_pc.Init(int(pcs.Interval), false)

						pcs.Pcs[_type][k] = _pc
					}
					pc := pcs.Pcs[_type][k]
					pcs.Lock.Unlock()

					pc.Inc_Qs(v)

				}
			}
		}
	}

	return nil
}

func (pcs *Perfcs) keeper() error {
	var qs, qps uint64

	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic pcs keeper: %s", pan)
		}
	}()

	for {
		time.Sleep(time.Duration(pcs.Interval) * time.Second)

		for _type, _v := range pcs.Pcs {
			for k, _ := range _v {

				pcs.Lock.RLock()
				pc := pcs.Pcs[_type][k]
				pcs.Lock.RUnlock()

				//update qps, reset qs counter
				qs = pc.Read_Qs()
				qps = uint64(qs / pc.Interval)

				pc.Zero_Qs()
				pc.Update_Qps(qps)

				OutDebug("Perf: dn:%s qs:%d qps:%d", k, qs, qps)

				//update load
				/*
					switch _type {
					case PERF_DOMAIN:
						//do load counter specific tasks of domain
						load := 0
						pc.Chan <- map[string]uint64{"LOAD": load}
					}
				*/
			}
		}

	}

	return nil
}

type Perf_Counter struct {
	Chan     chan map[string]uint64
	Interval uint64
	goit     bool
	qs       uint64
	qps      uint64
	load     uint64
	qslock   *sync.RWMutex
	qpslock  *sync.RWMutex
	loadlock *sync.RWMutex
}

func (pc *Perf_Counter) Init(interval int, goit bool) {
	pc.qs = 0
	pc.qps = 0
	pc.load = 0
	pc.Interval = uint64(interval)

	pc.qslock = new(sync.RWMutex)
	pc.qpslock = new(sync.RWMutex)
	pc.loadlock = new(sync.RWMutex)

	if pc.goit = goit; goit {
		pc.Chan = make(chan map[string]uint64, 1000)
		go pc.update_gp()
		go pc.keeper()
	}

}

func (pc *Perf_Counter) Read_Qs() uint64 {
	pc.qslock.RLock()
	qs := pc.qs
	pc.qslock.RUnlock()

	return qs
}

func (pc *Perf_Counter) Inc_Qs(i uint64) {
	pc.qslock.Lock()
	pc.qs = pc.qs + i
	pc.qslock.Unlock()
}

func (pc *Perf_Counter) Zero_Qs() {
	pc.qslock.Lock()
	pc.qs = 0
	pc.qslock.Unlock()
}

func (pc *Perf_Counter) Read_Qps() uint64 {
	pc.qpslock.RLock()
	qps := pc.qps
	pc.qpslock.RUnlock()

	return qps
}

func (pc *Perf_Counter) Update_Qps(qps uint64) {
	pc.qpslock.Lock()
	pc.qps = qps
	pc.qpslock.Unlock()
}

func (pc *Perf_Counter) Read_Load() uint64 {
	pc.loadlock.RLock()
	load := pc.load
	pc.loadlock.RUnlock()

	return load
}

func (pc *Perf_Counter) Update_Load(load uint64) {
	pc.loadlock.Lock()
	pc.load = load
	pc.loadlock.Unlock()
}

func (pc *Perf_Counter) update_gp() error {
	var qs, load, lc uint64

	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic pc update_gp: %s", pan)
		}
	}()

	for {
		select {
		case gpm := <-pc.Chan:
			if gpm != nil {
				for k, v := range gpm {
					switch k {
					case "QS", "qs":
						qs += v
						if qs > 100 {
							pc.Inc_Qs(qs)
							qs = 0
						}
					case "LOAD", "load":
						lc++
						load = v
						if lc > 0 {
							pc.Update_Load(load)
							load = 0
							lc = 0
						}
					}
				}
			}

		case <-time.After(time.Second * 5):
			//timeout
			if qs > 0 {
				pc.Inc_Qs(qs)
				qs = 0
			}
			if load > 0 {
				pc.Update_Load(load)
				load = 0
				lc = 0
			}
		}

	}

	return nil
}

func (pc *Perf_Counter) keeper() error {
	var qs, qps, load, idle, _idle, total, _total uint64

	defer func() {
		if pan := recover(); pan != nil {
			Outlog3(LOG_GSLB, "Panic pc keeper: %s", pan)
		}
	}()

	for {
		time.Sleep(time.Duration(pc.Interval) * time.Second)

		//update qps, reset qs counter
		qs = pc.Read_Qs()
		qps = uint64(qs / pc.Interval)

		pc.Zero_Qs()
		pc.Update_Qps(qps)

		//update load
		idle, total = getCPULoad()
		load = uint64(((total - _total) - (idle - _idle)) / (total - _total) * 100)
		pc.Chan <- map[string]uint64{"LOAD": load}
		_idle, _total = idle, total

		OutDebug("Perf: qs:%d qps:%d load:%d", qs, qps, load)

	}

	return nil
}

func getCPULoad() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

//Safety call to func with panic recovering
func Safefunc(f interface{}, args ...interface{}) {
	defer func() {
		if err := recover(); err != nil {
			Outlog3(LOG_GSLB, "Panic: %s", err)
		}
	}()

	if len(args) > 1 {
		f.(func(...interface{}))(args)
	} else if len(args) == 1 {
		f.(func(interface{}))(args[0])
	} else {
		f.(func())()
	}
}
