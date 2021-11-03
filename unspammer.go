package main

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/smtp"

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
	SmtpServer string `yaml:"smtpServer"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	Recipient  string `yaml:"recipient"`
	ImapServer string `yaml:"imapServer"`
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
				if err := <-idlyWaitForMail(c, "INBOX"); err != nil {
					log.Printf("Error waiting for mail: %#v\n", err)
					c = nil
					continue
				}
				if err := <-moveMail(c, "INBOX", account); err != nil {
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

func moveMail(c *client.Client, mailbox string, account Account) chan error {
	errChan := make(chan error)
	log.Println("Moving mail...")
	go func() {
		defer close(errChan)
		mbox, err := c.Select(mailbox, false)
		log.Println("Selected mailbox")
		if err != nil {
			errChan <- err
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
		items := []imap.FetchItem{section.FetchItem()}
		go func() {
			doneFetching <- c.Fetch(seqset, items, messages)
		}()

		for msg := range messages {
			//log.Printf("Message: %#v\n", msg)

			r := msg.GetBody(section)
			if r == nil {
				log.Println("Server didn't returned message body")
				continue
			}
			// Create a new mail reader
			mr, err := message.Read(r)
			if err != nil && !message.IsUnknownCharset(err) {
				log.Println("Reader trouble: ", err)
				continue
			}

			// Print some info about the message
			header := mr.Header
			if header.Get("X-Spam-Flag") == "YES" {
				log.Printf("cleaning spam message: %s\n", header.Get("Subject"))
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
				log.Println("re-submitting ...")
				if err := smtp.SendMail(account.SmtpServer, nil, returnPath, []string{account.Recipient}, rawMessage.Bytes()); err != nil {
					log.Println("SMTP problem: ", err)
					continue
				}

				seqset := new(imap.SeqSet)
				seqset.AddNum(msg.SeqNum)
				item := imap.FormatFlagsOp(imap.AddFlags, true)
				flags := []interface{}{imap.DeletedFlag}
				if err := c.Store(seqset, item, flags, nil); err != nil {
					log.Println("delete problem: ", err)
					continue
				}
				// Then delete it
				log.Println("removing message ...")
				if err := c.Expunge(nil); err != nil {
					log.Println("expunge problem: ", err)
					continue
				}
				//os.Exit(1)
			}
		}

		if err := <-doneFetching; err != nil {
			errChan <- err
		}
		errChan <- nil
	}()
	return errChan
}

// 		messages := make(chan *imap.Message, 10)
// 	done = make(chan error, 1)
// 		for {

// 			if err := c.Move(c.SelectedMailbox(), "INBOX.spam"); err != nil {
// 				errChan <- err
// 				return
// 			}
// 		}
// 	}()
// 	return errChan

// 		// Connect to server
// 		c, err := client.DialTLS(account.Server, nil)
// 		if err != nil {
// 			log.Fatal(err)
// 			os.Exit(1)
// 		}
// 		log.Printf("Connected %s", account.Server)
// 		// Don't forget to logout
// 		defer c.Logout()
// 		// Login
// 		if err := c.Login(account.Username, account.Password); err != nil {
// 			log.Fatal(err)
// 			os.Exit(1)
// 		}
// 		log.Println("Logged in")
// 		// Select a mailbox
// 		if _, err := c.Select("INBOX", false); err != nil {
// 			log.Fatal(err)
// 		}

// 		// Create a channel to receive mailbox updates
// 		updates := make(chan client.Update)
// 		c.Updates = updates

// 		stop := make(chan struct{})
// 		done := make(chan error, 1)
// 		// Start idling
// 		go func() {
// 			done <- c.Idle(stop, nil)
// 		}()
// 		// Listen for updates

// 		stopped := false
// 		for {
// 			select {
// 			case update := <-updates:
// 				switch msg := update.(type) {
// 				case *client.MailboxUpdate:
// 					log.Printf("MailboxUpdate: %#v", msg.Mailbox.Name)
// 					if !stopped {
// 						close(stop)
// 						stopped = true
// 					}
// 				case *client.MessageUpdate:
// 					log.Printf("MessageUpdate: %#v", msg.Message)
// 				case *client.ExpungeUpdate:
// 					log.Printf("ExpungeUpdate: %#v", msg.SeqNum)
// 				}
// 				// if we want to stop after one round
// 				// if !stopped {
// 				// 	close(stop)
// 				// 	stopped = true
// 				// }
// 			case err := <-done:
// 				if err != nil {
// 					log.Fatal(err)
// 				}
// 				log.Println("Stopped Ideling")
// 			}
// 		}
// 	}
// }

// // Start idling
// done := make(chan error, 1)
// 		// List mailboxes
// 	mailboxes := make(chan *imap.MailboxInfo, 10)
// 	done := make(chan error, 1)
// 	go func() {
// 		done <- c.List("", "*", mailboxes)
// 	}()

// 	log.Println("Mailboxes:")
// 	for m := range mailboxes {
// 		log.Println("* " + m.Name)
// 	}

// 	if err := <-done; err != nil {
// 		log.Fatal(err)
// 	}

// 	// Select INBOX
// 	mbox, err := c.Select("INBOX", false)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	log.Println("Flags for INBOX:", mbox.Flags)

// 	// Get the last 4 messages
// 	from := uint32(1)
// 	to := mbox.Messages
// 	if mbox.Messages > 3 {
// 		// We're using unsigned integers here, only subtract if the result is > 0
// 		from = mbox.Messages - 3
// 	}
// 	seqset := new(imap.SeqSet)
// 	seqset.AddRange(from, to)

// 	messages := make(chan *imap.Message, 10)
// 	done = make(chan error, 1)

// 	go func() {
// 		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
// 	}()

// 	log.Println("Last 4 messages:")
// 	for msg := range messages {
// 		log.Println("* " + msg.Envelope.Subject)
// 	}

// 	if err := <-done; err != nil {
// 		log.Fatal(err)
// 	}

// 	log.Println("Done!")
// }
