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
	"fmt"
	"path/filepath"
	"strings"

	"kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/elasticsearch/client/es"

	"github.com/appscode/go/crypto/rand"
	string_util "github.com/appscode/go/strings"
	"github.com/appscode/go/types"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/portforward"
)

var (
	JobPvcStorageSize = "200Mi"
)

func (fi *Invocation) getElasticsearchDataDir() string {
	return api.ElasticsearchDataDir
}

func (fi *Invocation) GetElasticsearchCommonConfig() string {
	dataPath := fi.getElasticsearchDataDir()

	commonSetting := es.Setting{
		Path: &es.PathSetting{
			Logs: filepath.Join(dataPath, "/elasticsearch/common-logdir"),
		},
	}
	data, err := yaml.Marshal(commonSetting)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

func (fi *Invocation) GetElasticsearchMasterConfig() string {
	dataPath := fi.getElasticsearchDataDir()

	masterSetting := es.Setting{
		Path: &es.PathSetting{
			Data: []string{filepath.Join(dataPath, "/elasticsearch/master-datadir")},
		},
	}
	data, err := yaml.Marshal(masterSetting)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

func (fi *Invocation) GetElasticsearchIngestConfig() string {
	dataPath := fi.getElasticsearchDataDir()
	clientSetting := es.Setting{
		Path: &es.PathSetting{
			Data: []string{filepath.Join(dataPath, "/elasticsearch/ingest-datadir")},
		},
	}
	data, err := yaml.Marshal(clientSetting)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

func (fi *Invocation) GetElasticsearchDataConfig() string {
	dataPath := fi.getElasticsearchDataDir()
	dataSetting := es.Setting{
		Path: &es.PathSetting{
			Data: []string{filepath.Join(dataPath, "/elasticsearch/data-datadir")},
		},
	}
	data, err := yaml.Marshal(dataSetting)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

func (fi *Invocation) GetElasticsearchCustomConfig() *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fi.app,
			Namespace: fi.namespace,
		},
		StringData: map[string]string{},
	}
}

func (fi *Invocation) IsElasticsearchUsingProvidedConfig(nodeInfo []es.NodeInfo) bool {
	for _, node := range nodeInfo {
		fmt.Println("node: ", node)
		if string_util.Contains(node.Roles, "master") || strings.HasSuffix(node.Name, "master") {
			masterConfig := &es.Setting{}
			err := yaml.Unmarshal([]byte(fi.GetElasticsearchMasterConfig()), masterConfig)
			Expect(err).NotTo(HaveOccurred())

			if !string_util.EqualSlice(node.Settings.Path.Data, masterConfig.Path.Data) {
				return false
			}
		}
		if (string_util.Contains(node.Roles, "ingest") &&
			!string_util.Contains(node.Roles, "master")) ||
			strings.HasSuffix(node.Name, "ingest") { // master config has higher precedence

			ingestConfig := &es.Setting{}
			err := yaml.Unmarshal([]byte(fi.GetElasticsearchIngestConfig()), ingestConfig)
			Expect(err).NotTo(HaveOccurred())

			if !string_util.EqualSlice(node.Settings.Path.Data, ingestConfig.Path.Data) {
				return false
			}
		}
		if (string_util.Contains(node.Roles, "data") &&
			!(string_util.Contains(node.Roles, "master") && !string_util.Contains(node.Roles, "ingest"))) ||
			strings.HasSuffix(node.Name, "data") { //master and ingest config has higher precedence
			dataConfig := &es.Setting{}
			err := yaml.Unmarshal([]byte(fi.GetElasticsearchDataConfig()), dataConfig)
			Expect(err).NotTo(HaveOccurred())
			if !string_util.EqualSlice(node.Settings.Path.Data, dataConfig.Path.Data) {
				return false
			}
		}

		// check for common config
		commonConfig := &es.Setting{}
		err := yaml.Unmarshal([]byte(fi.GetElasticsearchCommonConfig()), commonConfig)
		Expect(err).NotTo(HaveOccurred())
		if node.Settings.Path.Logs != commonConfig.Path.Logs {
			return false
		}
	}
	return true
}

func (fi *Invocation) StandaloneElasticsearch() *api.Elasticsearch {
	return &api.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("elasticsearch"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.ElasticsearchSpec{
			Version:  DBVersion,
			Replicas: types.Int32P(1),
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(fi.StorageClass),
			},
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (fi *Invocation) ClusterElasticsearch() *api.Elasticsearch {
	return &api.Elasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("elasticsearch"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.ElasticsearchSpec{
			Version: DBVersion,
			Topology: &api.ElasticsearchClusterTopology{
				Master: api.ElasticsearchNode{
					Replicas: types.Int32P(2),
					Prefix:   "master",
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(fi.StorageClass),
					},
				},
				Data: api.ElasticsearchNode{
					Replicas: types.Int32P(2),
					Prefix:   "data",
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(fi.StorageClass),
					},
				},
				Ingest: api.ElasticsearchNode{
					Replicas: types.Int32P(2),
					Prefix:   "ingest",
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(fi.StorageClass),
					},
				},
			},
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (f *Framework) CreateElasticsearch(obj *api.Elasticsearch) error {
	_, err := f.dbClient.KubedbV1alpha2().Elasticsearches(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) GetElasticsearch(meta metav1.ObjectMeta) (*api.Elasticsearch, error) {
	return f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchElasticsearch(meta metav1.ObjectMeta, transform func(*api.Elasticsearch) *api.Elasticsearch) (*api.Elasticsearch, error) {
	elasticsearch, err := f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	elasticsearch, _, err = util.PatchElasticsearch(context.TODO(), f.dbClient.KubedbV1alpha2(), elasticsearch, transform, metav1.PatchOptions{})
	return elasticsearch, err
}

func (f *Framework) DeleteElasticsearch(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInBackground())
}

func (f *Framework) EventuallyServices(meta metav1.ObjectMeta, fqn string) GomegaAsyncAssertion {
	return Eventually(func() error {
		labelMap := map[string]string{
			meta_util.NameLabelKey:     fqn,
			meta_util.InstanceLabelKey: meta.Name,
		}
		se := labels.SelectorFromSet(labelMap)

		svcList, err := f.kubeClient.CoreV1().Services(meta.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: se.String(),
		})
		if err != nil {
			return err
		}
		if len(svcList.Items) > 0 {
			return errors.New("Services haven't wiped out yet.")
		}

		return nil
	}, WaitTimeOut, PullInterval)

}

func (f *Framework) EventuallyElasticsearch(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			return true
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (f *Framework) EventuallyElasticsearchPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (f *Framework) EventuallyElasticsearchReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			elasticsearch, err := f.dbClient.KubedbV1alpha2().Elasticsearches(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return elasticsearch.Status.Phase == api.DatabasePhaseReady
		},
		2*WaitTimeOut,
		PullInterval,
	)
}

func (f *Framework) EventuallyElasticsearchClientReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			db, err := f.GetElasticsearch(meta)
			if err != nil {
				return false
			}
			client, tunnel, err := f.GetElasticClient(meta)
			if err != nil {
				return false
			}
			defer client.Stop()
			defer tunnel.Close()

			url := fmt.Sprintf("%v://127.0.0.1:%d", db.GetConnectionScheme(), tunnel.Local)
			if _, err := client.Ping(url); err != nil {
				return false
			}
			if db.Spec.Topology != nil {
				// cluster health status will be green only for dedicated elasicsearch
				if err := client.WaitForGreenStatus("10s"); err != nil {
					return false
				}
			} else if err := client.WaitForYellowStatus("10s"); err != nil {
				return false
			}

			return true
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (f *Framework) ElasticsearchIndicesCount(client es.ESClient) (int, error) {
	var count int
	var err error
	err = wait.PollImmediate(PullInterval, WaitTimeOut, func() (bool, error) {
		count, err = client.CountIndex()
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	return count, err
}

func (f *Framework) EventuallyElasticsearchIndicesCount(client es.ESClient) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			count, err := client.CountIndex()
			if err != nil {
				return -1
			}
			return count
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (f *Framework) CleanElasticsearch() {
	elasticsearchList, err := f.dbClient.KubedbV1alpha2().Elasticsearches(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range elasticsearchList.Items {
		if _, _, err := util.PatchElasticsearch(context.TODO(), f.dbClient.KubedbV1alpha2(), &e, func(in *api.Elasticsearch) *api.Elasticsearch {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching Elasticsearch. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha2().Elasticsearches(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Elasticsearch. Error: %v", err)
	}
}

func (f *Framework) GetElasticsearchIngestPodName(elasticsearch *api.Elasticsearch) string {
	clientName := elasticsearch.Name

	if elasticsearch.Spec.Topology != nil {
		if elasticsearch.Spec.Topology.Ingest.Prefix != "" {
			clientName = fmt.Sprintf("%v-%v", elasticsearch.Spec.Topology.Ingest.Prefix, clientName)
		} else {
			clientName = fmt.Sprintf("%v-%v", api.ElasticsearchIngestNodePrefix, clientName)
		}
	}
	return fmt.Sprintf("%v-0", clientName)
}

func (f *Framework) GetElasticClient(meta metav1.ObjectMeta) (es.ESClient, *portforward.Tunnel, error) {
	db, err := f.GetElasticsearch(meta)
	if err != nil {
		return nil, nil, err
	}
	ingestPodName := f.GetElasticsearchIngestPodName(db)

	tunnel, err := f.ForwardPort(meta, string(core.ResourcePods), ingestPodName, api.ElasticsearchRestPort)
	if err != nil {
		return nil, nil, err
	}

	url := fmt.Sprintf("%v://127.0.0.1:%d", db.GetConnectionScheme(), tunnel.Local)
	esClient, err := es.GetElasticClient(f.kubeClient, f.dbClient, db, url)
	if err != nil {
		return nil, nil, err
	}

	return esClient, tunnel, nil
}

func (f *Framework) GetAuthSecretForElasticsearch(es *api.Elasticsearch, mangedByKubeDB bool) *core.Secret {
	esVersion, err := f.dbClient.CatalogV1alpha1().ElasticsearchVersions().Get(context.TODO(), string(es.Spec.Version), metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	//mangedByKubeDB mimics a secret created and manged by kubedb and not user.
	// It should get deleted during wipeout
	adminPassword := rand.Characters(8)

	var dbObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("kubedb-%v-%v", es.Name, CustomSecretSuffix),
		Namespace: es.Namespace,
	}
	if mangedByKubeDB {
		dbObjectMeta.Labels = map[string]string{
			meta_util.ManagedByLabelKey: kubedb.GroupName,
		}
	}

	var data map[string][]byte

	if esVersion.Spec.AuthPlugin == v1alpha1.ElasticsearchAuthPluginSearchGuard || esVersion.Spec.AuthPlugin == v1alpha1.ElasticsearchAuthPluginOpenDistro {
		data = map[string][]byte{
			core.BasicAuthUsernameKey: []byte(api.ElasticsearchInternalUserAdmin),
			core.BasicAuthPasswordKey: []byte(adminPassword),
		}
	} else if esVersion.Spec.AuthPlugin == v1alpha1.ElasticsearchAuthPluginXpack {
		data = map[string][]byte{
			core.BasicAuthUsernameKey: []byte(api.ElasticsearchInternalUserElastic),
			core.BasicAuthPasswordKey: []byte(adminPassword),
		}
	}

	return &core.Secret{
		ObjectMeta: dbObjectMeta,
		Data:       data,
	}
}
