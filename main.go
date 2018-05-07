/*
Copyright 2018, RadiantBlue Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/venicegeo/vzutil-urlcheck/nt"
)

var basic = nt.NewHeaderBuilder().GetHeader()
var gitlab = nt.NewHeaderBuilder().GetHeader()
var locationRE = regexp.MustCompile(`^(https:\/\/git(?:(?:hub)|(?:lab))\.com\/[^\/]+\/[^\/]+)\/blob\/([^\/]+)\/(.+)$`)
var split = regexp.MustCompile(`,| |\n|(?:\.$)|(?:\. )`)

func main() {
	gitLocation := os.Getenv("CSV_LOCATION")
	if gitLocation == "" {
		panic("Not git location specified")
	}
	submatches := locationRE.FindStringSubmatch(gitLocation)
	if len(submatches) != 4 {
		panic("Cannot work with this location")
	}
	if dat, err := exec.Command("git", "clone", submatches[1], "work").Output(); err != nil {
		panic(err.Error() + " " + string(dat))
	}
	defer exec.Command("rm", "-rf", "work").Run()
	if dat, err := exec.Command("git", "-C", "work", "checkout", submatches[2]).Output(); err != nil {
		panic(err.Error() + " " + string(dat))
	}
	dat, err := ioutil.ReadFile("work/" + submatches[3])
	if err != nil {
		panic(err)
	}
	records, err := csv.NewReader(bytes.NewReader(dat)).ReadAll()
	if err != nil {
		panic(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(len(records))
	for _, r := range records {
		go func(row []string) {
			for _, item := range row {
				if item == "" || !strings.Contains(item, "http") {
					continue
				}
				parts := split.Split(item, -1)
				for _, part := range parts {
					if part == "" {
						continue
					}
					if !strings.HasPrefix(part, "http") {
						continue
					}
					header := basic
					if strings.Contains(part, "gitlab") {
						header = gitlab
					}
					code, _, _, err := nt.HTTP(nt.GET, part, header, nil)
					if err != nil || code != 200 {
						fmt.Printf("FAILED %s Code: [%d] Error: [%s]\n", part, code, err)
					} else {
						fmt.Println("PASSED", part)
					}
				}
			}
			wg.Done()
		}(r)
	}
	wg.Wait()
}
