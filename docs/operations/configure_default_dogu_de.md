# Konfiguration des Default-Dogus

Dieses Dokument beschreibt, wie das Default-Dogu konfiguriert werden kann.

Das Default-Dogu bestimmt, wohin ein Aufruf der Root-URL (`https://<host>/`) weiterleitet wird.

## Konfiguration über die values.yaml

Die Konfiguration erfolgt über den Wert `nginx.manager.config.defaultDogu`:

```yaml
nginx:
  manager:
    config:
      defaultDogu: "usermgt"
```

Der Wert entspricht dem Namen des Dogus, also dem Pfad, unter dem das Dogu erreichbar ist.
Im Beispiel oben wird ein Aufruf von `https://<host>/` auf `https://<host>/usermgt` weitergeleitet (HTTP 301).

Alternativ kann der Wert auch direkt beim Installieren bzw. Aktualisieren gesetzt werden:

```bash
helm upgrade --install k8s-ces-assets <chart> --set nginx.manager.config.defaultDogu=cas
```