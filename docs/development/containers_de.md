# Container-Zugehörigkeit

Die `k8s-ces-assets`-Komponente besteht im Ganzen aus 4 Containern. 

* Init-Container zum Kopieren der custom-HTML-Resourcen und zum Sichern der Warp-Html-Resourcen.
* nginx-manager für das eigentliche Ausbringen der statischen Resourcen
* maintenance-Mode-Container zum Erzeugen der Error-Page für den Maintenance-Mode
* warp-Container zum generischen Erzeugen der warp-menu-Einträge

Der Init-Container und der nginx-manger verwenden beide das Image, welches aus dieser Komponente erstellt wird. \
Dies ist notwendig, da sich der Maintenance- un der Manager-Container eine Datei `503.html.tpl` teilen, 
diese aber in einem Ordner liegt der Dateien bereitstellt, die Image enthalten sind.

Um eine gemeinsame Nutzung des gesamten Ordners mit den Inhalten aus dem Image und darin einer gemeinsam gemounteten Datei in den Containern zu erreichen,
muss der Image-Ordner zunächst kopiert und nach dem Mounten wieder synchronisiert werden. Das Hochkopieren aus den Quellen übernimmt dabei der Init-Container.

Eine Bearbeitung der `503.html` über das entsprechende Template erfolgt in einer Go-Applikation (siehe `./maintenance`) im Maintenance-Container.
Dieser liest die Maintenance-Konfiguration aus der global-config aus und erzeugt aus dem Template die Fehlerseite, die dann über ein gemeinsames VolumnMount
mit dem Manager geteilt wird, der diese via nginx ausliefert.

Der Warp-Container erzeugt bei Änderungen an den Dogus eine Warp.json Datei, welche vom manager-Container verwendet und bereitgestellt wird.