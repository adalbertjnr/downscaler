# downscaler - experimental

### Inspired by: https://github.com/hjacobs/kube-downscaler

Project for scale down kubernetes deployments/statefulsets with time rules by namespaces

## Description

The idea is to scale down development/staging environments after working hours to reduce waste. Very handy with karpenter.
The project can be used with postgres, sqlite or memory.

- **Memory**: It will always scale down to 0 and scale up to 1 (If managed with argocd the argo could handle the correct amount of replicas)
- **Postgres**: It will always scale down to 0 and scale up to the last seen replicas before the scale down occurs.
- **Sqlite**: It will always scale down to 0 and scale up to the last seen replicas before the scale down occurs. The difference is the database the sqlite creates must be persisted, otherwise it will be removed when the pod dies.

## Getting started - Yaml example

**cronLoggerInterval:** It will print the next cronjobs runs in the provided interval.
**schedule:** Each namespace within timeRules will use the timeZone and recurrence to create the cron rule. Recurrence can be @daily and weekday-weekday. Example to create a config to run from monday to friday. **recurrence: MON-FRI**
**downscalerOptions.ResourceScaling:** It will create a default config for any index of rules, meaning it will consider to scale deployments/statefulsets (If some namespace have different needs, maybe only statefulsets, can be overrided with overrideScaling)
**downscalerOptions.timeRules.rules:** Each index is a config block with namespaces to scale during downscaleTime and upscaleTime.

**- Image: ghcr.io/adalbertjnr/downscaler:latest**

```yaml
apiVersion: downscaler.go/v1alpha1
kind: Downscaler
metadata:
  name: kubetime-scaler
  namespace: kubetime-scaler
spec:
  config:
    cronLoggerInterval: 60
  schedule:
    timeZone: "America/Sao_Paulo"
    recurrence: "@daily"
  downscalerOptions:
    resourceScaling:
      - deployments
      - statefulset
    timeRules:
      rules:
        - name: "Rule to downscale deployments from requirement A"
          namespaces:
            - "app"
            - "app2"
          upscaleTime: "16:33"
          downscaleTime: "16:30"
          overrideScaling: ["deployments"]

        - name: "Rule to downscale default resources options (deployments and statefulsets) from requirement B"
          namespaces:
            - "app3"
            - "app4"
          upscaleTime: "16:33"
          downscaleTime: "16:30"

        - name: "Rule to downscale statefulset from requirement C"
          namespaces:
            - "app10"
          upscaleTime: "16:33"
          downscaleTime: "16:30"
          overrideScaling: ["statefulset"]
```

#### To enable or switch database mode it must me done in the deployment object.

A flag need to be enabled: --database=true
**Variables:**
DB_DRIVER: sqlite/postgres
DB_ADDR: if sqlite = ""
DB_ADDR: if postgres = postgres://user:password@host:5432/db

**If provided something wrong instead of both these drivers, the app will fallback to memory mode.**

The config below will enable sqlite.

```yaml
containers:
  - name: downscaler
    image: "ghcr.io/adalbertjnr/downscaler:latest"
    command:
      - /manager
    args:
      - "--database=true"
    env:
      - name: DB_DRIVER
        value: "sqlite"
      - name: DB_ADDR
        value: ""
```

The config below will enable postgres.

```yaml
containers:
  - name: downscaler
    image: "ghcr.io/adalbertjnr/downscaler:latest"
    command:
      - /manager
    args:
      - "--database=true"
    env:
      - name: DB_DRIVER
        value: "postgres"
      - name: DB_ADDR
        value: "postgres://user:password@host:5432/db"
```

#### logging:

![alt text](./assets/logs.png)

### Installing:

```
- kubectl create namespace kubetime-scaler
```

- RBAC:

```
kubectl apply -f config/deploy/rbac
```

- CRDs:

```
kubectl apply -f config/deploy/crds
```

- Downscaler (customize it first):

```
kubectl apply -f config/deploy/downscaler
```

- Deployment:

```
kubectl apply -f config/deploy/deployment
```
