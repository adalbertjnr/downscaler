### Examples

Deployment with upscaling=false:
```yaml
      containers:
        - name: downscaler-ctn
          image: ghcr.io/adalbertjnr/downscaler:latest
          imagePullPolicy: Always
          args:
            - --run_upscaling=false
```
Downscaler:
1 - ***if the flag run_upscaling is false the downscaler namespace should not be included in the exclude values because it is meant to be downscaled as well. Therefore, the downscaler namespace must be the last (chronologically) namespace to be downscaled. This means we can set it to unspecified such as the example below, which will not only be downscaled the downscaler namespace but every other namespace in the cluster. Alternatively, we can set the downscaler namespace, then every namespace will be preserved but not the downscaler and the namespaces presented in the rules list***
```yaml
apiVersion: scheduler.go/v1
kind: Downscaler
metadata:
  name: downscaler
  namespace: downscaler
spec:
  executionOpts:
    time:
      timeZone: "America/Sao_Paulo"
      recurrence: "MON-SAT"
      downscaler:
        downscalerSelectorTerms:
          matchExpressions:
            key: namespace
            operator: exclude
            values:
              - "kube-system"
              - "bar"
              - "baz"
        withNamespaceOpts:
          downscaleNamespacesWithTimeRules:
            rules:
              - namespaces:
                  - "nginx-2"
                withCron: "01:30-15:50"
              - namespaces:
                  - "nginx-5"
                withCron: "01:30-15:51"
              - namespaces:
                  - "nginx-6"
                withCron: "01:30-15:53"
              - namespaces:
                  - "unspecified"
                withCron: "01:30-15:57"

```
2 - ***every namespace must be downscaled to zero in the cluster at 8PM***
```yaml
apiVersion: scheduler.go/v1
kind: Downscaler
metadata:
  name: downscaler
  namespace: downscaler
spec:
  executionOpts:
    time:
      timeZone: "America/Sao_Paulo"
      recurrence: "MON-SAT"
      downscaler:
        downscalerSelectorTerms:
          matchExpressions:
            key: namespace
            operator: exclude
            values: []
        withNamespaceOpts:
          downscaleNamespacesWithTimeRules:
            rules:
              - namespaces:
                  - "unspecified"
                withCron: "01:30-08:00PM"
```
3 - ***only downscale nginx-1 namespace but preserve the rest. It is suggested to set the downscaler namespace as well, otherwise it will be up and running doing nothing in the cluster.***
```yaml
apiVersion: scheduler.go/v1
kind: Downscaler
metadata:
  name: downscaler
  namespace: downscaler
spec:
  executionOpts:
    time:
      timeZone: "America/Sao_Paulo"
      recurrence: "MON-SAT"
      downscaler:
        downscalerSelectorTerms:
          matchExpressions:
            key: namespace
            operator: exclude
            values: []
        withNamespaceOpts:
          downscaleNamespacesWithTimeRules:
            rules:
              - namespaces:
                  - "nginx-1"
                  - "downscaler"
                withCron: "01:30-08:00PM"
```
4 - ***downscale every namespace in the cluster at 10PM but not kube-system***
```yaml
apiVersion: scheduler.go/v1
kind: Downscaler
metadata:
  name: downscaler
  namespace: downscaler
spec:
  executionOpts:
    time:
      timeZone: "America/Sao_Paulo"
      recurrence: "MON-SAT"
      downscaler:
        downscalerSelectorTerms:
          matchExpressions:
            key: namespace
            operator: exclude
            values:
             - kube-system
        withNamespaceOpts:
          downscaleNamespacesWithTimeRules:
            rules:
              - namespaces:
                  - "unspecified"
                withCron: "01:30-10:00PM"
```

Deployment with upscaling=true
```yaml
      containers:
        - name: downscaler-ctn
          image: ghcr.io/adalbertjnr/downscaler:latest
          imagePullPolicy: Always
          args:
            - --run_upscaling=true
```

1 - ***with run_upscaling true, the only difference is that the downscaler namespace must be included in the values to be excluded during downscaling scheduling because it needs to be running to upscaling the namespaces during the upscaling time***
```yaml
apiVersion: scheduler.go/v1
kind: Downscaler
metadata:
  name: downscaler
  namespace: downscaler
spec:
  executionOpts:
    time:
      timeZone: "America/Sao_Paulo"
      recurrence: "MON-SAT"
      downscaler:
        downscalerSelectorTerms:
          matchExpressions:
            key: namespace
            operator: exclude
            values:
             - downscaler
             - kube-system
        withNamespaceOpts:
          downscaleNamespacesWithTimeRules:
            rules:
              - namespaces:
                  - "unspecified"
                withCron: "01:30-10:00PM"
```