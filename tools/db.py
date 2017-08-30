#!/usr/bin/env python
# -*- coding: UTF-8 -*- 

import os, sys, copy
import httplib, json
import gzip, StringIO

db_filename = ""
already_have = False
running_path = os.path.dirname(os.path.abspath(__file__))
writing_path = os.getcwd()

url = "oms.chinamaincloud.com:8000"

_type = "route"

try:
	_type = sys.argv[1]
except:
	pass

all_types = ["route", "ip", "cmdb", "domain"]

ver_uri = {"route": "/route/backend/version/", "ip": "/ipdb/backend/version/", "cmdb": "", "domain": ""}
uri = {"route": "/media/route/download/route.json.gz", "ip": "/media/ipdb/download/ipdb.mmdb.gz", "cmdb": "", "domain": ""}

print "running path:", running_path
print "writing path:", writing_path

if _type == "all":
	_types = copy.copy(all_types)
else:
	if _type not in all_types:
		print "unknown db type"
		sys.exit(1)
	else:
		_types = [_type]

print "db type:", _types

_conn = httplib.HTTPConnection(url, timeout=300)
if not _conn:
	print "error connecting to server"
	sys.exit(1)

for _type in _types:
	_conn.request("GET", ver_uri[_type])
	_r = _conn.getresponse()
	_resp = _r.read()

	_data = u''
	try:
		_data = json.loads(_resp)
	except:
		print "error getting version of", _type
		continue

	if _data:
		try:
			version = _data["version"]
			print _type, "db verion:", version
		except:
			print "error getting version of", _type
			continue
		db_filename = _type + "_v" + version + ".db"
		if os.path.exists(db_filename):
			already_have = True
	else:
		already_have = True

	if already_have:
		already_have = False
		print "db file already exists:", db_filename
		continue

	if db_filename:
		_conn.request("GET", uri[_type])
		_r = _conn.getresponse()
		_resp = _r.read()

		if _resp:
			print "writing db file:", db_filename

			#unzip and write the db
			_f = StringIO.StringIO()
			_f.write(_resp)
			_f.seek(0)
			fz = gzip.GzipFile(fileobj=_f, mode='rb')

			with open(writing_path + "/" + db_filename, "wb") as db_f:
				db_f.write(fz.read())
		else:
			print "error getting", _type, "db"

_conn.close()
