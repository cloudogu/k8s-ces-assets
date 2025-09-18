# Hinzufügen einer neuen Resource

Diese Komponente liefert einige Dateien direkt in `/etc/nginx` bzw. `/var/www/html` aus. 

Einige dieser Dateien sind zusätzlich noch getemplatet und werden von `helm` bei der Installation erstellt bzw. bearbeitet. \
All diese Dateien werden als Config-Map von der Komponente bereitgestellt und in den manager-Container gemountet.

Um nicht jede Datei einzeln in eine Config-Map mounten zu müssen, wird beim rendern des Deployment-Templates
der Ordner `./k8s/helm/resources` durchsucht. Die Dateien und Ordner in dieser Quelle werden so behandelt, als ob sie sich unter dem `root` Pfad befinden.

`./k8s/helm/resources/etc/nginx/include.d/subfilters.conf` -> `/etc/nginx/include.d/subfilters.conf`

Innerhalb der Dateien unterhalb des Resources-Ordners kann die helm-Template-Syntax verwendet werden gemeinsam mit allen
Werten, die in der `values.yaml` konfiguriert wurden.