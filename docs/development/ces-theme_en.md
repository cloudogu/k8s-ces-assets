# CES Theme Integration

In the Ces Assets component, the [CES Theme Tailwind][ces-theme-tailwind] is integrated to provide the general CES styles.

## Templating
In the [theme-build](../../theme-build) folder, a NodeJS project is defined that includes the [CES Theme Tailwind][ces-theme-tailwind]. <!-- markdown-link-check-disable-line -->

### Default Styles
The default styles are embedded by nginx as `default.css` into every HTML page.  
These styles are defined in the file [default.css.tpl](../../resources/var/www/html/styles/default.css.tpl). <!-- markdown-link-check-disable-line -->  
The color variables are included as CSS custom properties from the [CES Theme Tailwind][ces-theme-tailwind].

With `yarn template-default-css`, the `default.css` is generated from the template file with all color variables.

### Error Pages
The error pages in the Ces Assets component all follow the same structure and differ only in certain texts and images.  
For this reason, there is only one template for the error pages: [error-page.html.tpl](../../resources/var/www/html/errors/error-page.html.tpl). <!-- markdown-link-check-disable-line -->

From this template, the error pages are generated using `yarn template-error-pages`.

The configuration of the individual error pages can be found in the file [error-pages.json](../../theme-build/error-pages.json). <!-- markdown-link-check-disable-line -->


[ces-theme-tailwind]: https://github.com/cloudogu/ces-theme-tailwind