package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/smtp"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"os"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
	"github.com/google/go-jsonnet"
	"github.com/kardianos/service"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed config-schema.json
var embedFS embed.FS

type Config struct {
	ImapAccounts map[string]ImapAccount `yaml:"imapAccounts"`
	SmtpAccounts map[string]SmtpAccount `yaml:"smtpAccounts"`
	Tasks        map[string]Task        `yaml:"tasks"`
}
type ImapAccount struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Server   string `yaml:"server"`
}
type SmtpAccount struct {
	Server string `yaml:"server"`
}
type Task struct {
	ImapAccount   string `yaml:"imapAccount"`
	SmtpAccount   string `yaml:"smtpAccount"`
	WatchFolder   string `yaml:"watchFolder"`
	SelectMessage string `yaml:"selectMessage"`
	EditCopy      string `yaml:"editCopy"`
	StoreCopyIn   string `yaml:"storeCopyIn"`
	ForwardCopyTo string `yaml:"forwardCopyTo"`
	DeleteMessage bool   `yaml:"deleteMessage"`
	RtTag         string `yaml:"rtTag"`
	_imapAccount  ImapAccount
	_smtpAccount  SmtpAccount
	_name         string
	_client       *client.Client
}

var sl service.Logger

type program struct{}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

var cfgPath = flag.String("config", "/etc/unspammer/config.yaml", "config file")
var jsonetDump = flag.Bool("jsonnetdump", false, "dump generated json to stdout")

func main() {
	svcFlag := flag.String("service", "", "control the system service")
	flag.Parse()
	options := make(service.KeyValue)
	options["Restart"] = "on-success"
	options["SuccessExitStatus"] = "1 2 8 SIGKILL"

	svcConfig := &service.Config{
		Name:        "UnSpammer",
		DisplayName: "UnSpammer the IMAP Robot",
		Description: "Monitor IMAP folders and process new messages",
		Dependencies: []string{
			"Requires=network.target",
			"After=network-online.target syslog.target"},
		Option:    options,
		Arguments: []string{"-config", *cfgPath},
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	errs := make(chan error, 5)
	sl, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			err := <-errs
			if err != nil {
				log.Print(err)
			}
		}
	}()

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}

	err = s.Run()
	if err != nil {
		sl.Error(err)
	}
}
func (p *program) run() {
	sl.Infof("Starting UnSpammer on %v.", service.Platform())
	// status files are stored alongside the config file ... so go there
	os.Chdir(filepath.Dir(*cfgPath))

	cfg, err := readConfig(*cfgPath)
	if err != nil {
		sl.Errorf("error reading config: %v", err)
		os.Exit(1)
	}
	done := make(chan bool)

	// setup signal catching
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		s := <-sigs
		sl.Infof("RECEIVED SIGNAL: %s", s)
		done <- true
	}()

	for taskName, task := range cfg.Tasks {
		imapAccount := cfg.ImapAccounts[task.ImapAccount]
		sl.Infof("%s: start task handler", taskName)

		go func(task Task) {
		loop:
			for {
				if task._client == nil {
					if err := getClient(&task); err != nil {
						sl.Infof("%s: error connecting to %s: %s\n", task._name, imapAccount.Server, err)
						task._client.Terminate()
						task._client = nil
						time.Sleep(time.Second * 15)
						continue loop
					}
				}
				// scan for messages
				select {
				case err := <-scanMailbox(task):
					if err != nil {
						sl.Infof("%s: error scanning messages: %v", task._name, err)
						task._client.Terminate()
						task._client = nil
						time.Sleep(time.Second * 15)
						continue loop
					}
				case <-time.After(1 * time.Minute):
					sl.Infof("%s: timeout scanning messages", task._name)
					task._client.Terminate()
					task._client = nil
					continue loop
				}
				// wait for folder change
				select {
				case err := <-watchFolder(task):
					if err != nil {
						sl.Infof("%s: error watch folder: %v", task._name, err)
						task._client.Terminate()
						task._client = nil
						time.Sleep(time.Second * 15)
						continue loop
					}
				case <-time.After(60 * time.Minute):
					sl.Infof("%s: timeout watch folder", task._name)
					task._client.Terminate()
					task._client = nil
					continue loop
				}

			}
		}(task)
	}
	<-done
	sl.Info("bye bye")
}

