# Generierung der Einträge des Warp-Menüs

Der `k8s-ces-assets`-Operator generiert die `menu.json`, die das Warp-Menü steuert.
Das Menü wird bei jeder Änderung einer der beiden Quellen neu erstellt:

- **WarpMenuEntry-Custom-Resources** — werden von Dogus-Entwicklern erstellt, um ihre
  Anwendungen im Warp-Menü zu registrieren.
- **ConfigMap `k8s-ces-warp-config`** — definiert Standardkategorien und statische
  Einträge; wird über die `values.yaml` verwaltet.

---

## Standardkategorien und -einträge konfigurieren

Standardkategorien und statische Einträge werden in der `values.yaml` unter dem Schlüssel
`warp` konfiguriert. Helm rendert diese Werte automatisch in die ConfigMap
`k8s-ces-warp-config` — **die ConfigMap sollte nicht direkt bearbeitet werden**. Eine Konfiguration kann wie folgt aussehen:

```yaml
warp:
  categories:
    Development Apps:
      order: 100
      displayName:
        de: "Anwendungen"
        en: "Applications"
    Support:
      order: 400
      displayName:
        de: "Support"
        en: "Support"

  defaultEntries:
    docsCloudoguComUrl:
      enabled: true
      category: "Support"
      displayName:
        de: "Cloudogu EcoSystem Dokumentation"
        en: "Cloudogu EcoSystem documentation"
      href: "https://docs.cloudogu.com"
    aboutCloudoguToken:
      enabled: true
      category: "Support"
      displayName:
        de: "Über Cloudogu"
        en: "About Cloudogu"
      href: "/info/about"
    platform:
      enabled: false           # auf true setzen, um diesen Eintrag zu aktivieren
      category: "Support"
      displayName:
        de: "cloudogu platform"
        en: "cloudogu platform"
      href: "https://platform.cloudogu.com"
```

### Kategorien (`warp.categories`)

Jeder Schlüssel ist der **Kategoriebezeichner**, den WarpMenuEntry-Resources und
Standard-Einträge verwenden, um Links einzusortieren. Unbekannte Kategorien werden
automatisch unten im Warp-Menü angelegt.

| Feld | Pflicht | Beschreibung |
|---|---|---|
| `order` | nein | Anzeigeposition. Niedrigere Werte erscheinen weiter oben. Standard: 9999. |
| `displayName.de` | nein | Deutscher Kategoriename. Ohne Angabe wird der Bezeichner verwendet. |
| `displayName.en` | nein | Englischer Kategoriename. Ohne Angabe wird der Bezeichner verwendet. |

### Standard-Einträge (`warp.defaultEntries`)

Standard-Einträge sind statische Links, die unabhängig von den installierten Dogus immer
im Menü erscheinen. Mit `enabled: false` lässt sich ein Eintrag deaktivieren, ohne ihn
aus der `values.yaml` zu entfernen.

| Feld | Pflicht | Beschreibung |
|---|---|---|
| `enabled` | ja | Bei `false` wird der Eintrag nicht im Menü angezeigt. |
| `category` | ja | Kategorie, dem dieser Eintrag zugeordnet wird. |
| `displayName.de` | ja | Deutscher Anzeigename. |
| `displayName.en` | ja | Englischer Anzeigename. |
| `href` | ja | URL oder Pfad. Eine absolute URL (mit Schema, z. B. `https://`) öffnet den Link in einem neuen Tab. Ein relativer Pfad (z. B. `/info/about`) öffnet ihn im selben Fenster. |

---

## Einträge über WarpMenuEntry-Resources hinzufügen

Dogu-Entwickler registrieren ihre Anwendungen im Warp-Menü, indem sie eine
`WarpMenuEntry`-Custom-Resource im zugehörigen Namespace erstellen.

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: WarpMenuEntry
metadata:
  name: my-dogu
  namespace: ecosystem
spec:
  displayName:
    de: "Mein Dogu"
    en: "My Dogu"
  category: "Development Apps"
  path: /my-dogu
```

### Spec-Felder

| Feld | Pflicht | Einschränkungen | Beschreibung |
|---|---|---|---|
| `displayName.de` | ja | 1–50 Zeichen | Deutscher Anzeigename im Menü. |
| `displayName.en` | ja | 1–50 Zeichen | Englischer Anzeigename im Menü. |
| `category` | ja | 1–50 Zeichen | Kategorie. Einen vordefinierten Schlüssel aus der `values.yaml` verwenden oder einen neuen angeben (Reihenfolge: 9999). |
| `path` | ja | beginnt mit `/` | Serverrelativer URL-Pfad, z. B. `/my-dogu`. Darf keine Domain oder Schema enthalten. |
| `disabled` | nein | boolean | Bei `true` wird der Eintrag aus dem Menü ausgeblendet, ohne die Resource zu löschen. Standard: `false`. |

### Status-Conditions

Der Operator setzt nach jeder Reconciliation zwei Conditions am WarpMenuEntry.

| Condition | Status | Bedeutung |
|---|---|---|
| `Ready` | `True` | Eintrag ist gültig und das Menü wurde erfolgreich neu erstellt. |
| `Ready` | `False` | Eintrag hat einen Validierungsfehler (z. B. ungültiger Pfad, leere Kategorie) oder ein interner Fehler ist aufgetreten. Condition-Message oder Operator-Events prüfen. |
| `Visible` | `True` | Eintrag ist aktuell im Menü gerendert. |
| `Visible` | `False` | Eintrag ist ausgeblendet — entweder weil `disabled: true` gesetzt ist oder weil der Eintrag die Validierung nicht bestanden hat. |

Ressource inspizieren:

```shell
kubectl get warp -n ecosystem
kubectl describe warp my-dogu -n ecosystem
```

### Eintrag vorübergehend ausblenden

`disabled: true` setzen, um einen Eintrag zu verstecken, ohne die Resource zu löschen:

```yaml
spec:
  disabled: true
```
