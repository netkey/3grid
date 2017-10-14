package grid_globals

import "io/ioutil"
import "strconv"
import "strings"
import "sync"
import "time"

var Debug bool

var GP Perf_Counter

type Perf_Counter struct {
	Interval uint64
	qs       uint64
	qps      uint64
	load     uint64
	qslock   *sync.RWMutex
	qpslock  *sync.RWMutex
	loadlock *sync.RWMutex
	Chan     chan map[string]uint64
}

func (pc *Perf_Counter) Init(interval int) {
	pc.qs = 0
	pc.qps = 0
	pc.load = 0
	pc.Interval = uint64(interval)

	pc.qslock = new(sync.RWMutex)
	pc.qpslock = new(sync.RWMutex)
	pc.loadlock = new(sync.RWMutex)

	pc.Chan = make(chan map[string]uint64, 100)

	go pc.update_gp()
	go pc.keeper()
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
			case "QS":
				pc.Inc_Qs(v)
			case "LOAD":
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
