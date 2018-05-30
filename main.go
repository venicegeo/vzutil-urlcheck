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
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/tealeg/xlsx"
	"github.com/venicegeo/vzutil-urlcheck/nt"
)

var creds = map[string][2]string{}
var basic = nt.NewHeaderBuilder().GetHeader()
var split = regexp.MustCompile(`,| |\n|(?:\.$)|(?:\. )`)

func main() {
	for _, s := range os.Environ() {
		if !strings.HasPrefix(s, "VZCRED_") {
			continue
		}
		env := strings.SplitN(s, "=", 2)
		if len(env) != 2 {
			log.Fatalln("Credential could not be parsed correctly")
		}
		parts := strings.SplitN(env[1], " ", 3)
		if len(parts) != 3 {
			log.Fatalln("Credential could not be parsed correctly")
		}
		creds[parts[0]] = [2]string{parts[1], parts[2]}
	}
	gitLocation := os.Getenv("CSV_LOCATION")
	if gitLocation == "" {
		log.Fatalln("No git location specified")
	} else if !strings.HasSuffix(gitLocation, ".csv") && !strings.HasSuffix(gitLocation, ".xlsx") {
		log.Fatalln("The file format cannot be processed")
	}

	if _, err := os.Stat("work"); os.IsNotExist(err) {
		gitRepo := os.Getenv("CSV_REPO")
		if gitRepo == "" {
			log.Fatalln("No git repo specified")
		}
		if dat, err := exec.Command("git", "clone", gitRepo, "work").Output(); err != nil {
			log.Fatalln(err.Error() + " " + string(dat))
		}
	}
	defer exec.Command("rm", "-rf", "work").Run()

	var sheets [][][]string
	var err error
	if strings.HasSuffix(gitLocation, ".csv") {
		dat, e := ioutil.ReadFile("work/" + gitLocation)
		if e != nil {
			err = e
		} else {
			records, e := csv.NewReader(bytes.NewReader(dat)).ReadAll()
			sheets = [][][]string{records}
			err = e
		}
	} else {
		file, e := xlsx.OpenFile("work/" + gitLocation)
		if e != nil {
			err = e
		} else {
			sheets, err = file.ToSlice()
		}
	}

	if err != nil {
		log.Fatalln(err)
	}
	wgSheets := sync.WaitGroup{}
	wgSheets.Add(len(sheets))
	for _, s := range sheets {
		go func(records [][]string) {
			wgRecords := sync.WaitGroup{}
			wgRecords.Add(len(records))
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
							if !strings.HasPrefix(part, "https") {
								log.Println("[WARNING] Will not run against:", part)
								continue
							}
							header := basic
							for k, v := range creds {
								if strings.HasPrefix(part, k) {
									header = nt.NewHeaderBuilder().AddBasicAuth(v[0], v[1]).GetHeader()
									break
								}
							}
							code, _, _, err := nt.HTTP(nt.GET, part, header, nil)
							if err != nil {
								fmt.Printf("FAILED %s Code: [%d] Error: [%s]\n", part, code, err.Error())
							} else if code != 200 && (code > 304 || code < 299) {
								fmt.Printf("FAILED %s Code: [%d] Error: []\n", part, code)
							} else {
								fmt.Printf("PASSED %s Code: [%d]\n", part, code)
							}
						}
					}
					wgRecords.Done()
				}(r)
			}
			wgRecords.Wait()
			wgSheets.Done()
		}(s)
	}
	wgSheets.Wait()
}
