# Warp Menu Generation

The `k8s-ces-assets` operator generates the `menu.json` that drives the warp menu.
It rebuilds the menu whenever one of its two sources changes:

- **[WarpMenuEntry-Custom-Resources](https://github.com/cloudogu/k8s-warp-menu-entry-lib/blob/main/docs/operations/warp_menu_entry_en.md)** — created by dogu developer to register their
  applications in the warp menu.
- **`k8s-ces-warp-config` ConfigMap** — defines default categories and static entries;
  managed via `values.yaml`.

---

## Configuring default categories and entries

Default categories and static entries are configured in `values.yaml` under the `warp` key.
The Helm chart renders these values into the `k8s-ces-warp-config` ConfigMap automatically —
**the ConfigMap should not be edited directly**.

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
      enabled: false           # set to true to enable this entry
      category: "Support"
      displayName:
        de: "cloudogu platform"
        en: "cloudogu platform"
      href: "https://platform.cloudogu.com"
```

### Categories (`warp.categories`)

Each key is the **category identifier** used by WarpMenuEntry resources and default entries
to place their links. Any identifier not listed here is created on the fly with a default
order of 9999.

| Field | Required | Description |
|---|---|---|
| `order` | no | Display position. Lower values appear further up in the menu. Default: 9999. |
| `displayName.de` | no | German category label. Falls back to the identifier if omitted. |
| `displayName.en` | no | English category label. Falls back to the identifier if omitted. |

### Default entries (`warp.defaultEntries`)

Default entries are static links that always appear in the menu regardless of which dogus
are installed. Set `enabled: false` to exclude an entry without removing it from
`values.yaml`.

| Field | Required | Description |
|---|---|---|
| `enabled` | yes | When `false`, the entry is excluded from the menu. |
| `category` | yes | Category identifier where this entry should be placed. |
| `displayName.de` | yes | German display name. |
| `displayName.en` | yes | English display name. |
| `href` | yes | URL or path. An absolute URL (with scheme, e.g. `https://`) opens in a new tab. A relative path (e.g. `/info/about`) opens in the same window. |

