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
	dat, err := ioutil.ReadFile("work/" + gitLocation)
	if err != nil {
		log.Fatalln(err)
	}
	records, err := csv.NewReader(bytes.NewReader(dat)).ReadAll()
	if err != nil {
		log.Fatalln(err)
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
					if err != nil || code != 200 {
						fmt.Printf("FAILED %s Code: [%d] Error: [%#v]\n", part, code, err)
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
