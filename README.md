# fiaas repository

[![Build Status](https://travis-ci.org/karmab/fiaas.svg?branch=master)](https://travis-ci.org/karmab/fiaas)

This script is meant to provide conditional access to floating ips using a
rbac kind of feature where tenants can have a set of authorized subnets associated ( per name or per id)

## requirements

install requirements.

- python-keystoneclient
- python-neutronclient
- python-flask
- python-iptools

If running on rhel, those packages but the last can be found respectively in the Red Hat Openstack and Extra channel
Last package is available from epel

## manual installation 

```
cp fiaas.py /usr/bin
cp fiaas.conf /etc
cp fiaas.service /usr/lib/systemd/system
```

## ansible installation 

```
ansible-playbook init.yml
```

## Configuration

The configuration is located at /etc/fiaas.conf. Here s a provided sample

```
[DEFAULT]
host = 0.0.0.0
port = 7000
ssl = False
log_dir = /var/log
log_file = fiaas.log
[keystone]
auth_url = http://192.168.100.5:5000/v2.0
admin_user = admin
admin_password = unix1234
admin_tenant = admin
[rbac]
enabled = True
mappings = testk:dev+pre
blacklist = 192.168.8.1,192.168.8.45
```

Revelant elements are:

- The DEFAULT​ section where the listening interface can be specified, and the listening
port ( 7000 by default). In this section, ssl can also be enabled. In this case, the file
/etc/fiaas.crt and /etc/fiaas.key will be used as certificate and private key, although
alternative paths can be provided with the keys *cert*​ and *key*
- The keystone​ section with the *auth_url*, *admin_user* which defaults to admin if not
found, *admin_password* and *admin_tenant*, which defaults to admin if not found
- The rbac​ section which is disabled by default. If enabled, then the mapping key is
looked at, with the format: TENANT:SUBNET1+SUBNET2+... , separating each tenant
with a comma

Note that instead of the subnet name, subnet id can be used.

## How to run it on server side

```
# systemctl start fiaas
```

Default logging will be visible in /var/log/messages with lines as follow

```
Nov 16 16:46:10 neutron fiaas.py: 2016-11-16 16:46:10.908 20543 INFO __main__ [-]
Associating ip 192.168.17.13 from subnet pre to tenant testk
Nov 16 16:46:10 neutron fiaas.py: 2016-11-16 16:46:10.910 20543 INFO werkzeug [-] 127.0.0.1
- - [16/Nov/2016 16:46:10] "POST /getip HTTP/1.1" 200 -
```

## How to run it on client side
The following is an example of a request done using curl

```
$ curl -X POST -u myuser:mypassword -d 'tenant=testk&subnet=pre' 127.0.0.1:7000/getip
```

Note that credentials are passed using a combination of user and password and providing the
destination tenant as part of the payload. The subnet from which an ip will be allocated will also
be provided as part of the payload.

	
##Problems?

Send me a mail at [karimboumedhel@gmail.com](mailto:karimboumedhel@gmail.com) !

Mac Fly!!!

karmab