func readConfig(cfgFile string) (Config, error) {

	cfgString, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return Config{}, err
	}

	schemaText, _ := embedFS.ReadFile("config-schema.json")
	var m interface{}
	if strings.HasSuffix(cfgFile, ".yaml") {
		if err := yaml.Unmarshal(cfgString, &m); err != nil {
			return Config{}, err
		}
		m, _ = toStringKeys(m)
	}
	if strings.HasSuffix(cfgFile, ".json") {
		if err := json.Unmarshal(cfgString, &m); err != nil {
			return Config{}, err
		}
	}
	var jsonString string
	if strings.HasSuffix(cfgFile, ".jsonnet") {
		sl.Infof("compiling jsonnet %s", cfgFile)
		vm := jsonnet.MakeVM()
		var err error
		jsonString, err = vm.EvaluateAnonymousSnippet(
			cfgFile, string(cfgString))
		if err != nil {
			return Config{}, err
		}
		if *jsonetDump {
			sl.Infof("json: %s", jsonString)
		}
		if err := json.Unmarshal([]byte(jsonString), &m); err != nil {
			return Config{}, err
		}
	}

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

	if strings.HasSuffix(cfgFile, ".yaml") {
		if err := yaml.Unmarshal(cfgString, &config); err != nil {
			return Config{}, err
		}
	}
	if strings.HasSuffix(cfgFile, ".json") {
		if err := json.Unmarshal(cfgString, &config); err != nil {
			return Config{}, err
		}
	}
	if strings.HasSuffix(cfgFile, ".jsonnet") {
		if err := json.Unmarshal([]byte(jsonString), &config); err != nil {
			return Config{}, err
		}
	}

	for taskName := range config.Tasks {
		// make sure we have write access
		if task, ok := config.Tasks[taskName]; ok {
			task._name = taskName
			task._imapAccount = config.ImapAccounts[task.ImapAccount]
			task._smtpAccount = config.SmtpAccounts[task.SmtpAccount]
			if task.EditCopy == "rt-tag" {
				if _, err := getNextRtKey(task); err != nil {
					log.Fatalf("%s: %v", taskName, err)
				}
			}
			// write back the modified copy
			config.Tasks[taskName] = task
		}
	}
	return config, err
}

func getClient(task *Task) error {
	account := task._imapAccount
	sl.Infof("%s: connecting to %s", task._name, account.Server)
	c, err := client.DialTLS(account.Server, nil)
	if err != nil {
		return err
	}
	task._client = c
	if err := c.Login(account.Username, account.Password); err != nil {
		return err
	}
	sl.Infof("%s: connected", task._name)
	// when we're done, close the connection
	//defer c.Logout()

	return nil
}

func watchFolder(task Task) chan error {
	mailbox := task.WatchFolder
	c := task._client
	sl.Infof("%s: watch %s for changes", task._name, mailbox)
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
					sl.Infof("%s: change detected in %s", task._name, msg.Mailbox.Name)
					if !stopped {
						close(stopIdling)
						stopped = true

					}
				}
			case err := <-idlingStopped:
				sl.Infof("%s: idling stopped", task._name)
				c.Updates = nil
				errChan <- err
				return
			}
		}
	}()
	return errChan
}

func scanMailbox(task Task) chan error {
	c := task._client
	errChan := make(chan error)
	sl.Infof("%s: scanning %s", task._name, task.WatchFolder)
	go func() {
		defer close(errChan)
		mbox, err := c.Select(task.WatchFolder, false)
		if err != nil {
			errChan <- err
			return
		}
		from := uint32(1)
		to := mbox.Messages
		if to == 0 {
			sl.Infof("%s: no messages in %s", task._name, task.WatchFolder)
			errChan <- nil
			return
		}
		// never go for more than 10 messages at a time
		if to > 10 {
			from = mbox.Messages - 10
		}
		seqset := new(imap.SeqSet)
		seqset.AddRange(from, to)

		messages := make(chan *imap.Message, 10)
		doneFetching := make(chan error, 1)
		section := &imap.BodySectionName{}
		section.Peek = true // don't mark the message as read
		items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags, imap.FetchUid}
		go func() {
			doneFetching <- c.Fetch(seqset, items, messages)
		}()
	nextMessage:
		for msg := range messages {
			oldFlags := []string{}
			for _, flag := range msg.Flags {
				if flag == "usp-"+task._name {
					// skip messages which have already been processed
					continue nextMessage
				}
				if strings.HasPrefix(flag, "usp-") {
					oldFlags = append(oldFlags, flag)
				}
			}
			r := msg.GetBody(section)
			if r == nil {
				sl.Infof("%s: server didn't returned message body", task._name)
				continue
			}
			// parse the message header using  the mail reader
			mr, err := message.Read(r)
			if err != nil && !message.IsUnknownCharset(err) {
				sl.Infof("%s: reader trouble: %v", task._name, err)
				continue
			}

			header := mr.Header
			subject, _ := header.Text("Subject")
			switch task.SelectMessage {
			case "spam":
				if header.Get("X-Spam-Flag") != "YES" {
					continue
				}
			case "ham":
				if header.Get("X-Spam-Flag") == "YES" {
					continue
				}
			case "all":
				// do nothing
			}
			sl.Infof("%s: handling message: %s", task._name, subject)

			switch task.EditCopy {
			case "rt-tag":
				if err := addRtNumber(task, msg, mr); err != nil {
					sl.Infof("%s: error adding RT number: %v", task._name, err)
					continue
				}
			case "un-spam":
				removeSpamassassinHeaders(mr)

			case "no":
				// do nothing
			}

			if err := addMessageFlag(task, msg, "usp-"+task._name); err != nil {
				sl.Infof("%s: tagging problem: %v", task._name, err)
				continue
			}

			if task.ForwardCopyTo != "" {
				sl.Infof("%s: forwarding msg to %s\n", task._name, task.ForwardCopyTo)
				if err := forwardMessage(task, mr); err != nil {
					sl.Infof("%s: SMTP problem: %v", task._name, err)
					continue
				}
			}
			if task.StoreCopyIn != "" {
				sl.Infof("%s: storing unspammed msg in: %s\n", task._name, task.StoreCopyIn)
				if err := saveMessage(task, mr, oldFlags); err != nil {
					sl.Infof("%s: storing issue: %v", task._name, err)
					continue
				}
			}
			if task.DeleteMessage {
				sl.Infof("%s: removing message ...", task._name)
				if err := deleteMsg(task, msg.Uid); err != nil {
					sl.Infof("%s: expunge problem: %v", task._name, err)
					continue
				}
			}
		}
		errChan <- <-doneFetching
		sl.Infof("%s: scanning complete", task._name)
	}()
	return errChan
}

