# unspammer

## Sample Config File

```yaml
imapAccounts:
  tobi:
    username: "tobi@xxx.ch"
    password: "asdfasdf"
    server: "aaa.xxx.ch:993"

smtpAccounts:
  smtp:
    server: "bbb.xxx.ch:25"

tasks:
  unspam:
    imapAccount: tobi
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
    imapAccount: tobi
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
 ```
