# SCION coordinator systemd service

This readme explains necessary steps required for setting up scion coordinator.

## Configuring emailer:

SCION-coord systemd service is configured to send emails on every failure. To do so it uses python script `emailer.py`, which is being copied to `/usr/local/bin` as part of installation procedure. Few additional steps are required to make this work:

### Configuring email credentials

By default `unit-status-mail@.service` is looking at `/home/USER/.config/scion-coord/email.conf` for email configuration. It is necessary to create this file (or change file location in `unit-status-mail@.service` to different file) to be able to send emails. File has following structure:

```
[smtp]

email_from=<FROM_EMAIL>
email_password=<EMAIL_SMTP_PASS>
smtp_host=<SMTP_HOST>
smtp_port=<SMTP_PORT>
```

You may encounter problems if using Gmail as the provider. Refer to e.g. https://stackoverflow.com/a/27515833 for a workaround.

### Configuring recipients

By default `unit-status-mail@.service` uses `/home/USER/.config/scion-coord/recipients.txt` as a file that contains email addresses that will receive email on failure. This file has to be created or appropriate argument in `unit-status-mail@.service` must be changed to point to different file.
File should contain list of email addresses, each in new line. No comments are allowed. Example file could look like this:

```
john.doe@example.com
user@website.com
```

