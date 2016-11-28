#!/usr/bin/python
"""Script to assign next available floating ip from given external subnet to specified tenant
launch the following on client side:
curl -X POST -u testk:testk -d 'tenant=testk&subnet=dev' 127.0.0.1:5000/getip
requirements:
 - python-flask from redhat extra repos
 - python-iptools rpm from epel
 - keystone and neutron clients as found on network nodes
configuration:
 - create a configuration file /etc/neutron/fiaas.conf with the following content:
   [DEFAULT]
   listen = 0.0.0.0
   port = 5000
   [keystone]
   auth_url = http://192.168.100.5:5000/v2.0
   admin_password = unix1234
   [rbac]
   enabled = True
   mappings = testk:dev+pre+pro
"""

from flask import Flask
from flask import jsonify
from flask import request
from iptools import IpRange
import keystoneclient.v2_0.client as keystoneclient
from neutronclient.neutron import client as neutronclient
from oslo_config import cfg
from oslo_config.cfg import ConfigFilesNotFoundError
from oslo_config.cfg import ConfigFileParseError
from oslo_log import log as logging
import os

__version__ = "0.2"


LOG = logging.getLogger(__name__)
CONF = cfg.CONF
logging.register_options(CONF)
logging.setup(CONF, 'fiaas')
defaultgroup = cfg.OptGroup(name='DEFAULT')
defaultopts = [cfg.StrOpt('host', default='0.0.0.0'), cfg.IntOpt('port', default=7000), cfg.BoolOpt('ssl', default=False), cfg.StrOpt('cert', default='/etc/fiaas.crt'), cfg.StrOpt('key', default='/etc/fiaas.key')]
CONF.register_group(defaultgroup)
CONF.register_opts(defaultopts, defaultgroup)
authgroup = cfg.OptGroup(name='keystone')
authopts = [cfg.StrOpt('auth_url'), cfg.StrOpt('admin_user', default='admin'), cfg.StrOpt('admin_password'), cfg.StrOpt('admin_tenant', default='admin')]
CONF.register_group(authgroup)
CONF.register_opts(authopts, authgroup)

rbacgroup = cfg.OptGroup(name='rbac')
rbacopts = [cfg.BoolOpt('enabled', default=False), cfg.DictOpt('mappings', default=None), cfg.ListOpt('blacklist', default=None)]
CONF.register_group(rbacgroup)
CONF.register_opts(rbacopts, rbacgroup)

try:
    CONF(default_config_files=['/etc/fiaas.conf'])
except ConfigFilesNotFoundError:
    LOG.error("Missing configuration file /etc/fiaas.conf.Leaving...")
    os._exit(1)
except ConfigFileParseError:
    LOG.error("Incorrect configuration file /etc/fiaas.conf.Leaving...")
    os._exit(1)
host = CONF.DEFAULT.host
port = CONF.DEFAULT.port
auth_url = CONF.keystone.auth_url
admintenant = CONF.keystone.admin_tenant
adminuser = CONF.keystone.admin_user
adminpassword = CONF.keystone.admin_password
rbac = CONF.rbac.enabled
mappings = CONF.rbac.mappings
blacklist = CONF.rbac.blacklist
if auth_url is None or adminpassword is None:
    LOG.error("Incorrect keystone information in configuration file /etc/neutron/fiaas.conf.Leaving...")
    os._exit(0)
if mappings is not None:
    mappings = {k: mappings[k].split('+') for k in mappings}

ssl = CONF.DEFAULT.ssl
cert = CONF.DEFAULT.cert
key = CONF.DEFAULT.key
if ssl:
    context = (cert, key)
else:
    context = None

# auth_url = "http://192.168.100.5:5000/v2.0"
# admintenant = 'admin'
# adminuser = 'admin'
# adminpassword = 'unix1234'
# rbac = False
# mappings = {'testk': ['dev', 'pre', 'pro']}

admincredentials = {'username': adminuser, 'password': adminpassword, 'auth_url': auth_url, 'tenant_name': admintenant}


def authenticate(username, password, tenant):
    """This function is called to check if a username /
    password combination is valid.
    """
    try:
        usercredentials = {'username': username, 'password': password, 'auth_url': auth_url, 'tenant_name': tenant}
        keystoneclient.Client(**usercredentials)
        return True
    except Exception as error:
        print error
        return False

app = Flask(__name__)
# app.config.update(PROPAGATE_EXCEPTIONS=True)


@app.route("/getip", methods=['POST'])
def getip():
    auth = request.authorization
    username, password = auth.username, auth.password
    if 'subnet' not in request.form or 'tenant' not in request.form:
        message = {'error': {'message': 'Missing Data'}}
        response = jsonify(**message)
        response.status_code = 404
        return response
    if username is None or password is None:
        message = {'error': {'message': 'Missing Credentials'}}
        response = jsonify(**message)
        response.status_code = 401
        return response
    subnetname = request.form['subnet']
    tenantname = request.form['tenant']
    valid = authenticate(username, password, tenantname)
    if not valid:
        message = {'error': {'message': 'Wrong Credentials'}}
        response = jsonify(**message)
        response.status_code = 401
        return response
    k = keystoneclient.Client(**admincredentials)
    tenant = [t for t in k.tenants.list() if t.name == tenantname]
    tenantid = tenant[0].id
    neutronendpoint = k.service_catalog.url_for(service_type='network', endpoint_type='internalURL')
    n = neutronclient.Client('2.0', endpoint_url=neutronendpoint, token=k.auth_token, insecure=True, ca_cert=None)
    subnet = [i for i in n.list_subnets()['subnets'] if i['name'] == subnetname or i['id'] == subnetname]
    if not subnet:
        message = {'error': {'message': "Subnet %s not found" % subnetname}}
        response = jsonify(**message)
        response.status_code = 404
        return response
    else:
        subnet = subnet[0]
        if rbac and mappings is not None and tenantname in mappings and subnetname not in mappings[tenantname]:
            message = {'error': {'message': 'Subnet %s not allowed by RBAC' % subnetname}}
            response = jsonify(**message)
            response.status_code = 401
            return response
    networkid = subnet['network_id']
    # floatingips = [i['fixed_ips'][0]['ip_address'] for i in n.list_ports()['ports'] if i['device_owner'] == 'network:floatingip' or i['device_owner'] == 'network:router_gateway']
    cidr = subnet['cidr']
    range = IpRange(cidr)
    floatingips = [i['fixed_ips'][0]['ip_address'] for i in n.list_ports()['ports'] if 'ip_address' in i['fixed_ips'][0].keys() and i['fixed_ips'][0]['ip_address'] in range]
    if blacklist is not None:
        floatingips += blacklist
    for ip in range[2:-1]:
        if ip not in floatingips:
            break
    args = dict(floating_network_id=networkid, tenant_id=tenantid, floating_ip_address=ip)
    n.create_floatingip(body={'floatingip': args})
    LOG.info("Associating ip %s from subnet %s to tenant %s\n" % (ip, subnetname, tenantname))
    data = {'subnet': subnetname, 'tenant': tenantname, 'ip': ip}
    message = {'data': data}
    response = jsonify(**message)
    response.status_code = 200
    return response

if __name__ == "__main__":
    app.run(host=host, port=port, ssl_context=context)
