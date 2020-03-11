/*
Copyright © 2019 Nuxeo

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package lib

import (
	"fmt"
	"io/ioutil"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

var log = logrus.New()

const accessKeyIdPropName = "access_key_id"
const secretAccessKeyPropName = "secret_access_key"
const configPropName = "config"
const credentialsPropName = "credentials"
const rotateKeyLabel = "aws-rotate-key"

// loadClient parses a kubeconfig from a file and returns a Kubernetes
// client. It does not support extensions or client auth providers.
func LoadClient(kubeconfigPath string) (*k8s.Client, error) {

	if kubeconfigPath == "" {
		log.Info("Using in-cluster configuration")
		return k8s.NewInClusterClient()
	} else {
		data, err := ioutil.ReadFile(kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("read kubeconfig: %v", err)
		}

		// Unmarshal YAML into a Kubernetes config object.
		var config k8s.Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("unmarshal kubeconfig: %v", err)
		}
		return k8s.NewClient(&config)
	}
}

/**
 * Creates an AWS Session from a k8s Secret
 */
func createSessionFromSecret(secret *corev1.Secret) *session.Session {

	accessKeyId := string(secret.Data[accessKeyIdPropName])
	secretAccessKey := string(secret.Data[secretAccessKeyPropName])

	log.Infof("Creating session from secret %q containing accessKeyId=%s", *secret.Metadata.Name, accessKeyId)

	return createSession(accessKeyId, secretAccessKey, *secret.Metadata.Name+"-"+"orig")

}

/**
 * Creates an AWS Session using
 */
func createSession(accessKeyId string, secretAccessKey string, profileName string) *session.Session {
	return session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:      aws.String("eu-west-1"),
			Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, ""),
		},
		Profile: profileName,
	}))
}
