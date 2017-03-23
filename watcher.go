// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func date() string {
	t := time.Now()
	//return string(t.Format(time.UnixDate))
	return string(t.Unix())
}

func shell(arg string) string {
	bin := strings.Split(arg, " ")[0]
	args := strings.SplitAfterN(arg, " ", 2)[1:]
	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	str, _ := ioutil.ReadAll(stdout)
	errors, _ := ioutil.ReadAll(stderr)
	fmt.Println(string(errors))

	return string(str)
}
func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if !(strings.Contains(event.String(), "git") || strings.Contains(event.String(), "watcher")) {
					log.Println("event:", event)
					if event.Op&fsnotify.Write == fsnotify.Write {
						log.Println("modified file:", event.Name)
						fmt.Println(shell("git add " + event.Name))
						fmt.Println("'" + date() + "'")
						fmt.Println(shell("git commit -m " + "'" + date() + "'"))
						fmt.Println(shell("git log"))
					}
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(".")
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
