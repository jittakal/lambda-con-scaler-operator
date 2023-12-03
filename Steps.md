## Steps followed

### Intital operator kubebuilder steps

```bash

$ cd ~\workspace\git.ws

$ git clone https://github.com/jittakal/lambda-con-scaler-operator.git

$ cd lambda-con-scaler-operator

$ go mod init github.com/jittakal/lambda-con-scaler-operator

$ kubebuilder init --domain operators.jittakal.io --repo github.com/jittakal/lambda-con-scaler-operator

$ kubebuilder create api --group aws --version v1alpha1 --kind LambdaConcurrencyScaler
```

### Test locally

```bash
$ make manifest generate install run # or

# OR

$ make build
$ make install
$ make run
```

Open new terminal / bash shell

```bash
$ kubectl apply -f config/samples/aws_v1alpha1_lambdaconcurrencyscaler.yaml    
```

Delete the CRD application

```bash
$ kubectl delete -f config/samples/aws_v1alpha1_lambdaconcurrencyscaler.yaml
```

Stop local controller instance with cntrl + c key in first tab

Uninstall the kubernetes operator CRD

```bash
$ make uninstall
```

### Test within local cluster

Cleanup the deployment earlier if extis

```bash
$ kubectl delete -f config/samples/aws_v1alpha1_lambdaconcurrencyscaler.yaml

$ make undeploy

# Verify the crd does not list

$ kubectl get crd
```

Deploy crd and application

```bash
$ make build

$ make docker-build docker-push

$ kubectl create namespace lambda-con-scaler-operator-system

$ kubectl apply -f secret.yaml

$ kubectl create clusterrolebinding lambda-con-scaler-operator-prome-metrics --clusterrole=lambda-con-scaler-operator-metrics-reader --serviceaccount=observability:kube-prom-stack-kube-prome-prometheus


$ make deploy

# Verify crd is listed and running
$ kubectl get crd

$ kubectl apply -f config/samples/aws_v1alpha1_lambdaconcurrencyscaler.yaml
```