func addMessageFlag(task Task, msg *imap.Message, flag string) error {
	c := task._client
	seqset := new(imap.SeqSet)
	seqset.AddNum(msg.Uid)
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{flag}
	sl.Infof("%s: adding flag %s to message %d", task._name, flag, msg.Uid)
	return c.UidStore(seqset, item, flags, nil)
}

func saveMessage(task Task, mr *message.Entity, oldFlags []string) error {
	// Append it to INBOX, with two flags
	flags := []string{"usp-" + task._name}
	flags = append(flags, oldFlags...)
	c := task._client
	mailbox := task.StoreCopyIn
	rawMessage := makeRawMessage(mr)
	return c.Append(mailbox, flags, time.Now(), rawMessage)
}

func forwardMessage(task Task, mr *message.Entity) error {
	returnPath := mr.Header.Get("Return-Path")
	mr.Header.Del("Return-Path")
	rawMessage := makeRawMessage(mr)
	return smtp.SendMail(task._smtpAccount.Server, nil, returnPath, []string{task.ForwardCopyTo}, rawMessage.Bytes())
}

func getNextRtKey(task Task) (string, error) {
	rtCounterFile := fmt.Sprintf("%s.cnt", task._name)
	lastKey, err := os.ReadFile(rtCounterFile)
	thisYear := strconv.Itoa(time.Now().Year() - 2000)

	if err != nil {
		return "", err
	}
	lastKeySplit := strings.Split(string(lastKey), "-")
	lastCntInt, err := strconv.Atoi(strings.TrimSpace(lastKeySplit[1]))
	if err != nil {
		return "", err
	}
	lastYear := strings.TrimSpace(lastKeySplit[0])
	if lastYear == thisYear {
		return thisYear + "-" + strconv.Itoa(lastCntInt+1), nil
	}
	return thisYear + "-1", nil
}

func addRtNumber(task Task, msg *imap.Message, mr *message.Entity) error {
	subject := mr.Header.Get("Subject")
	tag := task.RtTag
	if tag == "" {
		tag = "UnSpammer"
	}
	if strings.Contains(subject, "["+tag+" -") {
		sl.Infof("%s: subject already tagged: %s", task._name, subject)
		return nil
	}
	sl.Infof("%s: adding RT number", task._name)
	rtCounterFile := fmt.Sprintf("%s.cnt", task._name)
	nextRtNumber, err := getNextRtKey(task)
	if err != nil {
		return err
	}

	mr.Header.Set("Subject", fmt.Sprintf("[%s - %s] %s", tag, nextRtNumber, subject))
	sl.Infof("%s: subject tagged with %s", task._name, nextRtNumber)

	newPath := fmt.Sprintf("%s.%d", rtCounterFile, rand.Uint64())
	if err := os.WriteFile(newPath,
		[]byte(nextRtNumber), 0644); err != nil {
		return err
	}
	return os.Rename(newPath, rtCounterFile)
}

func removeSpamassassinHeaders(msg *message.Entity) {
	header := msg.Header
	header.Del("Subject")
	header.Set("Subject", header.Get("X-Spam-Prev-Subject"))
	for _, x := range []string{
		"X-Spam-Prev-Subject",
		"X-Spam-Flag",
		"X-Spam-Status",
		"X-Spam-Level",
		"X-Spam-Score",
		"X-Spam-Tests",
		"X-Spam-Report",
		"X-Spam-Checker-Version",
	} {
		header.Del(x)
	}
	header.Set("X-Unspammer-Blessing", "BLESSED")
}

func makeRawMessage(mr *message.Entity) *bytes.Buffer {
	rawMessage := new(bytes.Buffer)
	textproto.WriteHeader(rawMessage, mr.Header.Header)
	io.Copy(rawMessage, mr.Body)
	return rawMessage
}

func deleteMsg(task Task, msgUid uint32) error {
	c := task._client
	seqset := new(imap.SeqSet)
	seqset.AddNum(msgUid)
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	sl.Infof("%s: deleting message %d", task._name, msgUid)
	if err := c.UidStore(seqset, item, flags, nil); err != nil {
		return err
	}
	return c.Expunge(nil)
}

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
