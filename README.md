AWS IAM key rotate tool
=======================

This is a really simple tool that takes a secret containing IAM credentials, rotates them and updates the secret with the new credentials.


How to use
==========

Rotate key policy
-----------------

The following policy has to be created and attached to the user so that he can change his own keys:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "iam:UpdateAccessKey",
                "iam:CreateAccessKey",
                "iam:ListAccessKeys",
                "iam:DeleteAccessKey"
            ],
            "Resource": "arn:aws:iam::*:user/${aws:username}"
        }
    ]
}
```

IAM user
--------

Create a user in AWS and attach the previous policy to it. Generate an access key for the user.

Secret in Kubernetes
--------------------

The following secret will hold the initial credentials of the user.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-iam-user-credentials
  labels:
    aws-rotate-key: "true"
stringData:
  access_key_id: AKIASX3NJFVAYLY464VG
  secret_access_key: xxxxxxxxxxxxxxxxxxxxxxxxx
```

Cron Job
--------

Finally we need a CRON job that runs with privileges to list secrets:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: aws-credentials-updater

---

kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: secret-edit
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "watch", "list", "update"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: aws-credentials-updater-rolebinding
subjects:
- kind: ServiceAccount
  name: aws-credentials-updater
roleRef:
  kind: Role
  name: secret-edit
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: rotate-keys
spec:
  schedule: "*/15 * * * *"
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            parent: "rotate-keys"
        spec:
          containers:
          - name: rotate
            image: nuxeo/aws-iam-credential-rotate
          restartPolicy: Never
          serviceAccount: aws-credentials-updater
          serviceAccountName: aws-credentials-updater
  successfulJobsHistoryLimit: 50
  failedJobsHistoryLimit: 50
```

# Licensing

Most of the source code in the Nuxeo Platform is copyright Nuxeo and
contributors, and licensed under the Apache License, Version 2.0.

See the [LICENSE](LICENSE) file and the documentation page [Licenses](http://doc.nuxeo.com/x/gIK7) for details.

# About Nuxeo

Nuxeo dramatically improves how content-based applications are built, managed and deployed, making customers more agile, innovative and successful. Nuxeo provides a next generation, enterprise ready platform for building traditional and cutting-edge content oriented applications. Combining a powerful application development environment with SaaS-based tools and a modular architecture, the Nuxeo Platform and Products provide clear business value to some of the most recognizable brands including Verizon, Electronic Arts, Sharp, FICO, the U.S. Navy, and Boeing. Nuxeo is headquartered in New York and Paris. More information is available at [www.nuxeo.com](http://www.nuxeo.com).

