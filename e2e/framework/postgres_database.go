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
	core "k8s.io/api/core/v1"
	"kmodules.xyz/client-go/tools/portforward"

	"crypto/rand"

	"fmt"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"kmodules.xyz/client-go/tools/certholder"
)

type PostgresInfo struct {
	DatabaseName string
	User         string
	Param        string
}
func (fi *Invocation) ForwardPort(meta metav1.ObjectMeta) (*portforward.Tunnel, error) {
	postgres, err := fi.GetPostgres(meta)
	if err != nil {
		return nil, err
	}

	clientPodName := fmt.Sprintf("%v-0", postgres.Name)
	tunnel := portforward.NewTunnel(portforward.TunnelOptions{
		Client:    fi.kubeClient.CoreV1().RESTClient(),
		Config:    fi.restConfig,
		Resource:  "pods",
		Name:      clientPodName,
		Namespace: postgres.Namespace,
		Remote:    api.PostgresDatabasePort,
	})
	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func (fi *Invocation) GetPostgresClient(tunnel *portforward.Tunnel, db *api.Postgres, dbName string) (*xorm.Engine, error) {
	dnsName := "127.0.0.1"
	user, pass, err := fi.GetPostgresAuthCredentials(db)
	if err != nil {
		return nil, fmt.Errorf("DB basic auth is not found for PostgreSQL %v/%v", db.Namespace, db.Name)
	}
	cnnstr := ""
	sslMode := db.Spec.SSLMode

	//  sslMode == "prefer" and sslMode == "allow"  don't have support for github.com/lib/pq postgres client. as we are using
	// github.com/lib/pq postgres client utils for connecting our server we need to access with  any of require , verify-ca, verify-full or disable.
	// here we have chosen "require" sslmode to connect postgres as a client
	if sslMode == "prefer" || sslMode == "allow" {
		sslMode = "require"
	}
	if db.Spec.TLS != nil {
		secretName := db.GetCertSecretName(api.PostgresClientCert)

		certSecret, err := fi.kubeClient.CoreV1().Secrets(db.Namespace).Get(context.TODO(), secretName, metav1.GetOptions{})

		if err != nil {
			return nil, err
		}

		certs, _ := certholder.DefaultHolder.ForResource(api.SchemeGroupVersion.WithResource(api.ResourcePluralPostgres), db.ObjectMeta)
		paths, err := certs.Save(certSecret)
		if err != nil {
			return nil, err
		}
		if db.Spec.ClientAuthMode == api.ClientAuthModeCert {
			cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=%v dbname=%s sslmode=%s sslrootcert=%s sslcert=%s sslkey=%s", user, pass, dnsName, tunnel.Local,dbName, sslMode, paths.CACert, paths.Cert, paths.Key)
		} else {
			cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=%v dbname=%s sslmode=%s sslrootcert=%s", user, pass, dnsName, tunnel.Local, dbName, sslMode, paths.CACert)
		}
	} else {
		cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=%v dbname=%s sslmode=%s", user, pass, dnsName, tunnel.Local,dbName, sslMode)
	}

	return xorm.NewEngine("postgres", cnnstr)
}


func (f *Invocation) EventuallyCreateSchema(db *api.Postgres, dbName string, userName string) GomegaAsyncAssertion {
	sql := fmt.Sprintf(`
DROP SCHEMA IF EXISTS "data" CASCADE;
CREATE SCHEMA "data" AUTHORIZATION "%s";`, userName)
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(db.ObjectMeta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			eng, err := f.GetPostgresClient(tunnel,db, dbName)
			if err != nil {
				return false
			}
			defer eng.Close()

			if err := f.CheckPostgres(eng); err != nil {
				return false
			}

			_, err = eng.Exec(sql)
			return err == nil
		},
		time.Minute*5,
		time.Second*5,
	)
}


var randChars = []rune("abcdefghijklmnopqrstuvwxyzabcdef")

