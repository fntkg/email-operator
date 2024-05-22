# k8s-operator

Kubernetes operator that manages custom resources for configuring email sending and sending of emails via a transactional email provider like MailerSend.

## Description

> The operator has been built with [Operator SDK](https://sdk.operatorframework.io/).

The CRD `Email` configuration is located in the `api/v1/email_types.go` file and the controller in `controllers/email_controller.go`.
The CRD `EmailSenderConfig` configuration is located in the `api/v1/emailsenderconfig_types.go` file and the controller in `controllers/emailsenderconfig_controller.go`.

The CRD EmailSenderConfig has a parameter not covered in the assignment statement, `provider`, which is used to configure the email provider to use. Possible values are `mailgun` and `emailsender`. Any other value will produce an error.
This value is used by the `email_controller.go` controller to determine which provider to use when sending an email.

This decision has been made to simplify the dynamic management of the possible different providers that could be added in the future.

## Getting Started

### Prerequisites
- go version v1.20.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Generate the manifests**

```sh
make generate
make manifests
```

**Build and push your image:**

It will be pushed to the personal registry you specified in the `Makefile` (https://github.com/fntkg/email-operator/blob/main/Makefile#L32).

```sh
make docker-build
```

**After building the image, push it to the registry:**

```sh
make docker-push
```

**NOTE:** This image ought to be published in the personal registry you specified. 
And it is required to have access to pull the image from the working environment. 
Make sure you have the proper permission to the registry if the above commands don’t work.

**Deploy the controller and the operator to the cluster:**

```sh
make deploy
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin 
privileges or be logged in as admin.

**Create instances of your solution**

👀 The first thing you have to do is to upload a secret to the cluster with the proper name that has a data called `apiToken`, make sure to encrypt it with base64
Example:
```yaml
apiVersion: v1
kind: Secret
metadata:
    name: mailersend-token
type: Opaque
data:
    apiToken: YzRlN2ExZTMzNTU4Y2I4ZDRiNDExMjJiNzhlODUyYTExNTYxYWRkY2Y2YTA1NmU0ZTJjMTg3ZTE5ODBiNTVlZA==
```

Remember that each `EmailSenderConfig` has to point to a secret in the cluster.

You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

There are 2 examples configured.
1. The first one configures and sends an email using the mailgun provider
2. The second one does the same but with the mailersend provider.

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

