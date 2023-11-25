## Steps followed

```bash

$ cd ~\workspace\git.ws

$ git clone https://github.com/jittakal/lambda-con-scaler-operator.git

$ cd lambda-con-scaler-operator

$ go mod init github.com/jittakal/lambda-con-scaler-operator

$ kubebuilder init --domain operators.jittakal.io --repo github.com/jittakal/lambda-con-scaler-operator

$ kubebuilder create api --group aws --version v1alpha1 --kind LambdaConcurrencyScaler
```