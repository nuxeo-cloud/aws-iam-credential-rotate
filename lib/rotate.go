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
	"context"
	"io/ioutil"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/ericchiang/k8s"
  corev1 "github.com/ericchiang/k8s/apis/core/v1"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/credentials"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/iam"
)


var log = logrus.New()


func RotateKeys(client *k8s.Client, namespace string) {
	ctx := context.Background()


	l := new(k8s.LabelSelector)
	l.Eq("aws-rotate-key", "true")

	var secrets corev1.SecretList
	if err := client.List(ctx, "", &secrets, l.Selector()); err != nil {
		log.Fatal(err)
	}

	for _, secret := range secrets.Items {

		accessKeyId := string(secret.Data["access_key_id"])
		secretAccessKey := string(secret.Data["secret_access_key"])

		log.Infof("Found %q containing accessKeyId=%s", *secret.Metadata.Name, accessKeyId)




	  mySession := session.Must(session.NewSession(&aws.Config{
			 Region: aws.String("us-east-1"),
			 Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, ""),
		}))

		// Create a IAM client from just a session.
		svc := iam.New(mySession)
		result, err := svc.CreateAccessKey(nil)
		if(err != nil) {
			log.Errorf("Unable to create new AccessKey")
			log.Fatal(err)
		}

		accessKey := result.AccessKey
		log.Infof("Created new AccessKey: %s", aws.StringValue(accessKey.AccessKeyId))

		secret.StringData = make(map[string]string)
    secret.StringData["access_key_id"] = aws.StringValue(accessKey.AccessKeyId)
    secret.StringData["secret_access_key"] = aws.StringValue(accessKey.SecretAccessKey)
    if err := client.Update(ctx, secret); err != nil {
    	log.Errorf("Unable to update kubernetes secret")
    	log.Fatal(err)
    }


    deleteAccessKeyInput := &iam.DeleteAccessKeyInput{
    	AccessKeyId: aws.String(accessKeyId),
    }

    _, err = svc.DeleteAccessKey(deleteAccessKeyInput)
    if(err != nil) {
    	log.Errorf("Unable to delete old AccessKey")
    	log.Fatal(err)
    }

	}

}


// loadClient parses a kubeconfig from a file and returns a Kubernetes
// client. It does not support extensions or client auth providers.
func LoadClient(kubeconfigPath string) (*k8s.Client, error) {

		if(kubeconfigPath == "") {
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