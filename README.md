# endpoints-escape
Drop Go endpoints 1.0 with the least amount of effort.

Add a method to your service struct with the path prefix you may have had before.

```
type AppDocsService struct{}

func (s *AppDocsService) EndpointPrefix() string {
	return "/appdocs/v1/"
}
```

Create a register function like so:
```
register := func(service endpoints.Service, orig, name, method, path, desc string, auth bool) {
  http.HandleFunc(service.EndpointPrefix()+path, endpoints.EndpointHandlerWrapper(service, orig))
}
```
Use almost as before, but create an instance of your struct and pass in as first argument. Note that depending on your particular usage, this might be the only change you have to make to your calls to register.

```
appDocsService := &AppDocsService{}
register(appDocsService, "AppDocsSubmit", "appDocsSubmit", "POST", "appDocsSubmit", "AppDocs submission endpoint", false)
```
