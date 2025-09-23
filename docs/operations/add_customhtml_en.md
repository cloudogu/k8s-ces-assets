# Adding Custom HTML Content

This document describes how custom HTML content can be delivered as `customhtml` via nginx.

## Creating a ConfigMap with Content

The content must be provided as a ConfigMap.

The keys within the ConfigMap serve as filenames.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: "my-customhtml"
data:
  sample.html: |-
    <html><body>Sample</body></html>
```

Multiple ConfigMaps can also be created and used. However, you must ensure that files are not duplicated across ConfigMaps.
If they are, it is non-deterministic which file will take precedence.

## Making the ConfigMaps Available via values.yaml

The custom HTML content is copied into the application by an InitContainer.
Therefore, the configuration takes place in the initContainer section.

```yaml
initContainer:
customHtmlMountPath: "/var/www/html/customhtml/"
configMaps:
- "my-customhtml"
- "your-customhtml"
```

The `initContainer.configMaps` entry is a list, which may be empty by default.

## Accessing Custom Content

By default, access to the content is provided through the route`/static`.

This can, however, be overridden using the values.yaml.
For this, the value nginx.manager.config.htmlContentUrl must be set.
