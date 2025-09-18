# Adding a New Resource

This component delivers some files directly into `/etc/nginx` or `/var/www/html`.

Some of these files are additionally templated and created or modified by `helm` during installation. \
All of these files are provided by the component as a config map and mounted into the manager container.

To avoid having to mount each file individually into a config map, the `./k8s/helm/resources` folder is scanned when rendering the deployment template.  
The files and folders in this source are treated as if they were located under the `root` path.

`./k8s/helm/resources/etc/nginx/include.d/subfilters.conf` -> `/etc/nginx/include.d/subfilters.conf`

Within the files under the resources folder, the helm template syntax can be used together with all values configured in the `values.yaml`.
