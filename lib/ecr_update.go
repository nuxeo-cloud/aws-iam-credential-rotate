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
	"encoding/base64"
  "context"
  "github.com/ericchiang/k8s"
  corev1 "github.com/ericchiang/k8s/apis/core/v1"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/service/ecr"


  "encoding/json"
)

// Dont  want to have full dependencies on k8s so copy/paste just
// to marshall dockerconfigJson
// https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/config.go
type DockerConfigJson struct {
	Auths DockerConfig `json:"auths"`
}

// DockerConfig represents the config file used by the docker CLI.
// This config that represents the credentials that should be used
// when pulling images from specific image repositories.
type DockerConfig map[string]DockerConfigEntry

type DockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}


func UpdateECR(client *k8s.Client, namespace string) {
  err, secrets := getSecretsToUpdate(client, namespace);
  if(err != nil) {
    log.Fatal(err)
  }

  for _, secret := range secrets.Items {

    log.Infof("Found ECR secret: %s", *secret.Metadata.Name)

    accessKeySecretName := secret.Metadata.Annotations["aws-ecr-updater/secret"]
    region := secret.Metadata.Annotations["aws-ecr-updater/region"]

    log.Infof("For region: %s", region)

    var accessKeySecret corev1.Secret
	  if err := client.Get(context.TODO(), namespace, accessKeySecretName, &accessKeySecret); err != nil {
	  	log.Errorf("Unable to get the secret to build AccessKey")
      log.Fatal(err)
	  }

    mySession := createSessionFromSecret(&accessKeySecret)


    // Get an authorization Token from ECR
    svc := ecr.New(mySession, aws.NewConfig().WithRegion(region))

    input := &ecr.GetAuthorizationTokenInput{}

    result, err := svc.GetAuthorizationToken(input)
    if(err != nil) {
      log.Errorf("Unable to get an Authorization token from ECR")
      log.Fatal(err)
    }

    log.Infof("Found %d authorizationData", len(result.AuthorizationData))

    err = updateSecretFromToken(client, secret, result.AuthorizationData[0])
    if(err != nil) {
      log.Errorf("Unable to update secret with Token")
      log.Fatal(err)
    }
    log.Infof("Secret %q updated with new ECR credentials", *secret.Metadata.Name)
  }

}


/**
 * Returns the list of secret that we want to rotate.
 */
func getSecretsToUpdate(client *k8s.Client, namespace string) (error, *corev1.SecretList) {

  l := new(k8s.LabelSelector)
  l.Eq("aws-ecr-updater", "true")

  var secrets corev1.SecretList
  if err := client.List(context.TODO(), namespace, &secrets, l.Selector()); err != nil {
    return err, nil
  }
  return nil, &secrets
}

/**
 * Updates a k8s secret with the given AWS ECR AuthorizationData
 */
func updateSecretFromToken(client *k8s.Client, secret *corev1.Secret, authorizationData *ecr.AuthorizationData) error {

	json, err := buildDockerJsonConfig(authorizationData)
	if(err != nil) {
		log.Errorf("Unable to build dockerJsonConfig from AuthorizationData")
		return err
	}

	if(secret.Metadata.Annotations == nil) {
		secret.Metadata.Annotations = make(map[string]string)
	}
	secret.Metadata.Annotations["aws-ecr-updater/expires-at"] = aws.TimeValue(authorizationData.ExpiresAt).String()

	secret.StringData = make(map[string]string)
  secret.StringData[".dockerconfigjson"] = string(json)
  return client.Update(context.TODO(), secret)
}

func buildDockerJsonConfig(authorizationData *ecr.AuthorizationData) ([]byte, error) {

	endpoint := aws.StringValue(authorizationData.ProxyEndpoint)

	dockerConfig := make(DockerConfig)
	user:= "AWS"
	token := aws.StringValue(authorizationData.AuthorizationToken)
	password := decodePassword(token)
	password = password[4:len(password)]

	dockerConfig[endpoint] = DockerConfigEntry {
		Username: user,
		Password: password,
		Email: "openshift@nuxeocloud.com",
		Auth: encodeDockerConfigFieldAuth(user, password),
	}

	config := &DockerConfigJson {
		Auths: dockerConfig,
	}

	return json.Marshal(config)
}


func decodePassword(pass string) string {
	bytes,_ :=  base64.StdEncoding.DecodeString(pass)
	return string(bytes)
}

func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}

