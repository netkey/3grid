package grid_tools

import IP "3grid/ip"
import RT "3grid/route"
import G "3grid/tools/globals"
import "log"
import "os"
import "path/filepath"
import "regexp"
import "strconv"
import "strings"

var checkprefix string
var checkfilelist map[string]string

func update_version(path string, finfo os.FileInfo, err error) error {
	_regex := checkprefix + "([0-9\\.]*)\\.db"
	_regex2 := "([0-9\\.]+[0-9])"

	re := regexp.MustCompile(_regex)
	re2 := regexp.MustCompile(_regex2)

	if re.MatchString(path) {
		_file := re.FindString(path)
		_ver := re2.FindString(_file)
		checkfilelist[_ver] = path
	}

	return nil

}

func split_version(ver string) (uint64, uint64, uint64) {
	var _major, _minor, _patch uint64
	va := strings.Split(ver, ".")
	lenva := len(va)

	if lenva > 2 {
		_major, _ = strconv.ParseUint(va[0], 10, 64)
		_minor, _ = strconv.ParseUint(va[1], 10, 64)
		_patch, _ = strconv.ParseUint(va[2], 10, 64)
	} else if lenva > 1 {
		_major, _ = strconv.ParseUint(va[0], 10, 64)
		_minor, _ = strconv.ParseUint(va[1], 10, 64)
	}

	return _major, _minor, _patch
}

func check_db_version(_type string) (uint64, uint64, uint64, string, string, error) {
	var err error
	var cur_dir string
	var max_ver string

	checkprefix = _type + "_v"
	checkfilelist = make(map[string]string)

	cur_dir, err = os.Getwd()
	if err != nil {
		log.Printf("%s", err)
	}

	if err = filepath.Walk(cur_dir+"/", update_version); err != nil {
		return 0, 0, 0, "", "", err
	}

	for key := range checkfilelist {
		if key > max_ver {
			max_ver = key
		}
	}

	//log.Printf("%s version matching: %s", _type, max_ver)

	//return major minor patch whole_version file_path error
	major, minor, patch := split_version(max_ver)
	whole_version := max_ver
	file_path := checkfilelist[max_ver]

	return major, minor, patch, whole_version, file_path, nil
}

func Check_db_versions() error {
	var err error

	IP.Ver_Major, IP.Ver_Minor, IP.Ver_Patch, IP.Version, IP.Db_file, err = check_db_version("ip")

	if err != nil {
		log.Printf("Check db version error: %s", err)
	} else {
		if G.Debug {
			log.Printf("IP db version:%s, major:%d, minor:%d, patch:%d, file_path:%s", IP.Version, IP.Ver_Major, IP.Ver_Minor, IP.Ver_Patch, IP.Db_file)
		}
	}

	RT.Ver_Major, RT.Ver_Minor, RT.Ver_Patch, RT.Version, RT.Db_file, err = check_db_version("route")

	if err != nil {
		log.Printf("Check db version error: %s", err)
	} else {
		if G.Debug {
			log.Printf("Route db version:%s, major:%d, minor:%d, patch:%d, file_path:%s", RT.Version, RT.Ver_Major, RT.Ver_Minor, RT.Ver_Patch, RT.Db_file)
		}
	}

	RT.DM_Ver_Major, RT.DM_Ver_Minor, RT.DM_Ver_Patch, RT.DM_Version, RT.DM_Db_file, err = check_db_version("domain")

	if err != nil {
		log.Printf("Check db version error: %s", err)
	} else {
		if G.Debug {
			log.Printf("Domain db version:%s, major:%d, minor:%d, patch:%d, file_path:%s", RT.DM_Version, RT.DM_Ver_Major, RT.DM_Ver_Minor, RT.DM_Ver_Patch, RT.DM_Db_file)
		}
	}

	return nil
}
