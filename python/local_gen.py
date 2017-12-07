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
:mod:`local_gen` --- Local config generation tool for a SCIONLab AS
===================================================================

"""

# Standard library
import argparse
import base64
import json
import os

# External packages
from Crypto import Random

# SCION
from lib.crypto.asymcrypto import (
    generate_sign_keypair,
    generate_enc_keypair,
)
from lib.crypto.certificate import Certificate
from lib.crypto.certificate_chain import CertificateChain
from lib.packet.scion_addr import ISD_AS
from lib.util import read_file
from topology.generator import INITIAL_CERT_VERSION, INITIAL_TRC_VERSION

# SCION-WEB
from ad_manager.util.local_config_util import (
    ASCredential,
    generate_zk_config,
    get_elem_dir,
    prep_supervisord_conf,
    write_as_conf_and_path_policy,
    write_certs_trc_keys,
    write_dispatcher_config,
    write_endhost_config,
    write_supervisord_config,
    write_topology_file,
    write_zlog_file,
    TYPES_TO_EXECUTABLES,
    TYPES_TO_KEYS,
)


# Directory structure and credential files
SCION_COORD_PATH = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
DEFAULT_PACKAGE_PATH = os.path.expanduser("~/scionLabConfigs")
DEFAULT_CORE_CERT_FILE = os.path.join(SCION_COORD_PATH, "credentials", "ISD1.crt")
DEFAULT_CORE_SIG_KEY = os.path.join(SCION_COORD_PATH, "credentials", "ISD1.key")
DEFAULT_TRC_FILE = os.path.join(SCION_COORD_PATH, "credentials", "ISD1.trc")


def create_scionlab_as_local_gen(args, tp):
    """
    Creates the usual gen folder structure for an ISD/AS under web_scion/gen,
    ready for Ansible deployment
    :param str isdas: ISD-AS as a string
    :param dict tp: the topology parameter file as a dict of dicts
    """
    new_ia = ISD_AS(args.joining_ia)
    core_ia = ISD_AS(args.core_ia)
    local_gen_path = os.path.join(args.package_path, args.user_id, 'gen')
    as_obj = generate_certificate(
        new_ia, core_ia, args.core_sign_priv_key_file, args.core_cert_file, args.trc_file)
    write_dispatcher_config(local_gen_path)
    for service_type, type_key in TYPES_TO_KEYS.items():
        executable_name = TYPES_TO_EXECUTABLES[service_type]
        instances = tp[type_key].keys()
        for instance_name in instances:
            config = prep_supervisord_conf(tp[type_key][instance_name], executable_name,
                                           service_type, instance_name, new_ia)
            instance_path = get_elem_dir(local_gen_path, new_ia, instance_name)
            # TODO(ercanucan): pass the TRC file as a parameter
            write_certs_trc_keys(new_ia, as_obj, instance_path)
            write_as_conf_and_path_policy(new_ia, as_obj, instance_path)
            write_supervisord_config(config, instance_path)
            write_topology_file(tp, type_key, instance_path)
            write_zlog_file(service_type, instance_name, instance_path)
    write_endhost_config(tp, new_ia, as_obj, local_gen_path)
    generate_zk_config(tp, new_ia, local_gen_path, simple_conf_mode=True)
    generate_sciond_config(tp, new_ia, local_gen_path, as_obj)


def generate_sciond_config(tp, ia, local_gen_path, as_obj):
    executable_name = "sciond"
    instance_name = "sd%s" % str(ia)
    service_type = "sciond"
    processes = []
    for svc_type in ["BorderRouters", "BeaconService", "CertificateService",
                     "HiddenPathService", "PathService"]:
        if svc_type not in tp:
            continue
        for elem_id, elem in tp[svc_type].items():
            processes.append(elem_id)
    processes.append(instance_name)
    config = prep_supervisord_conf(None, executable_name, service_type, instance_name, ia)
    config['group:'  "as%s" % str(ia)] = {'programs': ",".join(processes)}
    sciond_conf_path = get_elem_dir(local_gen_path, ia, "")
    write_supervisord_config(config, sciond_conf_path)


def generate_certificate(joining_ia, core_ia, core_sign_priv_key_file, core_cert_file, trc_file):
    """
    """
    validity = Certificate.AS_VALIDITY_PERIOD
    comment = "AS Certificate"
    core_ia_sig_priv_key = base64.b64decode(read_file(core_sign_priv_key_file))
    public_key_sign, private_key_sign = generate_sign_keypair()
    public_key_encr, private_key_encr = generate_enc_keypair()
    cert = Certificate.from_values(
        str(joining_ia), str(core_ia), INITIAL_TRC_VERSION, INITIAL_CERT_VERSION, comment,
        False, validity, public_key_encr, public_key_sign, core_ia_sig_priv_key)
    core_ia_chain = CertificateChain.from_raw(read_file(core_cert_file))
    sig_priv_key = base64.b64encode(private_key_sign).decode()
    enc_priv_key = base64.b64encode(private_key_encr).decode()
    joining_ia_chain = CertificateChain([cert, core_ia_chain.core_as_cert]).to_json()
    trc = open(trc_file).read()
    master_as_key = base64.b64encode(Random.new().read(16)).decode('utf-8')
    as_obj = ASCredential(sig_priv_key, enc_priv_key, joining_ia_chain, trc, master_as_key)
    return as_obj


def main():
    """
    Parse the command-line arguments and run the local config generation utility.
    """
    # TODO(mlegner): Add option specifying already existing keys and certificates
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
    parser.add_argument("--trc_file",
                        help='Trusted Root Configuration file',
                        default=DEFAULT_TRC_FILE)
    parser.add_argument("--topo_file",
                        help='Topology file to be used for config generation.')
    parser.add_argument("--package_path",
                        help='Path to generate and store AS configurations.',
                        default=DEFAULT_PACKAGE_PATH)
    parser.add_argument("--user_id",
                        help='User Identifier (email + IA)')
    args = parser.parse_args()

    with open(args.topo_file) as json_data:
        topo_dict = json.load(json_data)
    create_scionlab_as_local_gen(args, topo_dict)


if __name__ == '__main__':
    main()
