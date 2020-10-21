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

package matcher

import (
	"fmt"
	"reflect"

	"github.com/onsi/gomega/types"
	core "k8s.io/api/core/v1"
)

func BeSameAs(resources core.ResourceRequirements) types.GomegaMatcher {
	return &resourcesMatcher{
		resources: resources,
	}
}

type resourcesMatcher struct {
	resources core.ResourceRequirements
}

func (matcher *resourcesMatcher) Match(actual interface{}) (success bool, err error) {
	updatedResources := actual.(core.ResourceRequirements)
	if !reflect.DeepEqual(matcher.resources, updatedResources) {
		return false, nil
	}
	return true, nil
}

func (matcher *resourcesMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\tUpdated resources: %v\n to  be same as requested resources:  %v\n\t", actual, matcher.resources)
}

func (matcher *resourcesMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\tUpdated resources: %v\n not to be same as requested resources:  %v\n\t", actual, matcher.resources)
}
