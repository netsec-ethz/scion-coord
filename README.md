# scion-coord
SCIONLab Coordination service

### Setup and configuration

#### Dependencies

##### Python and Go installation

Make sure that you have python3 and go installed and your GOPATH set up correctly.

##### Clone scion and scion-coord to GOPATH

Download this repository as well as `netsec-ethz/scion`:
```
go get github.com/netsec-ethz/scion
go get github.com/netsec-ethz/scion-coord
```

##### Go dependencies

The application uses govendor. You need to install govendor via:
`go get github.com/kardianos/govendor`

After this step, you can go to the `scion-coord` directory (`$GOPATH/src/github.com/netsec-ethz/scion-coord`) and populate the dependencies in the `vendor` folder using:
`govendor sync`


#### Custom configuration

Copy the default configuration file using
`cp conf/development.conf.default conf/development.conf`
and adjust the settings to fit your setup.
Make sure that you set `email.pm_server_token`, `email.pm_account_token`, `captcha.site_key`, and `captcha.secret_key` as described below.

##### Postmark

SCIONLab Coordination service uses [Postmark](https://postmarkapp.com/ "Postmark") to send emails. You will need an account token and a server token which are obtained by signing up to their service.
Set the corresponding fields in the configuration file accordingly.

##### Captcha

In the configuration file also populate the captcha fields with the keys generated [on this site](https://www.google.com/recaptcha/admin "Google ReCaptcha admin page").
For a quick start and for testing these keys can be used:

```
Site key: 6LeIxAcTAAAAAJcZVRqyHh71UMIEGNQ_MXjiZKhI
Secret key: 6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe
```
Warning: Use above keys only for testing purposes as they circumvent the captcha.

#### MySQL database

The project needs a working MySQL server instance running locally. 
Under Ubuntu, you can
install MySQL server with the following command:

`sudo apt-get install mysql-server`

Refer to the `conf/development.conf` regarding how to set your root password
for MySQL server installation. You also need to ensure that there is a
database named `scion_coord_test`. 

You can do this by first logging into your
MySQL server by
`mysql -h 127.0.0.1 -u root -p`
and then executing the following command:
`CREATE DATABASE scion_coord_test;`


#### Credentials

In order for the configurations to be generated, the following files must exist:
```
credentials/ISD1-AS1-V0.crt
credentials/as-sig.key
credentials/ISD1-V0.trc
```


### Run scion-coord


Afterwards, you can run `go run main.go` from the root folder.
Otherwise you can also run the application via:

```
go build
./scion-coord
```

##### Populate database

At the first run, the program will create the necessary database tables. In order to work properly, you will have to manually add one tuple by connecting to MySQL as described above and then issue `INSERT INTO scion_coord_test.s_c_i_o_n_lab_server VALUES (1, "1-7", "1.1.1.1", "50000");`

Note that these are dummy variables, which will not work for a production setup.


### Current APIs

The APIs are defined in the `main.go` file.
Additional documentation can be found under Wiki.
