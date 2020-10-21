/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Free Trial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Free-Trial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testing

import (
	"fmt"
	"strconv"
	"strings"
)

//issues
const (
	ok                                   = "OK"
	errConnectionResetBypeer             = "Connection reset by peer"
	errUnknownSubCommandOrWrongArguments = "ERR Unknown subcommand or wrong number of arguments"
	errUnknownCommand                    = "ERR unknown command"
	errWrongNumberOfArfuments            = "ERR wrong number of arguments"
	errMoved                             = "MOVED "
	errInvalidSlots                      = "ERR Invalid slot"
	errUnknownNode                       = "ERR Unknown node"
	errClusterFailoverToReplica          = "ERR You should send CLUSTER FAILOVER to a replica"
	errNotFound                          = "not found"
	errNotMaster                         = "ERR The specified node is not a master"
	errNotSlave                          = "ERR The specified node is not a slave"
	errReadOnlySlave                     = "READONLY You can't write against a read only replica"
)

func movedError(res string) bool {

	arr := strings.Split(res, " ")
	if arr[0] == "MOVED" {
		slot, err := strconv.Atoi(arr[1])
		if err == nil && slot >= 0 && slot < 16384 {
			add := strings.Split(arr[2], ":")
			ip := add[0]
			if checkIPAddress(ip) {
				return true
			}
		}
	}
	return false

}
func checkIfResultError(res string) (string, error) {
	res = strings.TrimSuffix(res, "\n")
	if res == ok {
		return res, nil
	}
	if strings.Contains(res, errConnectionResetBypeer) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errUnknownSubCommandOrWrongArguments) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errUnknownCommand) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errWrongNumberOfArfuments) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errMoved) && movedError(res) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errInvalidSlots) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errUnknownNode) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errClusterFailoverToReplica) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errNotFound) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errNotMaster) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errNotSlave) {
		return "", fmt.Errorf(res)
	}
	if strings.Contains(res, errReadOnlySlave) {
		return "", fmt.Errorf(res)
	}
	return res, nil

}
