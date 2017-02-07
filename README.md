# scion-coord
SCION Coordination service

### How to run it

The application uses go vendoring, so you need at least Go 1.6

First you need to get the dependencies and the `vendor` folder via:

```
go get ./...
godep save
godep save ./...
```

Afterwards, you can run `go run main.go` from the root folder.
Otherwise run:

```
go build
./scion-coord
```

Important:
The project needs a working MySQL server instance running locally. You can
install MySQL server with the following command:

`sudo apt-get install mysql-server`

Refer to the `conf/development.conf` regarding how to set your root password
for MySQL server installation. You also need to ensure that there is a
database named `scion_coord_test`. You can do this by first logging into your
MySQL server as follows:

`mysql -h 127.0.0.1 -u root -p`

and then executing the following command:

`CREATE DATABASE scion_coord_test;`

You can change the settings in the config file located at: `conf/development.conf`


### Current APIs

The APIs are defined in the `main.go` file.
Additional documentation can be found under Wiki.
