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

package TtKVC

import (
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var Version = "0"

const (
	pVersion         = "${version}"
	pIndex           = "${index}"
	pId              = "${id}"
	pName            = "${name}"
	pWatch           = "${watch}"
	pAdmin           = "${admin}"
	pFilesPending    = "${files}"
	pVideoUrl        = "${videourl}"
	pIgnore          = "${ignorecmd}"
	tCmdSwitchIgnore = "/switchignore"
)

var logger = logging.MustGetLogger("observer")

func isEmpty(s string) bool {
	return len(s) == 0
}

func checkResponse(resp *http.Response, httpErr error) bool {
	return httpErr == nil && resp != nil && resp.StatusCode < 400
}

func responseError(resp *http.Response, httpErr error) error {
	var err error
	if httpErr != nil {
		err = httpErr
	} else {
		errMsg := "http: "
		if resp == nil {
			errMsg += "empty response"
		} else {
			errMsg += resp.Status
		}
		err = errors.New(errMsg)
	}
	return err
}

func formatMessage(template string, values map[string]interface{}) string {
	if values != nil {
		kv := make([]string, 0, len(values)*2)
		for k, v := range values {
			kv = append(kv, k, fmt.Sprint(v))
		}
		template = strings.NewReplacer(kv...).Replace(template)
	}
	return template
}

func downloadToDirectory(path, url, ext string) (string, error) {
	var err error
	var tmpFileName string
	var tmpFile *os.File
	if tmpFile, err = ioutil.TempFile(path, "*."+ext); err == nil {
		tmpFileName = tmpFile.Name()
		defer tmpFile.Close()
		if resp, httpErr := http.Get(url); checkResponse(resp, httpErr) {
			defer resp.Body.Close()
			_, err = io.Copy(tmpFile, resp.Body)
		} else {
			err = responseError(resp, httpErr)
		}
	}
	return tmpFileName, err
}
