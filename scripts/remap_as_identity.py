
import argparse
import requests
import json
import base64
import os
import sys
import pathlib
import re
import glob
import subprocess
import tempfile
import datetime
import shutil

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
    global IA
    print ("solving challenge, len=", len(challenge))
    # 1) get the location of the certificates and keys



    SC = os.environ["SC"] if "SC" in os.environ else os.path.join(str(pathlib.Path.home()), "go", "src", "github.com", "scionproto", "scion")
    SC = os.path.join(SC, "gen")
    # try:
    #     with open(os.path.join(SC, "ia")) as f:
    #         ia = f.readline()
    # except Exception as ex:
    #     print(ex)
    #     sys.exit(1)


    m = re.match("^([0-9]+)-([0-9]+)$", IA)
    if not m:
        print ("ERROR: could not understand the IA from: ", IA)
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
    print (certificate)
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
    print (certificate)
    chain = CertificateChain.from_raw(certificate)
    certificate = chain.as_cert
    print (certificate)
    publickey = certificate.subject_sig_key_raw

    result = verify(challenge, signed, publickey)
    print ("signature verified? ", result, ", using publickey: ", publickey)
    # ----------------------------------------------


    return signed




def something_pending():
    """
    Returns true/false if pending and
    dictionaty with the answer from the server
    """
    url = SCION_COORD_URL + "/api/as/remapId/" + IA
    try:
        resp = requests.get(url)
    except requests.exceptions.ConnectionError as e:
        print ("Error querying Coordinator: ", e)
        sys.exit(1)
    content = resp.content.decode('utf-8')
    content = json.loads(content)
    print ("------------------------- ANS: -----------------------")
    print (content)
    if "pending" not in content:
        print("ERROR: Wrong answer, does not contain the pending key")
        sys.exit(1)
    if not content["pending"]:
        sys.exit(1)


    answer = {"challenge": content["challenge"]}
    challenge = base64.standard_b64decode(content["challenge"])
    solution = solve_challenge(challenge=challenge)
    challenge_solution = base64.standard_b64encode(solution).decode("utf-8")
    answer["challenge_solution"] = challenge_solution
    print ("-------------- POST Solution to challenge: ---------------")
    print (answer)
    
    try:
        while url:
            resp = requests.post(url, json=answer, allow_redirects=False)
            url = resp.next.url if resp.is_redirect and resp.next else None
    except requests.exceptions.ConnectionError as e:
        print ("Error querying Coordinator solving challenge: ", e)
        sys.exit(1)
    content = resp.content.decode('utf-8')
    try:
        content = json.loads(content)
    except:
        content = {}
    print ("------------------------- Reply from Coordinator after solution: -----------------------")
    print (content)
    if content['error']:
        print("Error in the reply from the Coordinator after our solution to the challenge: ")
        print(content['msg'])
        sys.exit(2)
    # content["challenge"] = base64.standard_b64encode(challenge).decode("utf-8")
    content["challenge_solution"] = challenge_solution
    return True, content


def download_gen_folder(answer):
    mapped_ia = answer["ia"]
    # challenge = answer["challenge"]
    challenge_solution= answer["challenge_solution"]
    print("challenge_solution:", challenge_solution)
    url = SCION_COORD_URL + "/api/as/remapIdDownloadGen/" + IA

    try:
        while url:
            resp = requests.post(url, json=answer, allow_redirects=False)
            url = resp.next.url if resp.is_redirect and resp.next else None
    except requests.exceptions.ConnectionError as ex:
        print ("Error download Gen folder from Coordinator: ", ex)
        sys.exit(1)
    print("response:", resp)
    print(dir(resp))
    if resp.status_code != 200:
        print("Failed downloading gen folder")
        print(resp.body)
        sys.exit(2)
    # download a file with requests: https://stackoverflow.com/questions/16694907/how-to-download-large-file-in-python-with-requests-py
    filename = '/tmp/gen-data.tgz'
    with open(filename, 'wb') as f:
        for chunk in resp.iter_content(chunk_size=1024*1024):
            if chunk:
                f.write(chunk)
    print("Downloaded gen folder")
    return filename
    

def install_gen(gen_filename):
    """
    Installs the new gen folder. It assumes SCION is stopped
    """
    SC = os.environ['SC']
    with tempfile.TemporaryDirectory(prefix='gen-') as temp_dir:
        subprocess.check_output(['tar', 'xf', gen_filename], cwd=temp_dir)
        contents = os.listdir(temp_dir)
        if len(contents) != 1:
            print("Uncompressing file %s didn't return the right number of subdirectories" % (gen_filename,))
            sys.exit(1)
        p = os.path.join(temp_dir, contents[0])
        contents = os.listdir(p)
        if 'gen' not in contents:
            print('Could not find the gen directory in the contents of %s' % gen_filename)
            sys.exit(1)
        newgen_dir = os.path.join(p, 'gen')
        gen_dir = os.path.join(SC,'gen')
        os.rename(gen_dir, os.path.join(SC, 'gen.' + datetime.datetime.now().strftime('%Y-%m-%dT%H-%M-%S')))
        shutil.move(newgen_dir, gen_dir)

def notify_coordinator_all_okay():
    # TODO tell Coordinator we succeeded 
    pass

def stop_SCION():
    # TODO
    pass

def start_SCION():
    # TODO
    pass

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
    pending, answer = something_pending()
    if not pending:
        print ("Nothing is pending, out.")
        return 0
    stop_SCION()
    gen_file = download_gen_folder(answer)
    install_gen(gen_file)
    notify_coordinator_all_okay()
    start_SCION()



if __name__ == '__main__':
    main()
