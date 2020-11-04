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

package es

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"

	esv7 "github.com/olivere/elastic/v7"
	esv5 "gopkg.in/olivere/elastic.v5"
	esv6 "gopkg.in/olivere/elastic.v6"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ESClient interface {
	CreateIndex(count int) error
	CountIndex() (int, error)
	GetIndexNames() ([]string, error)
	GetAllNodesInfo() ([]NodeInfo, error)
	Stop()
	Ping(string) (int, error)
	WaitForGreenStatus(string) error
	WaitForYellowStatus(string) error
}

const (
	indexSetting = `{
"settings":{
	"number_of_shards":1,
	"number_of_replicas":1
	}
}`
)

type NodeSetting struct {
	Name   string `json:"name,omitempty"`
	Data   string `json:"data,omitempty"`
	Ingest string `json:"ingest,omitempty"`
	Master string `json:"master,omitempty"`
}

type PathSetting struct {
	Data []string `json:"data,omitempty" yaml:"data,omitempty"`
	Logs string   `json:"logs,omitempty" yaml:"logs,omitempty"`
	Home string   `json:"home,omitempty" yaml:"home,omitempty"`
}

type Setting struct {
	Node *NodeSetting `json:"node,omitempty" yaml:"node,omitempty"`
	Path *PathSetting `json:"path,omitempty" yaml:"path,omitempty"`
}

type NodeInfo struct {
	Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
	Roles    []string `json:"roles,omitempty" yaml:"roles,omitempty"`
	Settings *Setting `json:"settings,omitempty" yaml:"settings,omitempty"`
}

func GetElasticClient(kc kubernetes.Interface, extClient cs.Interface, db *api.Elasticsearch, url string) (ESClient, error) {
	secret, err := kc.CoreV1().Secrets(db.Namespace).Get(context.TODO(), db.Spec.AuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	elasicsearchversion, err := extClient.CatalogV1alpha1().ElasticsearchVersions().Get(context.TODO(), string(db.Spec.Version), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasPrefix(elasicsearchversion.Spec.Version, "5."):
		client, err := esv5.NewClient(
			esv5.SetHttpClient(&http.Client{
				Timeout: 0,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						MaxVersion:         tls.VersionTLS12,
					},
				},
			}),
			esv5.SetBasicAuth(string(secret.Data[core.BasicAuthUsernameKey]), string(secret.Data[core.BasicAuthPasswordKey])),
			esv5.SetURL(url),
			esv5.SetHealthcheck(false), // don't check health here. otherwise error message can be misleading for invalid credentials
			esv5.SetSniff(false),
		)
		if err != nil {
			return nil, err
		}

		// do a manual health check to test client
		_, err = client.ClusterHealth().Do(context.Background())
		if err != nil {
			return nil, err
		}

		return &ESClientV5{client: client}, nil
	// 6.x for searchguard & x-pack, 0.x for opendistro
	case strings.HasPrefix(string(elasicsearchversion.Spec.Version), "6."), strings.HasPrefix(string(elasicsearchversion.Spec.Version), "0."):
		client, err := esv6.NewClient(
			esv6.SetHttpClient(&http.Client{
				Timeout: 0,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						MaxVersion:         tls.VersionTLS12,
					},
				},
			}),
			esv6.SetBasicAuth(string(secret.Data[core.BasicAuthUsernameKey]), string(secret.Data[core.BasicAuthPasswordKey])),
			esv6.SetURL(url),
			esv6.SetHealthcheck(false), // don't check health here. otherwise error message can be misleading for invalid credentials
			esv6.SetSniff(false),
		)
		if err != nil {
			return nil, err
		}

		// do a manual health check to test client
		_, err = client.ClusterHealth().Do(context.Background())
		if err != nil {
			return nil, err
		}

		return &ESClientV6{client: client}, nil
	// 7.x for searchguard & x-pack, 1.x for opendistro
	case strings.HasPrefix(string(elasicsearchversion.Spec.Version), "7."), strings.HasPrefix(string(elasicsearchversion.Spec.Version), "1."):
		client, err := esv7.NewClient(
			esv7.SetHttpClient(&http.Client{
				Timeout: 0,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						MaxVersion:         tls.VersionTLS12,
					},
				},
			}),
			esv7.SetBasicAuth(string(secret.Data[core.BasicAuthUsernameKey]), string(secret.Data[core.BasicAuthPasswordKey])),
			esv7.SetURL(url),
			esv7.SetHealthcheck(false), // don't check health here. otherwise error message can be misleading for invalid credentials
			esv7.SetSniff(false),
		)
		if err != nil {
			return nil, err
		}

		// do a manual health check to test client
		_, err = client.ClusterHealth().Do(context.Background())
		if err != nil {
			return nil, err
		}

		return &ESClientV7{client: client}, nil
	}

	return nil, fmt.Errorf("unknown database verserion: %s", db.Spec.Version)
}
