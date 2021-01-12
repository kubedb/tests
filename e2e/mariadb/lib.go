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

package mariadb

import (
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"
)

func runTestCommunity(testProfile string) bool {
	return strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.All ||
		framework.TestProfiles.String() == framework.Community
}

func runTestEnterprise(testProfile string) bool {
	return strings.Contains(framework.TestProfiles.String(), testProfile) ||
		framework.TestProfiles.String() == framework.All ||
		framework.TestProfiles.String() == framework.Enterprise
}

func runTestDatabaseType() bool {
	return strings.Compare(framework.DBType, api.ResourceSingularMariaDB) == 0
}
