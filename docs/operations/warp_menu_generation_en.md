# Warp Menu Generation

The `k8s-ces-assets` operator generates the `menu.json` that drives the warp menu.
It rebuilds the menu whenever one of its two sources changes:

- **WarpMenuEntry custom resources** — created by dogu developer to register their
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

---

## Adding entries via WarpMenuEntry resources

Dogu developers register their applications in the warp menu by creating a `WarpMenuEntry`
custom resource in the operator namespace.

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

### Spec fields

| Field | Required | Constraints | Description |
|---|---|---|---|
| `displayName.de` | yes | 1–50 characters | German display name shown in the menu. |
| `displayName.en` | yes | 1–50 characters | English display name shown in the menu. |
| `category` | yes | 1–50 characters | Category identifier. Use a pre-defined key from `values.yaml` or provide any new identifier to create an ad-hoc category (order will be 9999). |
| `path` | yes | starts with `/` | Server-relative URL path, e.g. `/my-dogu`. Must not include a domain or scheme. |
| `disabled` | no | boolean | When `true`, the entry is hidden from the menu without deleting the resource. Defaults to `false`. |

### Status conditions

The operator sets two conditions on every WarpMenuEntry after reconciliation.

| Condition | Status | Meaning |
|---|---|---|
| `Ready` | `True` | Entry is valid and the menu was rebuilt successfully. |
| `Ready` | `False` | Entry has a validation error (e.g. bad path, empty category) or an internal error occurred. Check the condition message or operator events for details. |
| `Visible` | `True` | Entry is currently rendered in the menu. |
| `Visible` | `False` | Entry is hidden — either `disabled: true` is set or the entry failed validation. |

Inspect a resource with:

```shell
kubectl get warp -n ecosystem
kubectl describe warp my-dogu -n ecosystem
```

### Temporarily hiding an entry

Set `disabled: true` to remove an entry from the menu without deleting the resource:

```yaml
spec:
  disabled: true
```
