# Configuring the Default Dogu

This document describes how the default dogu can be configured.

The default dogu determines where a call to the root URL (`https://<host>/`) is redirected to.

## Configuration via values.yaml

The configuration is done with the value `nginx.manager.config.defaultDogu`:

```yaml
nginx:
  manager:
    config:
      defaultDogu: "usermgt"
```

The value is the name of the dogu, i.e. the path under which the dogu is reachable.
In the example above, a call to `https://<host>/` is redirected to `https://<host>/usermgt` (HTTP 301).

Alternatively, the value can be set directly during installation or upgrade:

```bash
helm upgrade --install k8s-ces-assets <chart> --set nginx.manager.config.defaultDogu=cas
```
