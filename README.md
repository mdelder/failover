


## Project assembly

The initial project was created following the [Operator SDK Tutorial for Golang](https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/).

```bash
operator-sdk init operator-sdk init --domain=open-cluster-management.io

operator-sdk create api --group=failover --version=v1alpha1 --kind=FailoverConfig
go mod vendor

make generate
make manifests
```

## Developing/Contributing

Contributions are welcome and encouraged via Pull Requests.

```bash

make generate
make manifests
make install
make run
```

## References

[Operator SDK Advanced Topics](https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/)