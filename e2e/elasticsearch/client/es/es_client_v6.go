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
	"encoding/json"

	"github.com/appscode/go/crypto/rand"
	esv6 "gopkg.in/olivere/elastic.v6"
	"sigs.k8s.io/yaml"
)

type ESClientV6 struct {
	client *esv6.Client
}

var _ ESClient = &ESClientV6{}

func (c *ESClientV6) CreateIndex(count int) error {
	for i := 0; i < count; i++ {
		_, err := c.client.CreateIndex(rand.Characters(5)).Body(indexSetting).Do(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ESClientV6) CountIndex() (int, error) {
	indices, err := c.client.IndexNames()
	if err != nil {
		return 0, err
	}
	return len(indices), nil
}

func (c *ESClientV6) GetIndexNames() ([]string, error) {
	return c.client.IndexNames()
}

func (c *ESClientV6) GetAllNodesInfo() ([]NodeInfo, error) {
	data, err := c.client.NodesInfo().Metric("settings").Do(context.Background())

	nodesInfo := make([]NodeInfo, 0)
	for _, v := range data.Nodes {
		var info NodeInfo
		info.Name = v.Name
		info.Roles = v.Roles

		js, err := json.Marshal(v.Settings)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(js, &info.Settings)
		if err != nil {
			return nil, err
		}
		nodesInfo = append(nodesInfo, info)
	}
	return nodesInfo, err
}

func (c *ESClientV6) Stop() {
	c.client.Stop()
}

func (c *ESClientV6) Ping(url string) (int, error) {
	_, code, err := c.client.Ping(url).Do(context.Background())
	return code, err
}

func (c *ESClientV6) WaitForGreenStatus(timeout string) error {
	return c.client.WaitForGreenStatus(timeout)
}

func (c *ESClientV6) WaitForYellowStatus(timeout string) error {
	return c.client.WaitForYellowStatus(timeout)
}