// Use this for generating random pat of a ID. Do not use this for generating short passwords or secrets.
func characters(len int) string {
	bytes := make([]byte, len)
	_, err := rand.Read(bytes)
	Expect(err).NotTo(HaveOccurred())
	r := make([]rune, len)
	for i, b := range bytes {
		r[i] = randChars[b>>3]
	}
	return string(r)
}

func (f *Invocation) EventuallyPingDatabase(db *api.Postgres, dbName string, userName string) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(db.ObjectMeta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			db, err := f.GetPostgresClient(tunnel, db, dbName)
			if err != nil {
				return false
			}
			defer db.Close()

			if err := f.CheckPostgres(db); err != nil {
				return false
			}

			return true
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Invocation) EventuallyCreateTablePostgres(db *api.Postgres, dbName string, userName string, total int) GomegaAsyncAssertion {
	count := 0
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(db.ObjectMeta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			db, err := f.GetPostgresClient(tunnel,db, dbName)
			if err != nil {
				return false
			}
			defer db.Close()

			if err := f.CheckPostgres(db); err != nil {
				return false
			}

			for i := count; i < total; i++ {
				table := fmt.Sprintf("SET search_path TO \"data\"; CREATE TABLE %v ( id bigserial )", characters(5))
				_, err := db.Exec(table)
				if err != nil {
					return false
				}
				count++
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Invocation) EventuallyCountTablePostgres(db *api.Postgres, dbName string, userName string) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			tunnel, err := f.ForwardPort(db.ObjectMeta)
			if err != nil {
				return -1
			}
			defer tunnel.Close()

			db, err := f.GetPostgresClient(tunnel, db, dbName)
			if err != nil {
				return -1
			}
			defer db.Close()

			if err := f.CheckPostgres(db); err != nil {
				return -1
			}

			res, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema='data'")
			if err != nil {
				return -1
			}

			return len(res)
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) CheckPostgres(db *xorm.Engine) error {
	return db.Ping()
}

type PgStatArchiver struct {
	ArchivedCount int
}

func (f *Invocation) EventuallyCountArchive(db *api.Postgres, dbName string, userName string) GomegaAsyncAssertion {
	previousCount := -1
	countSet := false
	return Eventually(
		func() bool {
			tunnel, err := f.ForwardPort(db.ObjectMeta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			db, err := f.GetPostgresClient(tunnel,db, dbName)
			if err != nil {
				return false
			}
			defer db.Close()

			if err := f.CheckPostgres(db); err != nil {
				return false
			}

			var archiver PgStatArchiver
			if _, err := db.Limit(1).Cols("archived_count").Get(&archiver); err != nil {
				return false
			}

			if !countSet {
				countSet = true
				previousCount = archiver.ArchivedCount
				return false
			} else {
				if archiver.ArchivedCount > previousCount {
					return true
				}
			}
			return false
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Invocation) EventuallyPGSettings(db *api.Postgres, dbName string, userName string, config string) GomegaAsyncAssertion {
	configPair := strings.Split(config, "=")
	sql := fmt.Sprintf("SHOW %s;", configPair[0])
	return Eventually(
		func() []map[string][]byte {
			tunnel, err := f.ForwardPort(db.ObjectMeta)
			if err != nil {
				return nil
			}
			defer tunnel.Close()

			db, err := f.GetPostgresClient(tunnel,db, dbName)
			if err != nil {
				return nil
			}
			defer db.Close()

			if err := f.CheckPostgres(db); err != nil {
				return nil
			}

			results, err := db.Query(sql)
			if err != nil {
				return nil
			}
			return results
		},
		time.Minute*5,
		time.Second*5,
	)
}

//get user and pass from auth secret
func  (fi *Invocation) GetPostgresAuthCredentials(db *api.Postgres) (string, string, error) {
	if db.Spec.AuthSecret == nil {
		return "", "", errors.New("no database secret")
	}
	secret, err := fi.kubeClient.CoreV1().Secrets(db.Namespace).Get(context.TODO(), db.Spec.AuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	return string(secret.Data[core.BasicAuthUsernameKey]), string(secret.Data[core.BasicAuthPasswordKey]), nil
}

