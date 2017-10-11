package grid_ip

import (
	G "3grid/tools/globals"
	"github.com/oschwald/geoip2-golang"
	"log"
	"net"
	//"strconv"
	"sync"
	"time"
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
var Ip_Cache_TTL int
var Ip_Cache_Size int

type IP_db struct {
	Ipcache map[string]Ipcache_Item
	Ipdb    *geoip2.Reader
	DBLock  *sync.RWMutex //DB file lock
	Lock    *sync.RWMutex //Cache lock
	Chan    chan map[string]string
}

type Ipcache_Item struct {
	AC string
	TS int64
}

func (ip_db *IP_db) IP_db_init() {

	if ip_db.Lock != nil {
		//reinit, reloading new db file
		ip_db.DBLock.Lock()
		ip_db.Ipdb.Close()
		ip_db.Ipdb, _ = geoip2.Open(Db_file)
		ip_db.DBLock.Unlock()
	} else {
		//first init, bala bala
		ip_db.Ipcache = make(map[string]Ipcache_Item)
		ip_db.Lock = new(sync.RWMutex)
		ip_db.DBLock = new(sync.RWMutex)

		ip_db.DBLock.Lock()
		ip_db.Ipdb, _ = geoip2.Open(Db_file)
		ip_db.DBLock.Unlock()

		ip_db.Chan = make(chan map[string]string, 100)
		Chan = &ip_db.Chan

		//cache maintenance func
		go ip_db.UpdateIPCache()
	}

	if G.Debug {
		log.Printf("Loading ip db..%s", Db_file)
	}

}

func (ip_db *IP_db) GetAreaCode(ip net.IP) string {
	var (
		ips string
		ipc Ipcache_Item
	)

	ips = ip.String()
	ipc = ip_db.ReadIPCache(ips)

	if ipc.AC == "" {
		re, err := ip_db.ReadIPdb(ip)
		if err == nil {
			cn := re.City.Names["MMY"]
			if cn == "" {
				cn = re.Country.Names["en"]
			}

			/*
				ipc.AC = "AC:" + cn + "|CC:" + re.Country.IsoCode + "|Coordinates:" +
				strconv.FormatFloat(re.Location.Latitude, 'f', 4, 64) + "," +
				strconv.FormatFloat(re.Location.Longitude, 'f', 4, 64) + "|AccuracyRadius:" +
				strconv.FormatUint(uint64(re.Location.AccuracyRadius), 10)
			*/

			//update ip cache
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

	return ipc.AC
}

func (ip_db *IP_db) ReadIPdb(ip net.IP) (*geoip2.City, error) {
	ip_db.DBLock.RLock()
	city, err := ip_db.Ipdb.City(ip)
	ip_db.DBLock.RUnlock()

	return city, err
}

func (ip_db *IP_db) ReadIPCache(ips string) Ipcache_Item {
	ip_db.Lock.RLock()
	ipc := ip_db.Ipcache[ips]
	ip_db.Lock.RUnlock()

	if ipc.TS == 0 {
		//no cache item
		return Ipcache_Item{}
	} else if ipc.TS+int64(Ip_Cache_TTL) < time.Now().Unix() {
		//cache expire, delete it or just ignore it?
		//ip_db.Chan <- map[string]string{ips: ""}
		return Ipcache_Item{}
	} else {
		return ipc
	}
}

func (ip_db *IP_db) UpdateIPCache() error {
	for {
		ipm := <-ip_db.Chan
		if ipm == nil {
			if G.Debug {
				log.Printf("Exiting ip cache update loop..")
			}
			break
		}
		for k, v := range ipm {
			ip_db.Lock.Lock()
			if v == "" {
				delete(ip_db.Ipcache, k)
			} else {
				if len(ip_db.Ipcache) >= Ip_Cache_Size {
					//cache too large, pop one
					for x, _ := range ip_db.Ipcache {
						delete(ip_db.Ipcache, x)
						break
					}
				}
				ip_db.Ipcache[k] = Ipcache_Item{AC: v, TS: time.Now().Unix()}
			}
			ip_db.Lock.Unlock()
		}
	}

	return nil
}
