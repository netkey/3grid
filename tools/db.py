#!/usr/bin/env python
# -*- coding: UTF-8 -*- 

import os, sys
import httplib, json
import gzip, StringIO

db_filename = ""
already_have = False

url = "oms.chinamaincloud.com:8000"

_type = "route"

try:
	_type = sys.argv[1]
except:
	pass

print "db type:", _type

if _type == "route":
	ver_uri = "/route/backend/version/"
	uri = "/media/route/download/route.csv"
elif _type == "ip":
	ver_uri = "/ip/backend/version/"
	uri = "/media/ip/download/ip.csv"
elif _type == "cmdb":
	pass
elif _type == "control":
	pass
elif _type == "domain":
	pass
else:
	print "unknown db type"
	sys.exit(1)

_conn = httplib.HTTPConnection(url)
if not _conn:
	print "error connecting to server"
	sys.exit(1)

_conn.request("GET", ver_uri)
_r = _conn.getresponse()
_resp = _r.read()

_data = u''
_data = json.loads(_resp)

if _data:
	version = _data["version"]
	print "db verion:", version
	db_filename = "route_v" + version + ".db"
	if os.path.exists(db_filename):
		already_have = True
else:
	already_have = True

if already_have:
	print "db file already exists:", db_filename
	_conn.close()
	sys.exit(0)

if db_filename:
	_conn.request("GET", uri)
	_r = _conn.getresponse()
	_resp = _r.read()

	if _resp:
		print "writing db file:", db_filename
		#unzip and write the route db
		_f = StringIO.StringIO()
		_f.write(_resp)
		_f.seek(0)
		fz = gzip.GzipFile(fileobj=_f, mode='rb')

		with open(db_filename, "wb") as db_f:
			db_f.write(fz.read())
	else:
		print "error getting db"

_conn.close()
