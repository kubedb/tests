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
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	appcat_util "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1/util"
)

const MongoDBPort = 27017

func (f *Framework) EventuallyAppBinding(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.GetAppBinding(meta)
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) CheckElasticsearchAppBindingSpec(meta metav1.ObjectMeta) error {
	elasticsearch, err := f.GetElasticsearch(meta)
	Expect(err).NotTo(HaveOccurred())

	appBinding, err := f.appCatalogClient.AppBindings(elasticsearch.Namespace).Get(context.TODO(), elasticsearch.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if appBinding.Spec.ClientConfig.Service == nil ||
		appBinding.Spec.ClientConfig.Service.Name != elasticsearch.ServiceName() ||
		appBinding.Spec.ClientConfig.Service.Port != api.ElasticsearchRestPort {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	if appBinding.Spec.Secret == nil ||
		appBinding.Spec.Secret.Name != elasticsearch.Spec.AuthSecret.Name {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	return nil
}

func (f *Framework) CheckMongoDBAppBindingSpec(meta metav1.ObjectMeta) error {
	mongodb, err := f.GetMongoDB(meta)
	Expect(err).NotTo(HaveOccurred())

	appBinding, err := f.appCatalogClient.AppBindings(mongodb.Namespace).Get(context.TODO(), mongodb.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if appBinding.Spec.ClientConfig.Service == nil ||
		appBinding.Spec.ClientConfig.Service.Name != mongodb.ServiceName() ||
		appBinding.Spec.ClientConfig.Service.Port != MongoDBPort {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	if appBinding.Spec.Secret == nil ||
		(appBinding.Spec.ClientConfig.CABundle == nil && appBinding.Spec.Secret.Name != mongodb.Spec.AuthSecret.Name) ||
		(appBinding.Spec.ClientConfig.CABundle != nil && appBinding.Spec.Secret.Name != mongodb.GetCertSecretName(api.MongoDBClientCert, "")) {
		return fmt.Errorf("appbinding %v/%v contains invalid secret", appBinding.Namespace, appBinding.Name)
	}
	return nil
}

func (f *Framework) CreateCustomAppBinding(oldAB *appcat.AppBinding, transfromFuncs ...func(in *appcat.AppBinding)) (*appcat.AppBinding, error) {
	newAB := &appcat.AppBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            oldAB.Name,
			Namespace:       oldAB.Namespace,
			Labels:          oldAB.Labels,
			Annotations:     oldAB.Annotations,
			OwnerReferences: oldAB.OwnerReferences,
		},
		Spec: appcat.AppBindingSpec{
			Type:         oldAB.Spec.Type,
			Version:      oldAB.Spec.Version,
			ClientConfig: oldAB.Spec.ClientConfig,
			Secret:       oldAB.Spec.Secret,
		},
	}
	newAB.Name = fmt.Sprintf("%s-custom", newAB.Name)
	// apply test specific modification
	for _, fn := range transfromFuncs {
		fn(newAB)
	}
	// create new AppBinding
	createdAB, _, err := appcat_util.CreateOrPatchAppBinding(
		context.TODO(),
		f.appCatalogClient,
		newAB.ObjectMeta,
		func(in *appcat.AppBinding) *appcat.AppBinding {
			in.Spec = newAB.Spec
			return in
		},
		metav1.PatchOptions{},
	)
	return createdAB, err
}

func (f *Framework) CheckRedisAppBindingSpec(meta metav1.ObjectMeta) error {
	redis, err := f.GetRedis(meta)
	Expect(err).NotTo(HaveOccurred())

	appBinding, err := f.appCatalogClient.AppBindings(redis.Namespace).Get(context.TODO(), redis.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if appBinding.Spec.ClientConfig.Service == nil ||
		appBinding.Spec.ClientConfig.Service.Name != redis.ServiceName() ||
		appBinding.Spec.ClientConfig.Service.Port != 6379 {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	return nil
}

// DeleteAppBinding deletes the custom appBinding that is created in test
func (f *Framework) DeleteAppBinding(meta metav1.ObjectMeta) error {
	return f.appCatalogClient.AppBindings(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (fi *Invocation) CheckMySQLAppBindingSpec(meta metav1.ObjectMeta) error {
	mysql, err := fi.GetMySQL(meta)
	Expect(err).NotTo(HaveOccurred())

	appBinding, err := fi.appCatalogClient.AppBindings(mysql.Namespace).Get(context.TODO(), mysql.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if appBinding.Spec.ClientConfig.Service == nil ||
		appBinding.Spec.ClientConfig.Service.Name != mysql.ServiceName() ||
		appBinding.Spec.ClientConfig.Service.Port != 3306 {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	if appBinding.Spec.Secret == nil ||
		appBinding.Spec.Secret.Name != mysql.Spec.AuthSecret.Name {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	return nil
}

func (fi *Invocation) CheckMariaDBAppBindingSpec(meta metav1.ObjectMeta) error {
	mariadb, err := fi.GetMariaDB(meta)
	Expect(err).NotTo(HaveOccurred())

	appBinding, err := fi.appCatalogClient.AppBindings(mariadb.Namespace).Get(context.TODO(), mariadb.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if appBinding.Spec.ClientConfig.Service == nil ||
		appBinding.Spec.ClientConfig.Service.Name != mariadb.ServiceName() ||
		appBinding.Spec.ClientConfig.Service.Port != 3306 {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	if appBinding.Spec.Secret == nil ||
		appBinding.Spec.Secret.Name != mariadb.Spec.AuthSecret.Name {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	return nil
}

func (fi *Invocation) GetMariaDBAppBinding(meta metav1.ObjectMeta) (*appcat.AppBinding, error) {
	return fi.Framework.appCatalogClient.AppBindings(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) GetAppBinding(meta metav1.ObjectMeta) (*appcat.AppBinding, error) {
	return f.appCatalogClient.AppBindings(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}
