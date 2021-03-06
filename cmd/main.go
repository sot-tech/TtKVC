/*
 * BSD-3-Clause
 * Copyright 2020 sot (PR_713, C_rho_272)
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 * this list of conditions and the following disclaimer in the documentation and/or
 * other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names of its contributors
 * may be used to endorse or promote products derived from this software without
 * specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA,
 * OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 */

package main

import (
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"os"
	"os/signal"
	"sot-te.ch/TtKVC"
	"syscall"
)

func main() {
	var confFile string
	flag.StringVar(&confFile, "c", "conf/ttkvc.json", "configuration file")
	flag.Parse()
	logger := logging.MustGetLogger("main")
	if len(confFile) == 0 {
		fmt.Println("Configuration not set")
		os.Exit(1)
	}
	crawler, err := TtKVC.ReadConfig(confFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var outputWriter io.Writer
	if crawler.Log.File == "" {
		outputWriter = os.Stdout
	} else {
		outputWriter, err = os.OpenFile(crawler.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	}
	if err == nil {
		backend := logging.AddModuleLevel(
			logging.NewBackendFormatter(
				logging.NewLogBackend(outputWriter, "", 0),
				logging.MustStringFormatter("%{time:2006-01-02 15:04:05.000}\t%{shortfile}\t%{shortfunc}\t%{level}:\t%{message}")))
		var level logging.Level
		if level, err = logging.LogLevel(crawler.Log.Level); err != nil {
			println(err)
			level = logging.INFO
		}
		backend.SetLevel(level, "")
		logging.SetBackend(backend)
	} else {
		println(err)
	}
	logger.Info("Starting TtKVC", TtKVC.Version)
	if err := crawler.Init(); err == nil {
		go crawler.Engage()
		ch := make(chan os.Signal, 2)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
	} else {
		logger.Fatal(err)
		os.Exit(1)
	}
}
