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
	"context"
	"time"

	"github.com/ericchiang/k8s"
	corev1 "github.com/ericchiang/k8s/apis/core/v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
)

func RotateKeys(client *k8s.Client, namespace string) {
	err, secrets := getSecretsToRotate(client, namespace)
	if err != nil {
		log.Fatal(err)
	}

	for _, secret := range secrets.Items {

		mySession := createSessionFromSecret(secret)

		accessKeyId := string(secret.Data[accessKeyIdPropName])
		oldAccessKeyId := string(secret.Data[accessKeyIdPropName])

		svc := iam.New(mySession)

		// List Keys and delete the one(s) we are not using
		keys, err := svc.ListAccessKeys(nil)
		if err != nil {
			log.Errorf("Unable to use new AccessKey")
			log.Fatal(err)
			continue
		} else {
			for _, k := range keys.AccessKeyMetadata {
				key := aws.StringValue(k.AccessKeyId)
				if key != accessKeyId {
					log.Infof("Found orphaned key %s, deleting it", key)
					deleteAccessKey(svc, key)
				}
			}
		}

		// Creating the new AccessKey

		result, err := svc.CreateAccessKey(nil)
		if err != nil {
			log.Errorf("Unable to create new AccessKey")
			log.Errorf(err.Error())
			continue
		}

		accessKey := result.AccessKey
		log.Infof("Created new AccessKey: %s", aws.StringValue(accessKey.AccessKeyId))

		// Wait for the key to be active
		time.Sleep(10 * time.Second)

		// Create a new Session
		newSession := createSession(aws.StringValue(accessKey.AccessKeyId), aws.StringValue(accessKey.SecretAccessKey), "new")
		newSvc := iam.New(newSession)

		// And make sure we can use it
		_, err = newSvc.ListAccessKeys(nil)
		if err != nil {
			log.Errorf("Unable to use new AccessKey")
			rollbackKeyCreation(svc, accessKey)
			log.Fatal(err)
		}

		// Update the secret in k8s
		err = updateSecret(client, secret, accessKey)
		if err != nil {
			log.Errorf("Unable to update kubernetes secret")
			rollbackKeyCreation(svc, accessKey)
			log.Fatal(err)
		}

		// Delete the old access key
		err = deleteAccessKey(newSvc, oldAccessKeyId)
		if err != nil {
			log.Errorf("Unable to delete old AccessKey")
			log.Fatal(err)
		} else {
			log.Infof("Successfully deleted old Access key (%s)", oldAccessKeyId)
		}

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
 * Rollback the creation of an AccessKey
 */
func rollbackKeyCreation(iamSvc *iam.IAM, accessKey *iam.AccessKey) {
	accessKeyId := aws.StringValue(accessKey.AccessKeyId)
	err := deleteAccessKey(iamSvc, accessKeyId)
	if err != nil {
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
