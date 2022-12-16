# unspammer

Unspammer watches IMAP folders of your choice and acts on mail arriving in these folders.

Unspammer can do the following:

* remove SpamAssassin headers from mail, accidentely tagged as spam
* forward mail to another address
* add a running counter to the subject of the mail to act as a ticket number

To install unspammer as a service, run `unspammer -config path/to/config.yaml -service install`
## Sample Config File

Config formats supported: `yaml`, `json` and `jsonnet`

```yaml
imapAccounts:
  support:
    username: "tobi@xxx.ch"
    password: "asdfasdf"
    server: "aaa.xxx.ch:993"

smtpAccounts:
  smtp:
    server: "bbb.xxx.ch:25"

tasks:
  unspam:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: INBOX/DeSpamMe
    # we only act on SpamAssassin detected spam
    selectMessage: spam
    # remove SpamAssassin Headers
    editCopy: un-spam
    # save copy into folder UnSpammed
    storeCopyIn: INBOX/UnSpammed
    # delete original message
    deleteMessage: true
  minrt:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: INBOX/UnSpammed
    # we only act on non-spam
    selectMessage: ham
    # we add a running counter to the subject
    editCopy: rt-tag
    # forward the mail
    forwardCopyTo: somewhere@other.xxx
    # remove the original
    deleteMessage: true
   forward:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: Inbox
    # we only act on non-spam
    selectMessage: ham
    # no message editing
    editCopy: no
    # forward the mail
    forwardCopyTo: rt@cloud-mail.oetiker.ch    
    # remove the original
    deleteMessage: true
 ```
