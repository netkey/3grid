#!/usr/bin/env python
# -*- coding: UTF-8 -*- 

import sys,pprint
import geoip2.database

ip = sys.argv[1]

with geoip2.database.Reader('../ip/GeoLite2-City.mmdb') as reader:
	res = reader.city(ip)

pp = pprint.PrettyPrinter(indent=1, width=1)

pp.pprint(vars(res))
