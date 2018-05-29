
import argparse
import requests
import json
import base64
import os
import sys
import pathlib
import re
import glob

from lib.crypto.certificate import Certificate
from lib.crypto.certificate_chain import CertificateChain
from lib.crypto.asymcrypto import sign, verify
# from local_config_util import (
#     Certificate
# )


SCION_COORD_URL = "http://localhost:8080"

def solve_challenge(challenge):
    """
    The parameter challenge comes in binary already
    """
    print ("solving challenge, len=", len(challenge))
    # 1) get the location of the certificates and keys
    SC = os.environ["SC"] if "SC" in os.environ else os.path.join(str(pathlib.Path.home()), "go", "src", "github.com", "scionproto", "scion")

    SC = os.path.join(SC, "gen")
    try:
        with open(os.path.join(SC, "ia")) as f:
            ia = f.readline()
    except Exception as ex:
        print(ex)
        sys.exit(1)
    m = re.match("^([0-9]+)-([0-9]+)$", ia)
    if not m:
        print ("ERROR: could not understand the IA from: ", ia)
    I = m.group(1)
    A = m.group(2)
    filepath = os.path.join(SC, "ISD"+I, "AS"+A, "bs"+I+"-"+A+"-1")



    # we don't need this: -------------------------
    certificate = os.path.join(filepath, "certs")
    certificates = [c for c in sorted(os.listdir(certificate), reverse=True) if c.endswith(".crt")]
    if len(certificates) < 1:
        print("ERROR: could not find a certificate under ", certificate)
        sys.exit(1)
    certificate = os.path.join(certificate, certificates[0])
    try:
        with open(certificate) as f:
            certificate = f.read()
    except Exception as ex:
        print("ERROR: could not read file %s: %s" % (certificate, ex))
        sys.exit(1)
    # -------------------------------------




    privkey = os.path.join(filepath, "keys")
    privkeys = [k for k in sorted(os.listdir(privkey), reverse=True) if k.endswith(".seed")]
    if len(privkeys) < 1:
        print("ERROR: could not find a private key under ", privkey)
        sys.exit(1)
    privkey = os.path.join(privkey, privkeys[0])
    try:
        with open(privkey) as f:
            privkey = f.read()
    except Exception as ex:
        print("ERROR: could not read file %s: %s" % (privkey, ex))
        sys.exit(1)
    privkey = base64.standard_b64decode(privkey)
    


    # 2) instantiate the private key and certificate and sign the challenge
    signed = sign(challenge, privkey)


    # -------------------- we don't need this ------
    chain = CertificateChain.from_raw(certificate)
    certificate = chain.as_cert
    publickey = certificate.subject_sig_key_raw

    result = verify(challenge, signed, publickey)
    print ("signature verified? ", result)
    # ----------------------------------------------


    return signed




def something_pending():
    url = SCION_COORD_URL + "/api/as/remapId/" + IA
    try:
        resp = requests.get(url)
    except requests.exceptions.ConnectionError as e:
        print ("Error querying Coordinator: ", e)
        return False
    content = resp.content.decode('utf-8')
    content = json.loads(content)
    print ("------------------------- ANS: -----------------------")
    print (content)


    answer = {"challenge": content["challenge"]}
    challenge = base64.standard_b64decode(content["challenge"])
    solution = solve_challenge(challenge=challenge)
    answer["answer"] = base64.standard_b64encode(solution).decode("utf-8")
    print ("-------------- POST Solution to challenge: ---------------")
    print (answer)
    
    try:
        while url:
            resp = requests.post(url, json=answer, allow_redirects=False)
            url = resp.next.url if resp.is_redirect and resp.next else None
    except requests.exceptions.ConnectionError as e:
        print ("Error querying Coordinator solving challenge: ", e)
        return False
    content = resp.content.decode('utf-8')
    content = json.loads(content)
    print ("------------------------- Reply from Coordinator after solution: -----------------------")
    print (content)
    return True

def parse_command_line_args():
    # global IA, ACC_ID, ACC_PW
    global IA
    parser = argparse.ArgumentParser(description="Update the SCION gen directory with new credentials, if needed")
    
    parser.add_argument("--ia", required=True, type=str,
                        help="The IA of this AS")
    
    # parser.add_argument("--accountId", required=True, type=str,
    #                     help="The SCION Coordinator account ID that has permission to access this AS")
    # parser.add_argument("--secret", required=True, type=str,
    #                     help="The secret for the SCION Coordinator account that has permission to access this AS")
    
    # The required arguments will be present, or parse_args will exit the application
    args = parser.parse_args()
    
    IA = args.ia    
    # ACC_ID = args.accountId
    # ACC_PW = args.secret


def main():
    parse_command_line_args()
    if something_pending():
        print ("Something is pending")
        pass


if __name__ == '__main__':
    main()
