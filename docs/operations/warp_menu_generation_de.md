# Generierung der Einträge des Warp-Menüs

Der `k8s-ces-assets`-Operator generiert die `menu.json`, die das Warp-Menü steuert.
Das Menü wird bei jeder Änderung einer der beiden Quellen neu erstellt:

- **[WarpMenuEntry-Custom-Resources](https://github.com/cloudogu/k8s-warp-menu-entry-lib/blob/main/docs/operations/warp_menu_entry_de.md)** — werden von Dogus-Entwicklern erstellt, um ihre
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

