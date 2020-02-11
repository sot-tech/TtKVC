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

package intl

import (
	"errors"
	"github.com/op/go-logging"
	"net/http"
)

var Version = "0"

const (
	msgIndex    = "${index}"
	msgErrorMsg = "${msg}"
	msgId       = "${id}"
	msgName     = "${name}"
	msgFiles    = "${files}"
	msgVersion  = "${version}"
)

type CommandResponse struct {
	Start        string `json:"start"`
	Attach       string `json:"attach"`
	Detach       string `json:"detach"`
	State        string `json:"state"`
	Unauthorized string `json:"auth"`
	Unknown      string `json:"unknown"`
}

type Message struct {
	Commands CommandResponse `json:"cmds"`
	Announce string          `json:"announce"`
	Error    string          `json:"error"`
}

var Logger = logging.MustGetLogger("observer")
var Messages = Message{}

func checkResponse(resp *http.Response, httpErr error) bool {
	return httpErr == nil && resp != nil && resp.StatusCode < 400
}

func responseError(resp *http.Response, httpErr error) error {
	var err error
	if httpErr != nil {
		err = httpErr
	} else {
		errMsg := "kaltura: "
		if resp == nil {
			errMsg += "empty response"
		} else {
			errMsg += resp.Status
		}
		err = errors.New(errMsg)
	}
	return err
}
