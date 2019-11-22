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
  "time"
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

const accessKeyIdPropName = "access_key_id"
const secretAccessKeyPropName = "secret_access_key"
const rotateKeyLabel = "aws-rotate-key"



func RotateKeys(client *k8s.Client, namespace string) {
  err, secrets := getSecretsToRotate(client, namespace);
  if(err != nil) {
    log.Fatal(err)
  }

  for _, secret := range secrets.Items {

    mySession := createSessionFromSecret(secret)
    oldAccessKeyId := string(secret.Data[accessKeyIdPropName])


    // Creating the new AccessKey
    svc := iam.New(mySession)
    result, err := svc.CreateAccessKey(nil)
    if(err != nil) {
      log.Errorf("Unable to create new AccessKey")
      log.Fatal(err)
    }

    accessKey := result.AccessKey
    log.Infof("Created new AccessKey: %s", aws.StringValue(accessKey.AccessKeyId))


    // Wait for the key to be active
    time.Sleep(10 * time.Second)

    // Create a new Session
    newSession := createSession(aws.StringValue(accessKey.AccessKeyId), aws.StringValue(accessKey.SecretAccessKey),"new")
    newSvc := iam.New(newSession)

    // And make sure we can use it
    _, err = newSvc.ListAccessKeys(nil)
    if(err != nil) {
      log.Errorf("Unable to use new AccessKey")
      rollbackKeyCreation(svc, accessKey)
      log.Fatal(err)
    }

    // Update the secret in k8s
    err = updateSecret(client, secret, accessKey);
    if(err != nil) {
      log.Errorf("Unable to update kubernetes secret")
      rollbackKeyCreation(svc, accessKey)
      log.Fatal(err)
    }

    // Delete the old access key
    err = deleteAccessKey(newSvc, oldAccessKeyId)
    if(err != nil) {
      log.Errorf("Unable to delete old AccessKey")
      log.Fatal(err)
    } else {
      log.Infof("Successfully deleted old Access key (%s)", oldAccessKeyId)
    }

  }

}


// loadClient parses a kubeconfig from a file and returns a Kubernetes
// client. It does not support extensions or client auth providers.
func LoadClient(kubeconfigPath string) (*k8s.Client, error) {

    if(kubeconfigPath == "") {
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
 * Returns the list of secret that we want to rotate.
 */
func getSecretsToRotate(client *k8s.Client, namespace string) (error, *corev1.SecretList) {

  l := new(k8s.LabelSelector)
  l.Eq(rotateKeyLabel, "true")

  var secrets corev1.SecretList
  if err := client.List(context.TODO(), namespace, &secrets, l.Selector()); err != nil {
    return err, nil
  }
  return nil, &secrets
}


/**
 * Creates an AWS Session from a k8s Secret
 */
func createSessionFromSecret(secret *corev1.Secret) *session.Session {

  accessKeyId := string(secret.Data[accessKeyIdPropName])
  secretAccessKey := string(secret.Data[secretAccessKeyPropName])

  log.Infof("Creating session from secret %q containing accessKeyId=%s", *secret.Metadata.Name, accessKeyId)

  return createSession(accessKeyId, secretAccessKey, *secret.Metadata.Name + "-" +"orig")

}

/**
 * Creates an AWS Session using
 */
func createSession(accessKeyId string, secretAccessKey string, profileName string) *session.Session {
  return session.Must(session.NewSessionWithOptions(session.Options{
    Config: aws.Config{
      Region: aws.String("us-east-1"),
      Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, ""),
    },
    Profile: profileName,
  }))
}

/**
 * Rollback the creation of an AccessKey
 */
func rollbackKeyCreation(iamSvc *iam.IAM, accessKey *iam.AccessKey) {
  accessKeyId := aws.StringValue(accessKey.AccessKeyId)
  err  := deleteAccessKey(iamSvc, accessKeyId)
  if(err != nil) {
    log.Errorf("Unable to delete new AccessKey, there are now probably 2 access keys for this user")
  } else {
    log.Errorf("Rollbacked new AccessKey (%s)", accessKeyId)
  }
}


/**
 * Updates a k8s secret with the given AWS AccessKey
 */
func updateSecret(client *k8s.Client, secret *corev1.Secret, accessKey *iam.AccessKey) error {

  secret.StringData = make(map[string]string)
  secret.StringData[accessKeyIdPropName] = aws.StringValue(accessKey.AccessKeyId)
  secret.StringData[secretAccessKeyPropName] = aws.StringValue(accessKey.SecretAccessKey)

  return client.Update(context.TODO(), secret)
}

/**
 * Deletes an AWS AccessKey based on its Id.
 */
func deleteAccessKey(iamSvc *iam.IAM, accessKeyId string) error {
  deleteAccessKeyInput := &iam.DeleteAccessKeyInput{
    AccessKeyId: aws.String(accessKeyId),
  }

  _, err := iamSvc.DeleteAccessKey(deleteAccessKeyInput)
  return err
}


