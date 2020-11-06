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

package framework

import (
	"context"
	"errors"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) EventuallyCRD() GomegaAsyncAssertion {
	return Eventually(
		func() error {
			// Check MongoDB TPR
			switch DBType {
			case string(api.ResourceKindElasticsearch):
				if _, err := f.dbClient.KubedbV1alpha2().Elasticsearches(core.NamespaceAll).List(context.TODO(), metav1.ListOptions{}); err != nil {
					return errors.New("CRD Elasticsearch is not ready")
				}
			case string(api.ResourceKindMongoDB):
				if _, err := f.dbClient.KubedbV1alpha2().MongoDBs(core.NamespaceAll).List(context.TODO(), metav1.ListOptions{}); err != nil {
					return errors.New("CRD MongoDB is not ready")
				}
			}
			return nil
		},
		WaitTimeOut,
		PullInterval,
	)
}
