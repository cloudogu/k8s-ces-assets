{{ $defaultDogu := .GlobalConfig.GetOrDefault "default_dogu" "default"}}
{{if eq $defaultDogu "default"}}
# redirect to ces-about page if no dogu is configured as default dogu
location = / {
  return 301 https://$host/info;
}
{{else}}
# redirect to configured default dogu
location = / {
  return 301 https://$host/{{ .GlobalConfig.Get "default_dogu" }};
}
{{end}}
