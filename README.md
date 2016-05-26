# scion-coord
SCION Coordination service

### How to run it

The application uses go vendoring, so you need at least Go 1.6  
It's enough to run `go run main.go` from the root foler.  
Otherwise run:  

```
go build
./scion-coord
```

Important:  
The project needs a working mysql server instance running locally.  
You can change the settings in the config file located at: `conf/development.conf`


### Current APIs

The APIs are defined in the `main.go` file.  
Additional documentation can be found under Wiki.
