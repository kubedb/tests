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

package go_es

import (
	"context"
	"encoding/json"
	"strings"

	esv6 "github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	"github.com/pkg/errors"
)

type ESClientV6 struct {
	client *esv6.Client
}

func (es *ESClientV6) ClusterStatus() (string, error) {
	res, err := es.client.Cluster.Health(
		es.client.Cluster.Health.WithPretty(),
	)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if err2 := json.NewDecoder(res.Body).Decode(&response); err2 != nil {
		return "", errors.Wrap(err2, "failed to parse the response body")
	}
	if value, ok := response["status"]; ok {
		return value.(string), nil
	}
	return "", errors.New("status is missing")
}

func (es *ESClientV6) CreateIndex(_index string) error {
	req := esapi.IndicesCreateRequest{
		Index:  _index,
		Pretty: true,
		Human:  true,
	}

	res, err := req.Do(context.Background(), es.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return decodeError(res.Body, res.StatusCode)
	}

	return nil
}

func (es *ESClientV6) GetIndices(indices ...string) (map[string]interface{}, error) {
	req := esapi.IndicesGetRequest{
		Index:  indices,
		Pretty: true,
		Human:  true,
	}

	res, err := req.Do(context.Background(), es.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, decodeError(res.Body, res.StatusCode)
	}

	response := make(map[string]interface{})
	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response, nil
}

func (es *ESClientV6) DeleteIndex(indices ...string) error {
	req := esapi.IndicesDeleteRequest{
		Index:  indices,
		Pretty: true,
		Human:  true,
	}

	res, err := req.Do(context.Background(), es.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return decodeError(res.Body, res.StatusCode)
	}
	return nil
}

func (es *ESClientV6) PutData(_index, _type, _id string, data map[string]interface{}) error {
	var b strings.Builder
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "failed to Marshal data")
	}
	b.Write(dataBytes)

	req := esapi.CreateRequest{
		Index:        _index,
		DocumentType: _type,
		DocumentID:   _id,
		Body:         strings.NewReader(b.String()),
		Pretty:       true,
		Human:        true,
	}

	res, err := req.Do(context.Background(), es.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return decodeError(res.Body, res.StatusCode)
	}
	return nil
}

func (es *ESClientV6) GetData(_index, _type, _id string) (map[string]interface{}, error) {
	req := esapi.GetRequest{
		Index:        _index,
		DocumentType: _type,
		DocumentID:   _id,
		Pretty:       true,
		Human:        true,
	}

	res, err := req.Do(context.Background(), es.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, decodeError(res.Body, res.StatusCode)
	}

	response = make(map[string]interface{})
	if err2 := json.NewDecoder(res.Body).Decode(&response); err2 != nil {
		return nil, errors.Wrap(err2, "failed to parse the response body")
	}

	return response, nil
}
