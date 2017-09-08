#!/usr/bin/env python
# -*- coding: UTF-8 -*- 

import os,sys,pprint
import geoip2.database

ip = sys.argv[1]

running_path = os.path.dirname(os.path.abspath(__file__))

dbfile = running_path + '/ip.db'

try:
	dbfile = sys.argv[2]
except:
	pass

print "Querying db:", dbfile

with geoip2.database.Reader(dbfile) as reader:
	res = reader.city(ip)

pp = pprint.PrettyPrinter(indent=1, width=1)

pp.pprint(vars(res))

