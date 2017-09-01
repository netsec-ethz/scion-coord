# scion-coord
SCION Coordination service

### How to run it

#### Dependencies

The application uses govendor. You need to install govendor via:
`go get github.com/kardianos/govendor`

After this step, you can populate the dependencies in the `vendor` folder using:
`govendor sync`

SCION Coordination service uses [Postmark](https://postmarkapp.com/ "Postmark") to send emails. You will need an account token and a server token which are obtained by signing up to their service.
Set the corresponding fields in the configuration file accordingly.

In the configuration file also populate the captcha fields with the keys generated [on this site](https://www.google.com/recaptcha/admin "Google ReCaptcha admin page").
For a quick start and for testing these keys can be used:

```
Site key: 6LeIxAcTAAAAAJcZVRqyHh71UMIEGNQ_MXjiZKhI
Secret key: 6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe
```
Warning: Use above keys only for testing purposes as they circumvent the captcha.


#### MySQL database

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


#### Custom settings

You can change the settings in the config file located at: `conf/development.conf`


#### Credentials

In order for the configurations to be generated, the following files must exist:
```
credentials/ISD1-AS1-V0.crt
credentials/as-sig.key
credentials/ISD1-V0.trc
```


#### Run scion-coord


Afterwards, you can run `go run main.go` from the root folder.
Otherwise you can also run the application via:

```
go build
./scion-coord
```


### Current APIs

The APIs are defined in the `main.go` file.
Additional documentation can be found under Wiki.
