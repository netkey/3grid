package main

import (
	//"3grid/dns"
	"bufio"
	"os"
	"strings"
	"testing"
)

func sp_sort(sp string) []string {
	var aaa = []string{}
	var _aaa = []string{}
	var first bool = true

	sss := strings.Split(sp, ",")
	for _, s := range sss {
		if first {
			first = false
			aaa = append(aaa, s)
			continue
		}
		for i, a := range aaa {
			//sort, a<b<c...
			if s < a {
				if i == 0 {
					aaa = append([]string{s}, aaa...)
				} else {
					_aaa = append(aaa[0:i-1], s)
					aaa = append(_aaa, aaa[i:]...)
				}
			}
		}
	}

	return aaa
}

func TestGrid(t *testing.T) {
	var testfile string = "logs/slb1/test_cdn.log"
	var line string

	if file, err := os.Open(testfile); err == nil {
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line = scanner.Text()
			a := strings.Split(line, "|")
			ip := a[0]
			dn := a[1]
			aaa := sp_sort(a[2])
			t.Logf("ip:%s dn:%s aaa:%+v", ip, dn, aaa)
		}
	}

}
