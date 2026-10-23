# unspammer, the IMAP robot

[![CI](https://github.com/oposs/unspammer/actions/workflows/ci.yml/badge.svg)](https://github.com/oposs/unspammer/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/oposs/unspammer)](https://github.com/oposs/unspammer/releases/latest)

unspammer watches IMAP folders and acts on the mail that arrives in them.
For each new message it can:

* remove the SpamAssassin headers from mail that was tagged as spam by mistake
* store an edited copy in another folder
* forward the message to one or more addresses
* add a running number to the subject, to serve as a ticket number

One unspammer process serves several IMAP accounts and folders in parallel.
The configuration is YAML, JSON or Jsonnet.

Release archives exist for Linux (amd64, arm64), macOS (Intel, Apple Silicon), Windows (amd64, arm64) and illumos.

## Install

Debian and Ubuntu:

```sh
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl -fsSL -o /etc/apt/keyrings/gitea-oposs.asc https://gitea.oetiker.ch/api/packages/oposs/debian/repository.key
echo "deb [signed-by=/etc/apt/keyrings/gitea-oposs.asc] https://gitea.oetiker.ch/api/packages/oposs/debian stable main" | sudo tee /etc/apt/sources.list.d/oposs.list
sudo apt update && sudo apt install unspammer
```

Fedora, RHEL, Rocky and Alma, with dnf4 and dnf5 alike:

```sh
sudo curl -fsSL -o /etc/yum.repos.d/oposs.repo https://gitea.oetiker.ch/api/packages/oposs/rpm.repo
sudo dnf install unspammer
```

Homebrew:

```sh
brew tap oposs/unspammer https://github.com/oposs/unspammer
brew trust --formula oposs/unspammer/unspammer
brew install unspammer
```

`brew trust` exists from Homebrew 6 on; skip it on older versions.

mise:

```sh
mise use -g github:oposs/unspammer
```

Direct download: <https://github.com/oposs/unspammer/releases/latest>

## Run as a service

```sh
unspammer -config /etc/unspammer/config.yaml -service install
unspammer -service start
```

## Options

- `-config <file>`: The configuration file. The default is `/etc/unspammer/config.yaml`.
- `-jsonnetdump`: Print the JSON that a Jsonnet configuration file produces to standard output.
- `-service <action>`: Control the system service: `install`, `uninstall`, `start`, `stop` or `restart`.
- `-version`: Print the version and exit.

The manual, [docs/manual.md](docs/manual.md), describes every configuration key.

## Sample configuration

```yaml
imapAccounts:
  support:
    username: "support@example.com"
    password: "secret"
    server: "imap.example.com:993"

smtpAccounts:
  smtp:
    server: "smtp.example.com:25"

tasks:
  unspam:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: INBOX/DeSpamMe
    # act only on mail SpamAssassin flagged as spam
    selectMessage: spam
    # remove the SpamAssassin headers
    editCopy: un-spam
    # store the copy in INBOX/UnSpammed
    storeCopyIn: INBOX/UnSpammed
    # delete the original message
    deleteMessage: true
  ticket:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: INBOX/UnSpammed
    selectMessage: ham
    # add a running number to the subject; the count is kept in ticket.cnt
    editCopy: rt-tag
    forwardCopiesTo:
      - rt@example.com
    deleteMessage: true
  forward:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: INBOX
    selectMessage: ham
    editCopy: "no"
    forwardCopiesTo:
      - helpdesk@example.com
    deleteMessage: true
```
