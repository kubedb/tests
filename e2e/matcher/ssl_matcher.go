/*
Copyright The KubeDB Authors.
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
package matcher

import (
	"fmt"
	"strings"

	"github.com/onsi/gomega/types"
)

func HaveSSL(config string) types.GomegaMatcher {
	return &sslMatcher{
		expected: config,
	}
}

type sslMatcher struct {
	expected string
}

func (matcher *sslMatcher) Match(actual interface{}) (success bool, err error) {
	results := actual.([]map[string][]byte)
	sslconfigVarPair := strings.Split(matcher.expected, "=")

	var variableName, variableValue []byte
	for _, rs := range results {
		val, ok := rs["Variable_name"]
		if ok {
			variableName = val
		}
		val, ok = rs["Value"]
		if ok {
			variableValue = val
		}
	}

	if string(variableName) == sslconfigVarPair[0] && string(variableValue) == sslconfigVarPair[1] {
		return true, nil
	}
	return false, nil
}

func (matcher *sslMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %v to be equivalent to %v", actual, matcher.expected)
}

func (matcher *sslMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %v not to be equivalent to %v", actual, matcher.expected)
}
