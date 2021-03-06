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
	"bytes"
	"errors"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
)

var Version = "0"

const (
	pVersion         = "version"
	pIndex           = "index"
	pId              = "id"
	pName            = "name"
	pWatch           = "watch"
	pAdmin           = "admin"
	pFilesPending    = "files"
	pVideoUrl        = "videourl"
	pIgnore          = "ignorecmd"
	pMeta            = "meta"
	pTags            = "tags"
	fReplace         = "replace"
	tCmdSwitchIgnore = "/switchignore"
	tCmdForceUpload  = "/forceupload"
)

var logger = logging.MustGetLogger("observer")
var nonLetterNumberSpaceRegexp = regexp.MustCompile(`(?m)[^\p{L}\p{N}_\s]`)
var nonEmptyRegexp = regexp.MustCompile("^$")
var allSpacesRegexp = regexp.MustCompile(`(?m)\s`)

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

func formatMessage(tmpl *template.Template, values map[string]interface{}) (string, error) {
	var err error
	var res string
	if tmpl != nil {
		buf := bytes.Buffer{}
		values[fReplace] = strings.ReplaceAll
		if err = tmpl.Execute(&buf, values); err == nil {
			res = buf.String()
		}
	} else {
		err = errors.New("template not inited")
	}
	return res, err
}

func downloadToDirectory(path, url, ext string) (string, error) {
	var err error
	var tmpFileName string
	var tmpFile *os.File
	if tmpFile, err = ioutil.TempFile(path, "*."+ext); err == nil {
		tmpFileName = tmpFile.Name()
		if resp, httpErr := http.Get(url); checkResponse(resp, httpErr) {
			defer resp.Body.Close()
			if _, err = io.Copy(tmpFile, resp.Body); err == nil{
				err = tmpFile.Sync()
			}
		} else {
			err = responseError(resp, httpErr)
		}
		if fErr := tmpFile.Close(); fErr != nil{
			logger.Error(err)
		}
	}
	return tmpFileName, err
}

func formatHashTags(commaSeparatedWords string) string {
	sb := strings.Builder{}
	if !isEmpty(commaSeparatedWords) {
		for _, e := range strings.Split(commaSeparatedWords, ",") {
			e = strings.TrimSpace(e)
			if !isEmpty(e) {
				sb.WriteRune('#')
				sb.WriteString(e)
				sb.WriteRune(' ')
			}
		}
	}
	return sb.String()
}
