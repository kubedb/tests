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
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	test_util "kubedb.dev/tests/e2e/redis/testing"

	"github.com/appscode/go/crypto/rand"
	cm "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	_ "gocloud.dev/blob/memblob"
	"gomodules.xyz/blobfs"
	"gomodules.xyz/cert/certstore"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	ka "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	core_util "kmodules.xyz/client-go/core/v1"
	appcat_cs "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1"
	scs "stash.appscode.dev/apimachinery/client/clientset/versioned"
)

const (
	Timeout       = 20 * time.Minute
	RetryInterval = 5 * time.Second
)

var (
	DockerRegistry   = "kubedbci"
	DBType           = api.ResourceSingularMariaDB
	TestProfiles     stringSlice
	DBVersion        = "10.5.8"
	OldDBVersion     = "10.4.17"
	DBUpdatedVersion = "6.0.6"
	PullInterval     = time.Second * 2
	WaitTimeOut      = time.Minute * 5
	RootFramework    *Framework
	SSLEnabled       bool
	InMemory         bool
	TestFailed       = false
)

type Framework struct {
	restConfig        *rest.Config
	kubeClient        kubernetes.Interface
	apiExtKubeClient  crd_cs.ApiextensionsV1beta1Interface
	dbClient          cs.Interface
	kaClient          ka.Interface
	dmClient          dynamic.Interface
	appCatalogClient  appcat_cs.AppcatalogV1alpha1Interface
	StashClient       scs.Interface
	topology          *core_util.Topology
	namespace         string
	name              string
	StorageClass      string
	CertStore         *certstore.CertStore
	certManagerClient cm.Interface

	// for Redis test
	testConfig *test_util.TestConfig
}

func New(
	restConfig *rest.Config,
	kubeClient kubernetes.Interface,
	apiExtKubeClient crd_cs.ApiextensionsV1beta1Interface,
	dbClient cs.Interface,
	kaClient ka.Interface,
	dmClient dynamic.Interface,
	appCatalogClient appcat_cs.AppcatalogV1alpha1Interface,
	stashClient scs.Interface,
	storageClass string,
	certManagerClient cm.Interface,
) (*Framework, error) {
	topology, err := core_util.DetectTopology(context.TODO(), metadata.NewForConfigOrDie(restConfig))
	if err != nil {
		return nil, err
	}
	store, err := certstore.New(blobfs.NewInMemoryFS(), filepath.Join("", "pki"))
	if err != nil {
		return nil, err
	}

	err = store.InitCA()
	if err != nil {
		return nil, err
	}

	testConfig := &test_util.TestConfig{
		RestConfig:    restConfig,
		KubeClient:    kubeClient,
		DBCatalogName: DBVersion,
		UseTLS:        SSLEnabled,
	}

	return &Framework{
		testConfig:        testConfig,
		restConfig:        restConfig,
		kubeClient:        kubeClient,
		apiExtKubeClient:  apiExtKubeClient,
		dbClient:          dbClient,
		kaClient:          kaClient,
		dmClient:          dmClient,
		appCatalogClient:  appCatalogClient,
		StashClient:       stashClient,
		name:              fmt.Sprintf("%s-operator", DBType),
		namespace:         rand.WithUniqSuffix(strings.ToLower(DBType)),
		StorageClass:      storageClass,
		topology:          topology,
		CertStore:         store,
		certManagerClient: certManagerClient,
	}, nil
}

func NewInvocation() *Invocation {
	return RootFramework.Invoke()
}

func (f *Framework) Invoke() *Invocation {
	return &Invocation{
		Framework:     f,
		app:           rand.WithUniqSuffix(strings.ToLower(fmt.Sprintf("%s-e2e", DBType))),
		testResources: make([]interface{}, 0),
	}
}

func (fi *Invocation) DBClient() cs.Interface {
	return fi.dbClient
}

func (fi *Invocation) KubeClient() kubernetes.Interface {
	return fi.kubeClient
}

func (fi *Invocation) App() string {
	return fi.app
}

type Invocation struct {
	*Framework
	app           string
	testResources []interface{}
}

func (fi *Invocation) TestConfig() *test_util.TestConfig {
	return fi.testConfig
}

func (fi *Invocation) RestConfig() *rest.Config {
	return fi.restConfig
}
