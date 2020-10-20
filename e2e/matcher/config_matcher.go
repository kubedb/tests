/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

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

func UseCustomConfig(config string) types.GomegaMatcher {
	return &configMatcher{
		expected: config,
	}
}

type configMatcher struct {
	expected string
}

func (matcher *configMatcher) Match(actual interface{}) (success bool, err error) {
	results := actual.([]map[string][]byte)
	configPair := strings.Split(matcher.expected, "=")

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

	if string(variableName) == configPair[0] && string(variableValue) == configPair[1] {
		return true, nil
	}
	return false, nil
}

func (matcher *configMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %v to be equivalent to %v", actual, matcher.expected)
}

func (matcher *configMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %v not to be equivalent to %v", actual, matcher.expected)
}
