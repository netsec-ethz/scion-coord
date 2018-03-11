#!/usr/bin/python3

import smtplib
from email.message import EmailMessage
import os
import argparse
import configparser

class Configuration:

    def __init__(self, file_name):
        configParser = configparser.RawConfigParser()
        configFilePath = file_name
        configParser.read(configFilePath)    

        self.mail_from=configParser.get('smtp', 'email_from')
        self.smtp_password=configParser.get('smtp', 'email_password')
        self.smtp_host=configParser.get('smtp', 'smtp_host')
        self.smtp_port=configParser.get('smtp', 'smtp_port')

        print(self.mail_from)
        print(self.smtp_password)
        print(self.smtp_host)
        print(self.smtp_port)


def load_recipients_from_file(file_name):
    emails=[]
    with open(file_name) as f:
        content = f.readlines()
        emails = [x.strip() for x in content]
    return emails 

parser = argparse.ArgumentParser(description='Send email notification to users')
parser.add_argument('--recipients', '-r', metavar='recipients', required=True,
                   help='File that contains list of recipients in every line')
parser.add_argument('--subject', '-s', metavar='subject', required=True,
                   help='Message subject text')
parser.add_argument('--body', '-b', metavar='message_body', required=True,
                   help='Body of the message')
parser.add_argument('--config', '-c', metavar='smtp_configuration', required=True,
                   help='File containing SMTP configuration properties')

args = parser.parse_args()

recipients=load_recipients_from_file(args.recipients)
config=Configuration(args.config)

msg = EmailMessage()
msg.set_content(args.body)
msg['Subject'] = args.subject
msg['From'] = config.mail_from
msg['To'] = recipients

try:  
    server = smtplib.SMTP_SSL(config.smtp_host, int(config.smtp_port))
    server.ehlo()
    server.login(config.mail_from, config.smtp_password)
    server.send_message(msg)
    server.close()

    print('Email notification sent')
except Exception as e:  
    print('Error sending email...')
    print(e)
