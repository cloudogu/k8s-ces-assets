# Hinzufügen von eigenem HTML-Content

Dieses Dokument enthält eine Beschreibung, wie eigener HTML-Content als `customhtml` über den nginx ausgbracht werden kann.

## Anlegen einer Config-Map mit Inhalt

Der Inahlt muss als ConfigMap bereitgestellt werden.

Dabei gelten Die Schlüssel innerhalb der Config-Map als Dateinamen. 

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: "my-customhtml"
data:
  sample.html: |-
    <html><body>Sample</body></html>
```

Es können auch mehrere ConfigMaps erstellt und verwendet werden. Dabei muss jedoch selbst darauf geachtet werden, dass
Dateien nicht doppelt in den ConfigMaps eingetragen sind. Es ist nicht deterministisch, welche Datei dann den Vorzug bekommt.

## Verfügbarmachen der ConfigMaps über die values.yaml

Die Custom-HTML-Inhalte werden über einen InitContainer in die Applikation kopiert.
Daher erfolgt die Konfiguration im Abschnitt `initContainer`.

```yaml
initContainer:
  customHtmlMountPath: "/var/www/html/customhtml/"
  configMaps: 
   - "my-customhtml"
   - "your-customhtml"
```

Bei `initContainer.configMaps` handelt es sich um eine Liste, die grundsätzlich leer sein darf.

# Zugriff auf eigenen Inhalt.

Per default wird der Zugriff auf den Inhalt über die Route "/static" gewährleistet.

Mit Hilfe der values.yaml lässt sich dies jedoch Übersteuern. \
Dazu muss der Wert `nginx.manager.config.htmlContentUrl` gesetzt sein.


