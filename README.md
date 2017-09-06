# scion-coord
SCIONLab Coordination service

### Setup and configuration

#### Dependencies

##### Setup go, scion, and scion-coord

Make sure that you have `go` and `python3` installed. Then, follow instructions 2 and 3 at https://github.com/netsec-ethz/scion.

Then, download this repository either by cloning it or by using `go get github.com/netsec-ethz/scion-coord`.


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
For a quick start and for testing the keys in the `development.conf.default` can be used.

Warning: Use these keys only for testing purposes as they circumvent the captcha.


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


#### Certificate authority setup

In order to use openVPN inside the generated SCIONLab VMs, it is necessary to set up a certificate authority.
We use the tool `easy-rsa` for this.

Install `easy-rsa` (version 2.x) using your package manager or download it from https://github.com/OpenVPN/easy-rsa/tree/release/2.x.

Copy the `easy-rsa` directory to the PACKAGE_DIRECTORY (we assume you are using the default `~/scionLabConfigs`). If you installed it via the package manager, you can usually do this by 
```
mkdir -p ~/scionLabConfigs
cp -r /usr/share/easy-rsa ~/scionLabConfigs
```

Now, copy the default configuration `cp conf/easy-rsa_vars.default ~/scionLabConfigs/easy-rsa/vars` and adjust the parameters if necessary. You may have to specify the openssl executable (be sure to use openssl 0.9.6, 0.9.8, or 1.0.x).
Now you can initialize your certificate authority by issuing (you can use the default for all prompts):
```
cd ~/scionLabConfigs/easy-rsa
source vars
./clean-all
./build-ca
./build-key-server server
./build-dh
```

You have to transfer files `ca.crt`, `server.crt`, `server.key`, and `dh4096.pem` located in the `keys` directory to the machine running the openVPN server. You should delete `server.key` from this machine.


### Run scion-coord

Afterwards, you can run `go run main.go` from the root folder.
Otherwise you can also run the application via:

```
go build
./scion-coord
```


### Current APIs

The APIs are defined in the `main.go` file.
Additional documentation can be found under Wiki.
