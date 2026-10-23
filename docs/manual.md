---
title: UNSPAMMER
section: 8
header: unspammer manual
footer: unspammer
date: 2026-10-21
---

# NAME

unspammer - watch IMAP folders and act on the mail that arrives in them

# SYNOPSIS

**unspammer** [**-config** *file*] [**-jsonnetdump**]

**unspammer** [**-config** *file*] **-service** *action*

**unspammer** **-version**

# DESCRIPTION

**unspammer** logs in to one or more IMAP accounts and watches one folder per task.
When mail arrives in a watched folder, the task selects messages by their SpamAssassin flag,
edits a copy,
stores the copy in another folder or forwards it by SMTP,
and deletes the original when configured to.

Each task runs in parallel with the others.
After a connection error the task waits 15 seconds and connects again.
The task also reconnects when a folder scan takes longer than one minute,
and after 60 minutes without a change in the watched folder.

Without **-service**, **unspammer** runs in the foreground until it receives **SIGINT**.

# CONFIGURATION

The configuration file is YAML, JSON or Jsonnet.
The file name extension selects the format: `.yaml`, `.json` or `.jsonnet`.
**unspammer** checks the file against its built-in JSON schema at startup and exits on an error.

The file has three keys.
Account and task names start with a lower-case letter, followed by lower-case letters, digits, `-` or `_`.

- `imapAccounts`: A map of IMAP accounts. Each has `username`, `password` and `server` (`host:port`). The connection uses TLS.
- `smtpAccounts`: A map of SMTP servers. Each has `server` (`host:port`).
- `tasks`: A map of tasks, with the keys below.

Keys of a task:

- `imapAccount`: Name of the IMAP account. Required.
- `smtpAccount`: Name of the SMTP server used for forwarding. Required.
- `watchFolder`: The IMAP folder to watch. Required.
- `selectMessage`: Which messages to act on. `spam` selects messages with `X-Spam-Flag: YES`, `ham` selects all others, `all` selects every message. Required.
- `editCopy`: How to edit the copy. `un-spam` restores the subject from `X-Spam-Prev-Subject` and removes the SpamAssassin headers. `rt-tag` puts a running number in front of the subject. `no` leaves the copy as it is. Required.
- `rtTag`: The tag `rt-tag` writes into the subject, as `[`*tag*` - `*YY*`-`*N*`] `*subject*. A subject that already contains `[`*tag* is not tagged again. The default is `UnSpammer`.
- `storeCopyIn`: Folder to store the edited copy in.
- `forwardCopiesTo`: List of addresses to forward the edited copy to. The envelope sender is the `Return-Path` of the original message.
- `deleteMessage`: Delete the original message after it is handled. Required.

Example:

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
    selectMessage: spam
    editCopy: un-spam
    storeCopyIn: INBOX/UnSpammed
    deleteMessage: true
  ticket:
    imapAccount: support
    smtpAccount: smtp
    watchFolder: INBOX/UnSpammed
    selectMessage: ham
    editCopy: rt-tag
    forwardCopiesTo:
      - rt@example.com
    deleteMessage: true
```

# OPTIONS

- `-config <file>`: The configuration file. The default is `/etc/unspammer/config.yaml`.
- `-jsonnetdump`: Print the JSON that a Jsonnet configuration file produces to standard output.
- `-service <action>`: Control the system service and exit. The actions are `install`, `uninstall`, `start`, `stop` and `restart`. `install` registers the service with the **-config** given on the same command line.
- `-version`: Print the version and exit.

# EXIT STATUS

**unspammer** exits with 1 when the configuration file cannot be read or does not match the schema,
and when a **-service** action fails.

# FILES

- `/etc/unspammer/config.yaml`: The default configuration file.
- `<task>.cnt`: The last running number of a task with `editCopy: rt-tag`, in the directory of the configuration file. The file holds *YY*`-`*N*, for example `26-0`. When the file is missing, the count starts at 1. **unspammer** exits at startup when the file holds anything else than *YY*`-`*N*. The number restarts at 1 in a new year.

# NOTES

The SMTP connection uses STARTTLS when the server offers it, without verifying the server certificate.

# AUTHORS

Tobias Oetiker <tobi@oetiker.ch>,
Manuel Oetiker <manuel@oetiker.ch>,
Claude (Anthropic).

# REPORTING BUGS

<https://github.com/oposs/unspammer/issues>

# COPYRIGHT

Copyright (c) 2021-2026 OETIKER+PARTNER AG.
Released under the GNU General Public License version 3, see
<https://github.com/oposs/unspammer/blob/main/LICENSE>.

# SEE ALSO

Project home: <https://github.com/oposs/unspammer>

**spamassassin**(1)
