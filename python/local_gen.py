# Copyright 2017 ETH Zurich
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
:mod:`local_gen` --- Local config generation tool for a SCIONLab VM
===================================================================

"""

# XXX (ercanucan): Note that this file contains some code from
# scion-web, though with modifications.
# Even though this is relatively a small piece of code, it may be a
# better to re-factore those functions in scion-web and use them here.


# Standard library
import argparse
import base64
import configparser
import json
import os
import yaml
from shutil import (
    copyfile,
    rmtree,
)
from string import Template

# External packages
from Crypto import Random
from nacl.signing import SigningKey

# SCION
from lib.crypto.asymcrypto import (
    generate_sign_keypair,
    generate_enc_keypair,
    get_enc_key_file_path,
    get_sig_key_file_path,
)
from lib.crypto.certificate import Certificate
from lib.crypto.certificate_chain import (
    CertificateChain,
    get_cert_chain_file_path,
)
from lib.crypto.trc import get_trc_file_path
from lib.defines import (
    AS_CONF_FILE,
    GEN_PATH,
    PROJECT_ROOT,
)
from lib.packet.scion_addr import ISD_AS
from lib.util import (
    read_file,
    write_file,
)
from topology.generator import (
    DEFAULT_PATH_POLICY_FILE,
    INITIAL_CERT_VERSION,
    INITIAL_TRC_VERSION,
    PATH_POLICY_FILE,
)

# TODO (ercanucan): Ensure that this folder exists!
GEN_ROOT = os.path.expanduser("~/scionLabConfigs")
SCION_COORD_PATH = os.path.expanduser("~/go/src/github.com/netsec-ethz/scion-coord")
DEFAULT_CORE_CERT_FILE = os.path.join(SCION_COORD_PATH, "python", "ISD1-AS1-V0.crt")
DEFAULT_CORE_SIG_KEY = os.path.join(SCION_COORD_PATH, "python", "as-sig.key")
DEFAULT_TRC_FILE = os.path.join(SCION_COORD_PATH, "python", "ISD1-V0.trc")


TYPES_TO_EXECUTABLES = {
    'router': 'border',
    'beacon_server': 'beacon_server',
    'path_server': 'path_server',
    'certificate_server': 'cert_server',
    'sibra_server': 'sibra_server'
}

TYPES_TO_KEYS = {
    'beacon_server': 'BeaconService',
    'certificate_server': 'CertificateService',
    'router': 'BorderRouters',
    'path_server': 'PathService',
    'sibra_server': 'SibraService'
}


def create_scionlab_vm_local_gen(args, tp):
    """
    Creates the usual gen folder structure for an ISD/AS under web_scion/gen,
    ready for Ansible deployment
    :param str isdas: ISD-AS as a string
    :param dict tp: the topology parameter file as a dict of dicts
    """
    new_ia = ISD_AS(args.joining_ia)
    core_ia = ISD_AS(args.core_ia)
    local_gen_path = os.path.join(GEN_ROOT, args.user_id, 'gen')
    # XXX (ercanucan): we remove user's past configs when he re-requests!
    rmtree(local_gen_path, ignore_errors=True)
    sig_priv_key, enc_priv_key, cert_chain = generate_certificate(
        new_ia, core_ia, args.core_sign_priv_key_file, args.core_cert_file)
    write_dispatcher_config(local_gen_path)
    master_as_key = base64.b64encode(Random.new().read(16))
    for service_type, type_key in TYPES_TO_KEYS.items():
        executable_name = TYPES_TO_EXECUTABLES[service_type]
        instances = tp[type_key].keys()
        for instance_name in instances:
            config = prep_supervisord_conf(tp[type_key][instance_name], executable_name,
                                           service_type, instance_name, new_ia)
            instance_path = get_elem_dir(local_gen_path, new_ia, instance_name)
            # TODO (ercanucan): pass the TRC file as a parameter
            write_certs_trc_keys(new_ia, instance_path, cert_chain, DEFAULT_TRC_FILE,
                                 sig_priv_key, enc_priv_key)
            write_as_conf_and_path_policy(new_ia, instance_path, master_as_key)
            write_supervisord_config(config, instance_path)
            write_topology_file(tp, type_key, instance_path)
            write_zlog_file(service_type, instance_name, instance_path)
    generate_zk_config(tp, new_ia, local_gen_path)


def write_dispatcher_config(local_gen_path):
    """
    Creates the supervisord and zlog files for the dispatcher and writes
    them into the dispatcher folder.
    :param str local_gen_path: the location to create the dispatcher folder in.
    """
    disp_folder_path = os.path.join(local_gen_path, 'dispatcher')
    if not os.path.exists(disp_folder_path):
        os.makedirs(disp_folder_path)
    disp_supervisord_conf = prep_dispatcher_supervisord_conf()
    write_supervisord_config(disp_supervisord_conf, disp_folder_path)
    write_zlog_file('dispatcher', 'dispatcher', disp_folder_path)


def prep_supervisord_conf(instance_dict, executable_name, service_type, instance_name, isd_as):
    """
    Prepares the supervisord configuration for the infrastructure elements
    and returns it as a ConfigParser object.
    :param dict instance_dict: topology information of the given instance.
    :param str executable_name: the name of the executable.
    :param str service_type: the type of the service (e.g. beacon_server).
    :param str instance_name: the instance of the service (e.g. br1-8-1).
    :param ISD_AS isd_as: the ISD-AS the service belongs to.
    :returns: supervisord configuration as a ConfigParser object
    :rtype: ConfigParser
    """
    config = configparser.ConfigParser()
    env_tmpl = 'PYTHONPATH=python:.,ZLOG_CFG="%s/%s.zlog.conf"'
    if service_type == 'router':  # go router
        env_tmpl += ',GODEBUG="cgocheck=0"'
        cmd = ('bash -c \'exec bin/%s -id "%s" -confd "%s" &>logs/%s.OUT\'') % (
            executable_name, instance_name, get_elem_dir(GEN_PATH, isd_as, instance_name),
            instance_name)
    else:  # other infrastructure elements
        cmd = ('bash -c \'exec bin/%s "%s" "%s" &>logs/%s.OUT\'') % (
            executable_name, instance_name, get_elem_dir(GEN_PATH, isd_as, instance_name),
            instance_name)
    env = env_tmpl % (get_elem_dir(GEN_PATH, isd_as, instance_name), instance_name)
    config['program:' + instance_name] = {
        'autostart': 'false',
        'autorestart': 'false',
        'environment': env,
        'stdout_logfile': 'NONE',
        'stderr_logfile': 'NONE',
        'startretries': '0',
        'startsecs': '5',
        'priority': '100',
        'command':  cmd
    }
    return config


def generate_zk_config(tp, isd_as, local_gen_path):
    """
    Generates Zookeeper configuration files for Zookeeper instances of an AS.
    :param dict tp: the topology of the AS provided as a dict of dicts.
    :param ISD_AS isd_as: ISD-AS for which the ZK config will be written.
    :param str local_gen_path: The gen path of scion-web.
    """
    for zk_id, zk in tp['ZookeeperService'].items():
        instance_name = 'zk%s-%s-%s' % (isd_as[0], isd_as[1], zk_id)
        write_zk_conf(local_gen_path, isd_as, instance_name, zk)


def write_zk_conf(local_gen_path, isd_as, instance_name, zk):
    """
    Writes a Zookeeper configuration file for the given Zookeeper instance.
    :param str local_gen_path: The gen path of scion-web.
    :param ISD_AS isd_as: ISD-AS for which the ZK config will be written.
    :param str instance_name: the instance of the ZK service (e.g. zk1-5-1).
    :param dict zk: Zookeeper instance information from the topology as a
    dictionary.
    """
    conf = {
        'tickTime': 100,
        'initLimit': 10,
        'syncLimit': 5,
        'dataDir': '/var/lib/zookeeper',
        'clientPort': zk['L4Port'],
        'maxClientCnxns': 0,
        'autopurge.purgeInterval': 1,
        'clientPortAddress': '127.0.0.1'
    }
    zk_conf_path = get_elem_dir(local_gen_path, isd_as, instance_name)
    zk_conf_file = os.path.join(zk_conf_path, 'zoo.cfg')
    write_file(zk_conf_file, yaml.dump(conf, default_flow_style=False))


def get_elem_dir(path, isd_as, elem_id):
    """
    Generates and returns the directory of a SCION element.
    :param str path: Relative or absolute path.
    :param ISD_AS isd_as: ISD-AS to which the element belongs.
    :param elem_id: The name of the instance.
    :returns: The directory of the instance.
    :rtype: string
    """
    return "%s/ISD%s/AS%s/%s" % (path, isd_as[0], isd_as[1], elem_id)


def prep_dispatcher_supervisord_conf():
    """
    Prepares the supervisord configuration for dispatcher.
    :returns: supervisord configuration as a ConfigParser object
    :rtype: ConfigParser
    """
    config = configparser.ConfigParser()
    env = 'PYTHONPATH=python:.,ZLOG_CFG="gen/dispatcher/dispatcher.zlog.conf"'
    cmd = """bash -c 'exec bin/dispatcher &>logs/dispatcher.OUT'"""
    config['program:dispatcher'] = {
        'autostart': 'false',
        'autorestart': 'false',
        'environment': env,
        'stdout_logfile': 'NONE',
        'stderr_logfile': 'NONE',
        'startretries': '0',
        'startsecs': '1',
        'priority': '50',
        'command':  cmd
    }
    return config


def write_topology_file(tp, type_key, instance_path):
    """
    Writes the topology file into the instance's location.
    :param dict tp: the topology as a dict of dicts.
    :param str type_key: key to describe service type.
    :param instance_path: the folder to write the file into.
    """
    path = os.path.join(instance_path, 'topology.json')
    with open(path, 'w') as file:
        json.dump(tp, file, indent=2)


def write_zlog_file(service_type, instance_name, instance_path):
    """
    Creates and writes the zlog configuration file for the given element.
    :param str service_type: the type of the service (e.g. beacon_server).
    :param str instance_name: the instance of the service (e.g. br1-8-1).
    """
    tmpl = Template(read_file(os.path.join(PROJECT_ROOT,
                                           "topology/zlog.tmpl")))
    cfg = os.path.join(instance_path, "%s.zlog.conf" % instance_name)
    write_file(cfg, tmpl.substitute(name=service_type,
                                    elem=instance_name))


def write_supervisord_config(config, instance_path):
    """
    Writes the given supervisord config into the provided location.
    :param ConfigParser config: supervisord configuration to write.
    :param instance_path: the folder to write the config into.
    """
    if not os.path.exists(instance_path):
        os.makedirs(instance_path)
    conf_file_path = os.path.join(instance_path, 'supervisord.conf')
    with open(conf_file_path, 'w') as configfile:
        config.write(configfile)


def generate_certificate(joining_ia, core_ia, core_sign_priv_key_file, core_cert_file):
    """
    """
    validity = Certificate.AS_VALIDITY_PERIOD
    comment = "AS Certificate"
    core_ia_sig_priv_key = base64.b64decode(read_file(core_sign_priv_key_file))
    public_key_sign, private_key_sign = generate_sign_keypair()
    public_key_encr, private_key_encr = generate_enc_keypair()
    cert = Certificate.from_values(
        str(joining_ia), str(core_ia), INITIAL_TRC_VERSION, INITIAL_CERT_VERSION, comment,
        False, validity, public_key_encr, public_key_sign, SigningKey(core_ia_sig_priv_key)
    )
    core_ia_chain = CertificateChain.from_raw(read_file(core_cert_file))
    joining_ia_chain = CertificateChain([cert, core_ia_chain.core_as_cert])
    return private_key_sign, private_key_encr, joining_ia_chain


def write_certs_trc_keys(isd_as, instance_path, cert_chain, trc_file, sig_priv_key, enc_priv_key):
    """
    Writes the certificate and the keys for the given service
    instance of the given AS.
    :param ISD_AS isd_as: ISD the AS belongs to.
    :param str instance_path: Location (in the file system) to write
    the configuration into.
    """
    # write keys
    sig_path = get_sig_key_file_path(instance_path)
    enc_path = get_enc_key_file_path(instance_path)
    write_file(sig_path, base64.b64encode(sig_priv_key).decode())
    write_file(enc_path, base64.b64encode(enc_priv_key).decode())
    # write cert
    cert_chain_path = get_cert_chain_file_path(
        instance_path, isd_as, INITIAL_CERT_VERSION)
    write_file(cert_chain_path, str(cert_chain))
    # write trc
    trc_path = get_trc_file_path(instance_path, isd_as[0], INITIAL_TRC_VERSION)
    copyfile(trc_file, trc_path)


def write_as_conf_and_path_policy(isd_as, instance_path, master_as_key):
    """
    Writes AS configuration (i.e. as.yml) and path policy files.
    :param ISD_AS isd_as: ISD-AS for which the config will be written.
    :param str instance_path: Location (in the file system) to write
    the configuration into.
    """
    conf = {
        'MasterASKey': master_as_key.decode('utf-8'),
        'RegisterTime': 5,
        'PropagateTime': 5,
        'CertChainVersion': 0,
        'RegisterPath': True,
    }
    conf_file = os.path.join(instance_path, AS_CONF_FILE)
    write_file(conf_file, yaml.dump(conf, default_flow_style=False))
    path_policy_file = os.path.join(PROJECT_ROOT, DEFAULT_PATH_POLICY_FILE)
    copyfile(path_policy_file, os.path.join(instance_path, PATH_POLICY_FILE))


def main():
    """
    Parse the command-line arguments and run the local config generation utility.
    """
    parser = argparse.ArgumentParser()
    parser.add_argument("--joining_ia",
                        help='ISD-AS for which the configuration is generated.')
    parser.add_argument("--core_ia",
                        help='Signing Core ISD-AS',
                        default='1-1')
    parser.add_argument("--core_sign_priv_key_file",
                        help='Signing private key of the core AS',
                        default=DEFAULT_CORE_SIG_KEY)
    parser.add_argument("--core_cert_file",
                        help='Certificate file of the signing core AS',
                        default=DEFAULT_CORE_CERT_FILE)
    parser.add_argument("--topo_file",
                        help='Topology file to be used for config generation.')
    parser.add_argument("--user_id",
                        help='User Identifier (email)')
    args = parser.parse_args()
    with open(args.topo_file) as json_data:
        topo_dict = json.load(json_data)
    create_scionlab_vm_local_gen(args, topo_dict)


if __name__ == '__main__':
    main()
