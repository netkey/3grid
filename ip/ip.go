package grid_ip

import (
	G "3grid/tools/globals"
	"github.com/oschwald/geoip2-golang"
	"log"
	"net"
	//"strconv"
	"sync"
)

//ip db version and file path
var Version string
var Db_file0, Db_file string
var Ver_Major, Ver_Minor, Ver_Patch uint64

//use for ip cache update
var Chan *chan map[string]string

/*IP Cache update method

ip cache update record : map[string]string

ip_address(key) : area_code(value)
"1.1.1.1" : "*.CN.HAD.SH"

update:
IP.Chan <- map[string]string{ip: ac}

*/

var Ipdb *IP_db

type IP_db struct {
	Ipcache map[string]string
	Ipdb    *geoip2.Reader
	Lock    *sync.RWMutex
	Chan    chan map[string]string
}

func (ip_db *IP_db) IP_db_init() {
	ip_db.Ipcache = make(map[string]string)
	ip_db.Ipdb, _ = geoip2.Open(Db_file)

	ip_db.Lock = new(sync.RWMutex)
	ip_db.Chan = make(chan map[string]string, 100)
	Chan = &ip_db.Chan

	if G.Debug {
		log.Printf("Loading ip db..%s", Db_file)
	}

	go ip_db.UpdateIPCache()
}

func (ip_db *IP_db) GetAreaCode(ip net.IP) string {
	var (
		ips string
		ipc string
	)

	ips = ip.String()
	ipc = ip_db.ReadIPCache(ips)

	if ipc == "" {
		re, err := ip_db.Ipdb.City(ip)
		if err == nil {
			cn := re.City.Names["MMY"]
			if cn == "" {
				cn = re.Country.Names["en"]
			}

			/*
				ipc = "AC:" + cn + "|CC:" + re.Country.IsoCode + "|Coordinates:" +
					strconv.FormatFloat(re.Location.Latitude, 'f', 4, 64) + "," +
					strconv.FormatFloat(re.Location.Longitude, 'f', 4, 64) + "|AccuracyRadius:" +
					strconv.FormatUint(uint64(re.Location.AccuracyRadius), 10)
			*/
			ip_db.Chan <- map[string]string{ips: cn}

			if G.Debug {
				log.Printf("Area Code of %s: %s", ips, cn)
			}
		} else {
			if G.Debug {
				log.Printf("IP lookup error: %s", err)
			}
		}
	}

	return ipc
}

func (ip_db *IP_db) ReadIPCache(ips string) string {
	ip_db.Lock.RLock()
	ipc := ip_db.Ipcache[ips]
	ip_db.Lock.RUnlock()

	return ipc
}

func (ip_db *IP_db) UpdateIPCache() error {
	for {
		ipm := <-ip_db.Chan
		for k, v := range ipm {
			ip_db.Lock.Lock()
			ip_db.Ipcache[k] = v
			ip_db.Lock.Unlock()
		}
	}

	return nil
}
