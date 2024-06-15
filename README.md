## Downscaler

Project for downscale kubernetes deployments with time rules by namespaces


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
      recurrence: "MON-FRI"
      downscaler:
        downscalerSelectorTerms:
          matchExpressions:
            key: namespace
            operator: exclude
            values:
            - "local-path-storage"
            - "kube-system"
            - "downscaler"
        withNamespaceOpts:
          downscaleNamespacesWithTimeRules:
            rules:
              - namespaces: 
                - "nginx-2"
                withCron: "01:30-14:50"
              - namespaces:
                - "nginx-5"
                withCron: "01:30-14:52"
              - namespaces:
                - "nginx-6"
                withCron: "01:30-14:54"
              - namespaces:
                - "any-other"
                withCron: "01:30-14:56"
```

### Yaml breakdown

**timeZone**: will be evaluated to match withCriteria in the withAdvancedNamesaceOpts
**recurrence**: time window when the program will downscale the deployments
```yaml
timeZone: "America/Sao_Paulo"
recurrence: "MON-FRI"
```
<br>

**key**: only namespace is available for now. It means it will works at namespace level for match deployments to downscale
operator: only exclude is available for now. All namespaces under the list will be ignored during the downscaling scheduling
**values**: list of namespaces to be ignored during downscaling process, also it can override any namespace configured in the withAdvancedNamespaceOpts
```yaml
matchExpressions:
  key: namespace
  operator: exclude
  values:
  - "local-path-storage"
  - "kube-system"
  - "downscaler"
```

<br>

**namespaces**: a list of namespaces that all deployments will be downscaled to zero
withCron: the provided time will be evaluated to downscale the deployments. For example, 01:30-14:50 means after 14:50 all deployments in the provided namespace will be downscaled to zero

<br>

**any-other**: this is a special name to set under namespaces list such as the last index in the example below. It means that every deployment in any namespace in the cluster will be downscaled to zeroa except the namespaces provided in the matchExpressions like the example above

<br>

**even if the program still running, everything in the yaml can be updated in realtime, no need to restart the pod**


```yaml
downscaleNamespacesWithTimeRules:
  rules:
    - namespaces: 
      - "nginx-2"
      withCron: "01:30-14:50"
    - namespaces:
      - "nginx-5"
      withCron: "01:30-14:52"
    - namespaces:
      - "nginx-6"
      withCron: "01:30-14:54"
    - namespaces:
      - "any-other"
      withCron: "01:30-14:56"
```
<br>

**create downscaler namespace**

```
kubectl create namespace downscaler
```

**apply the downscaler crd**
```
kubectl apply -f deploy/crds/downscaler_crd.yaml
```

**apply the rbac permissions to grant api access to downscaler**
```
kubectl apply -f deploy/rbac/rbac.yaml
```
**apply the Downscaler kind with your needs**
```
kubectl apply -f deploy/deployment/downscaler.yaml
```
**apply the downscaler deployment itself**
```
kubectl apply -f deploy/deployment/deployment.yaml
```