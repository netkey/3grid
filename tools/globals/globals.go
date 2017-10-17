package grid_globals

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"
)

var Debug bool //global debug flag

var GP Perf_Counter //global perf counter
var PC Perfcs       //specific perf counter

const (
	PERF_DOMAIN = "domain"
)

type Perfcs struct {
	Interval uint64
	Pcs      map[string]map[string]*Perf_Counter //string for type, string for name/id
	Chan     chan map[string]map[string]uint64
}

func (pcs *Perfcs) Init(interval int) {
	pcs.Interval = uint64(interval)
	pcs.Pcs = make(map[string]map[string]*Perf_Counter)
	pcs.Chan = make(chan map[string]map[string]uint64, 1000)

	go pcs.update_pcs()
	go pcs.keeper()
}

func (pcs *Perfcs) Read_Perfcs(perf_type string) *map[string]string {
	var qps uint64

	pfcs := make(map[string]string)

	switch perf_type {
	case PERF_DOMAIN:
		for k, v := range pcs.Pcs[perf_type] {
			qps = v.Read_Qps()
			pfcs[k] = strconv.Itoa(int(qps))
		}
	}

	return &pfcs
}

func (pcs *Perfcs) update_pcs() error {
	for {
		pcm := <-pcs.Chan
		for _type, _values := range pcm {
			for k, v := range _values {
				if pcs.Pcs[_type] == nil {
					pcs.Pcs[_type] = make(map[string]*Perf_Counter)
				}
				if pcs.Pcs[_type][k] == nil {
					_pc := &Perf_Counter{}
					_pc.Init(int(pcs.Interval), false)

					pcs.Pcs[_type][k] = _pc
				}
				pc := pcs.Pcs[_type][k]
				pc.Inc_Qs(v)
			}
		}
	}

	return nil
}

func (pcs *Perfcs) keeper() error {
	var qs, qps uint64

	for {
		time.Sleep(time.Duration(pcs.Interval) * time.Second)

		for _type, _v := range pcs.Pcs {
			for k, _ := range _v {
				pc := pcs.Pcs[_type][k]

				//update qps, reset qs counter
				qs = pc.Read_Qs()
				qps = uint64(qs / pc.Interval)

				pc.Zero_Qs()
				pc.Update_Qps(qps)

				if Debug {
					Outlog(LOG_DEBUG, fmt.Sprintf("Perf: dn:%s qs:%d qps:%d", k, qs, qps))
				}
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
	for {
		gpm := <-pc.Chan
		for k, v := range gpm {
			switch k {
			case "QS", "qs":
				pc.Inc_Qs(v)
			case "LOAD", "load":
				pc.Update_Load(v)
			}
		}
	}

	return nil
}

func (pc *Perf_Counter) keeper() error {
	var qs, qps, load, idle, _idle, total, _total uint64

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

		if Debug {
			Outlog(LOG_DEBUG, fmt.Sprintf("Perf: qs:%d qps:%d load:%d", qs, qps, load))
		}

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
