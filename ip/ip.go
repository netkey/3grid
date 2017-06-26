package grid_ip

import (
	"github.com/oschwald/geoip2-golang"
	"net"
	"strconv"
	"sync"
)

type IP_db struct {
	Ipcache map[string]string
	Ipdb    *geoip2.Reader
	Lock    *sync.RWMutex
}

func (ip_db *IP_db) IP_db_init() {
	ip_db.Ipcache = make(map[string]string)
	ip_db.Ipdb, _ = geoip2.Open("ip/GeoLite2-City.mmdb")
	ip_db.Lock = new(sync.RWMutex)
}

func (ip_db *IP_db) GetAreaCode(ip *net.UDPAddr) string {
	var (
		ips string
		ipc string
	)

	ips = ip.IP.String()
	ipc = ip_db.Ipcache[ip.IP.String()]

	if ipc == "" {
		re, err := ip_db.Ipdb.City(ip.IP)
		if err == nil {
			cn := re.City.Names["en"]
			if cn == "" {
				cn = re.Country.Names["en"]
			}

			ipc = cn + "|" + re.Country.IsoCode + "|" + re.Postal.Code + "|Coordinates: " +
				strconv.FormatFloat(re.Location.Latitude, 'f', 4, 64) + ", " +
				strconv.FormatFloat(re.Location.Longitude, 'f', 4, 64)

			ip_db.Lock.Lock()
			ip_db.Ipcache[ips] = ipc
			ip_db.Lock.Unlock()
		}
	}

	return ipc
}
