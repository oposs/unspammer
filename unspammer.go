package main

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/smtp"
	"time"

	"os"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed config-schema.json
var embedFS embed.FS

// make sure all keys are strings, since this is not necessarily the case
// in yaml files
func toStringKeys(val interface{}) (interface{}, error) {
	switch val := val.(type) {
	case map[interface{}]interface{}:
		hash := make(map[string]interface{})
		for k, v := range val {
			k, ok := k.(string)
			if !ok {
				return nil, errors.New("found non-string key")
			}
			var err error
			hash[k], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return hash, nil
	case []interface{}:
		var err error
		list := make([]interface{}, len(val))
		for i, v := range val {
			list[i], err = toStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return list, nil
	default:
		return val, nil
	}
}

type Config struct {
	Accounts []Account `yaml:"accounts"`
}

type Account struct {
	SmtpServer     string `yaml:"smtpServer"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	ImapServer     string `yaml:"imapServer"`
	Inbox          string `yaml:"inbox"`
	DeleteOriginal bool   `yaml:"deleteOriginal"`
	Outbox         string `yaml:"outbox"`    // optional
	ForwardTo      string `yaml:"forwardTo"` // optional
}

func readConfig(cfgFile string) (Config, error) {
	yamlText, _ := ioutil.ReadFile(cfgFile)
	schemaText, _ := embedFS.ReadFile("config-schema.json")
	var m interface{}
	if err := yaml.Unmarshal(yamlText, &m); err != nil {
		return Config{}, err
	}
	m, _ = toStringKeys(m)

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json",
		strings.NewReader(string(schemaText))); err != nil {
		return Config{}, err
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return Config{}, err
	}

	if err := schema.Validate(m); err != nil {
		return Config{}, err
	}
	config := Config{}
	err = yaml.Unmarshal(yamlText, &config)
	return config, err
}

func main() {
	cfg, err := readConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error parsing config.yaml\n%#v", err)
		os.Exit(1)
	}
	log.Println("Connecting to server...")
	for _, account := range cfg.Accounts {

		log.Printf("Connected to %s\n", account.ImapServer)
		done := make(chan int)
		go func() {
			var c *client.Client
			for {
				if c == nil {
					var err error
					c, err = getClient(account)
					if err != nil {
						log.Printf("Error connecting to %s: %s\n", account.ImapServer, err)
						continue
					}
				}
				if err := <-idlyWaitForMail(c, account.Inbox); err != nil {
					log.Printf("Error waiting for mail: %#v\n", err)
					c = nil
					continue
				}
				if err := <-scanMailbox(c, account); err != nil {
					log.Printf("Error moving mail: %#v\n", err)
					c = nil
					continue
				}
			}
		}()
		<-done
	}
}

func getClient(account Account) (*client.Client, error) {
	c, err := client.DialTLS(account.ImapServer, nil)
	if err != nil {
		return nil, err
	}
	if err := c.Login(account.Username, account.Password); err != nil {
		return nil, err
	}
	// when we're done, close the connection
	//defer c.Logout()

	return c, nil
}

func idlyWaitForMail(c *client.Client, mailbox string) chan error {
	errChan := make(chan error)
	go func() {
		defer close(errChan)
		if _, err := c.Select(mailbox, false); err != nil {
			errChan <- err
		}
		// Create a channel to receive mailbox updates
		updates := make(chan client.Update)
		c.Updates = updates
		stopIdling := make(chan struct{})    // close this to stop idling
		idlingStopped := make(chan error, 1) // get notified when idling has stopped
		// Start idling
		go func() {
			idlingStopped <- c.Idle(stopIdling, nil)
		}()
		stopped := false

		for {
			select {
			case update := <-updates:
				switch msg := update.(type) {
				case *client.MailboxUpdate:
					log.Printf("MailboxUpdate: %#v", msg.Mailbox.Name)
					if !stopped {
						close(stopIdling)
						stopped = true

					}
				}
			case err := <-idlingStopped:
				log.Println("idling stopped")
				c.Updates = nil
				errChan <- err
				return
			}
		}
	}()
	return errChan
}

func scanMailbox(c *client.Client, account Account) chan error {
	errChan := make(chan error)
	log.Printf("scanning %s", account.Inbox)
	go func() {
		defer close(errChan)
		mbox, err := c.Select(account.Inbox, false)
		if err != nil {
			errChan <- err
			return
		}
		from := uint32(1)
		to := mbox.Messages
		// never go for more than 10 messages at a time
		if mbox.Messages > 10 {
			from = mbox.Messages - 10
		}
		seqset := new(imap.SeqSet)
		seqset.AddRange(from, to)

		messages := make(chan *imap.Message, 10)
		doneFetching := make(chan error, 1)
		section := &imap.BodySectionName{}
		section.Peek = true // don't mark the message as read
		items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags}
		go func() {
			doneFetching <- c.Fetch(seqset, items, messages)
		}()
	nextMessage:
		for msg := range messages {
			for _, flag := range msg.Flags {
				if flag == "unspammer-processed" {
					log.Println("skip message - already processed")
					continue nextMessage
				}
			}
			r := msg.GetBody(section)
			if r == nil {
				log.Println("Server didn't returned message body")
				continue
			}
			// parse the message header using  the mail reader
			mr, err := message.Read(r)
			if err != nil && !message.IsUnknownCharset(err) {
				log.Println("Reader trouble: ", err)
				continue
			}

			header := mr.Header
			if header.Get("X-Spam-Flag") != "YES" {
				continue
			}
			subject, _ := header.Text("Subject")
			log.Printf("cleaning spam message: %s\n", subject)
			seqset := new(imap.SeqSet)
			seqset.AddNum(msg.SeqNum)
			item := imap.FormatFlagsOp(imap.AddFlags, true)
			flags := []interface{}{"UnSpammer-Processed"}
			if err := c.Store(seqset, item, flags, nil); err != nil {
				log.Println("tagging problem: ", err)
				continue
			}
			rawMessage := new(bytes.Buffer)
			header.Del("Subject")
			header.Set("Subject", header.Get("X-Spam-Prev-Subject"))
			header.Del("X-Spam-Prev-Subject")
			header.Del("X-Spam-Flag")
			header.Del("X-Spam-Status")
			header.Del("X-Spam-Level")
			header.Del("X-Spam-Checker-Version")
			header.Del("X-Spam-Report")
			header.Set("X-Unspammer-Blessing", "BLESSED")
			returnPath := header.Get("Return-Path")
			header.Del("Return-Path")
			textproto.WriteHeader(rawMessage, header.Header)
			io.Copy(rawMessage, mr.Body)

			if account.ForwardTo != "" {
				log.Printf("forwarding unspammed msg to %s\n", account.ForwardTo)
				if err := smtp.SendMail(account.SmtpServer, nil, returnPath, []string{account.ForwardTo}, rawMessage.Bytes()); err != nil {
					log.Println("SMTP problem: ", err)
					continue
				}
			}
			if account.Outbox != "" {
				log.Printf("storing unspammend msg in: %s\n", account.Outbox)
				// Append it to INBOX, with two flags
				flags := []string{"UnSpammer-Generated"}
				if err := c.Append(account.Outbox, flags, time.Now(), rawMessage); err != nil {
					log.Println("Storing issue: ", err)
					continue
				}
			}
			if account.DeleteOriginal {
				log.Println("removing spam message ...")
				seqset := new(imap.SeqSet)
				seqset.AddNum(msg.SeqNum)
				item := imap.FormatFlagsOp(imap.AddFlags, true)
				flags := []interface{}{imap.DeletedFlag}
				if err := c.Store(seqset, item, flags, nil); err != nil {
					log.Println("delete problem: ", err)
					continue
				}

				if err := c.Expunge(nil); err != nil {
					log.Println("expunge problem: ", err)
					continue
				}
			}
		}

		if err := <-doneFetching; err != nil {
			errChan <- err
		}
		errChan <- nil
	}()
	return errChan
}
