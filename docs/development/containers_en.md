# Container Affiliation

The `k8s-ces-assets` component consists of 4 containers in total.

* Init container for copying the custom HTML resources and backing up the Warp HTML resources.
* nginx manager for serving the static resources
* maintenance mode container for generating the error page for maintenance mode
* warp container for generically generating the warp menu entries

Both the init container and the nginx manager use the image that is built from this component. \
This is necessary because the maintenance and manager containers share a file `503.html.tpl`,
which is located in a folder that serves files contained within the image.

To enable shared usage of the entire folder with the image contents, along with a commonly mounted file in the containers,
the image folder must first be copied and then synchronized again after mounting. Uploading from the sources is handled by the init container.

Editing of the `503.html` via the corresponding template is done in a Go application (see `./maintenance`) inside the maintenance container.  
This container reads the maintenance configuration from the global config and generates the error page from the template, which is then shared through a common VolumeMount with the manager, who serves it via nginx.

The warp container generates a Warp.json file whenever Dogus are changed, which is then used and served by the manager container.
