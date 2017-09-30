package grid_globals

import "sync"
import "time"

var Debug bool

var GP GSLB_Params

type GSLB_Params struct {
	gp_interval uint64
	qs          uint64
	qps         uint64
	load        uint64
	qslock      *sync.RWMutex
	qpslock     *sync.RWMutex
	loadlock    *sync.RWMutex
	Chan        chan map[string]uint64
}

func (gp *GSLB_Params) Init() {
	gp.gp_interval = 100
	gp.qs = 0
	gp.qps = 0
	gp.load = 0
	gp.qslock = new(sync.RWMutex)
	gp.qpslock = new(sync.RWMutex)
	gp.loadlock = new(sync.RWMutex)
	gp.Chan = make(chan map[string]uint64, 100)

	go gp.update_gp()
	go gp.keeper()
}

func (gp *GSLB_Params) Read_Qs() uint64 {
	gp.qslock.RLock()
	qs := gp.qs
	gp.qslock.RUnlock()

	return qs
}

func (gp *GSLB_Params) Inc_Qs(i uint64) {
	gp.qslock.Lock()
	gp.qs = gp.qs + i
	gp.qslock.Unlock()
}

func (gp *GSLB_Params) Zero_Qs() {
	gp.qslock.Lock()
	gp.qs = 0
	gp.qslock.Unlock()
}

func (gp *GSLB_Params) Read_Qps() uint64 {
	gp.qpslock.RLock()
	qps := gp.qps
	gp.qpslock.RUnlock()

	return qps
}

func (gp *GSLB_Params) Update_Qps(qps uint64) {
	gp.qpslock.Lock()
	gp.qps = qps
	gp.qpslock.Unlock()
}

func (gp *GSLB_Params) Read_Load() uint64 {
	gp.loadlock.RLock()
	load := gp.load
	gp.loadlock.RUnlock()

	return load
}

func (gp *GSLB_Params) Update_Load(load uint64) {
	gp.loadlock.Lock()
	gp.load = load
	gp.loadlock.Unlock()
}

func (gp *GSLB_Params) update_gp() error {
	for {
		gpm := <-gp.Chan
		for k, v := range gpm {
			switch k {
			case "QS":
				gp.Inc_Qs(v)
			case "LOAD":
				gp.Update_Load(v)
			}
		}
	}

	return nil
}

func (gp *GSLB_Params) keeper() error {
	for {
		time.Sleep(time.Duration(gp.gp_interval) * time.Second)

		//update qps, reset qs counter
		qs := gp.Read_Qs()
		qps := qs / gp.gp_interval

		gp.Zero_Qs()
		gp.Update_Qps(qps)
	}

	return nil
}
