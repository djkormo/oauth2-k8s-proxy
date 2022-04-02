# oauth2-k8s-proxy
Project for creating 
Simple oath2 proxy for kubernetes services in go

```
go get golang.org/x/oauth2
go get github.com/coreos/go-oidc/v3/oidc
```

```
docker build . -t  djkormo/oauth2-k8s-proxy:main 

docker push djkormo/oauth2-k8s-proxy:main 

docker run djkormo/oauth2-k8s-proxy:main
```

```
kubectl apply -R -f deploy/manifests -n proxy

kubectl -n proxy logs deploy/oauth2-k8s-proxy -f

kubectl -n proxy get events  --sort-by=.metadata.creationTimestamp
```


Based on

https://mac-blog.org.ua/kubernetes-oauth2-proxy


