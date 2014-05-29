package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/mxk/go-imap/imap"
	flag "github.com/ogier/pflag"
)

type MailClient struct {
	host     string
	port     uint
	ssl      bool
	username string
	password string
	client   *imap.Client
}

func (mc *MailClient) addr() string {
	return fmt.Sprintf("%s:%d", mc.host, mc.port)
}

func (mc *MailClient) connect() error {
	var err error

	if mc.port == 993 || mc.ssl == true {
		mc.client, err = imap.DialTLS(mc.addr(), &tls.Config{})
	} else {
		mc.client, err = imap.Dial(mc.addr())
	}

	if err != nil {
		return fmt.Errorf("IMAP dial error! ", err)
	}

	if mc.client.Caps["STARTTLS"] {
		_, err = imap.Wait(mc.client.StartTLS(nil))
	}

	if err != nil {
		return fmt.Errorf("Could not stablish TLS encrypted connection. ", err)
	}

	if mc.client.Caps["ID"] {
		_, err = imap.Wait(mc.client.ID("name", "go-postman"))
	}

	mc.client.SetLogMask(imap.LogConn)
	_, err = imap.Wait(mc.client.Login(mc.username, mc.password))
	if err != nil {
		return fmt.Errorf("IMAP authentication failed! Invalid credentials.")
	}
	mc.client.SetLogMask(imap.DefaultLogMask)

	return err
}

func (mc *MailClient) disconnect() {
	imap.Wait(mc.client.Logout(30 * time.Second))
	mc.client.Close(true)
}

func (mc *MailClient) selectMailbox(mailbox string) error {
	_, err := imap.Wait(mc.client.Select(mailbox, false))

	if err != nil {
		return fmt.Errorf("Failed to switch to mailbox %s", mailbox)
	}

	return err
}

func (mc *MailClient) query(arguments ...string) ([]uint32, error) {
	args := []imap.Field{}
	for _, a := range arguments {
		args = append(args, a)
	}

	cmd, err := imap.Wait(mc.client.Search(args...))
	if err != nil {
		return nil, fmt.Errorf("An error ocurred while searching for messages. ", err)
	}

	return cmd.Data[0].SearchResults(), nil
}

func (mc *MailClient) messagesForIds(ids []uint32) ([]string, error) {
	messages := []string{}

	if len(ids) > 0 {
		set, _ := imap.NewSeqSet("")
		set.AddNum(ids...)

		cmd, err := imap.Wait(mc.client.Fetch(set, "RFC822"))
		if err != nil {
			return messages, fmt.Errorf("An error ocurred while fetching unread messages data. ", err)
		}

		for _, msg := range cmd.Data {
			attrs := msg.MessageInfo().Attrs
			messages = append(messages, imap.AsString(attrs["RFC822"]))
		}
	}

	return messages, nil
}

func (mc *MailClient) unseenMessages() (messages []string, err error) {
	var ids []uint32

	ids, err = mc.query("UNSEEN")
	if err != nil {
		return messages, err
	}

	messages, err = mc.messagesForIds(ids)
	if err != nil {
		return messages, err
	}

	return messages, err
}

func (mc *MailClient) waitForIncoming() (err error) {
	_, err = mc.client.Idle()
	if err != nil {
		return fmt.Errorf("Could not start IDLE process. ", err)
	}

	err = mc.client.Recv(29 * time.Minute)
	if err != nil && err != imap.ErrTimeout {
		return fmt.Errorf("Some error ocurred while IDLING: %q", err)
	}

	_, err = imap.Wait(mc.client.IdleTerm())
	if err != nil {
		return fmt.Errorf("IDLE command termination failed by some reason. ", err)
	}

	return err
}

func (mc *MailClient) incomingMessages() (messages []string, err error) {
	err = mc.waitForIncoming()
	if err != nil {
		return messages, err
	}

	ids := []uint32{}
	for _, resp := range mc.client.Data {
		switch resp.Label {
		case "EXISTS":
			ids = append(ids, imap.AsNumber(resp.Fields[0]))
		}
	}

	mc.client.Data = nil

	messages, err = mc.messagesForIds(ids)
	if err != nil {
		return messages, err
	}

	return messages, err
}

func NewMailClient(host string, port uint, ssl bool, username string, password string) *MailClient {
	return &MailClient{
		host:     host,
		port:     port,
		ssl:      ssl,
		username: username,
		password: password}
}

type MessageHandler interface {
	Deliver(message string) error
}

type PostBackHandler struct {
	url          string
	encodeOnPost bool
}

func (hnd *PostBackHandler) getPostBody(data string) string {
	if hnd.encodeOnPost == true {
		return url.QueryEscape(data)
	}

	return data
}

func (hnd *PostBackHandler) Deliver(message string) error {
	buff := strings.NewReader(hnd.getPostBody(message))

	_, err := http.Post(hnd.url, "text/plain", buff)
	if err != nil {
		return fmt.Errorf("An error ocurred delivering a message. %q", err)
	}

	return nil
}

func NewPostBackHandler(postUrl string, encodeOnPost bool) *PostBackHandler {
	return &PostBackHandler{url: postUrl, encodeOnPost: encodeOnPost}
}

type LoggerHandler struct {
	logger *log.Logger
}

func (hnd *LoggerHandler) Deliver(message string) error {
	hnd.logger.Printf("Message:\n%q", message)

	return nil
}

func NewLoggerHandler(out *log.Logger) *LoggerHandler {
	return &LoggerHandler{logger: out}
}

type Watch struct {
	mailbox  string
	handlers []MessageHandler
	client   *MailClient
	logger   *log.Logger
	chMsgs   chan []string
}

func (w *Watch) Mailbox() string {
	return w.mailbox
}

func (w *Watch) SetMailbox(value string) {
	w.mailbox = value
}

func (w *Watch) SetLogger(logger *log.Logger) {
	w.logger = logger
}

func (w *Watch) Logger() *log.Logger {
	return w.logger
}

func (w *Watch) AddHandler(handler MessageHandler) {
	w.handlers = append(w.handlers, handler)
}

func (w *Watch) Handlers() []MessageHandler {
	return w.handlers
}

func (w *Watch) Run() {
	w.chMsgs = make(chan []string)

	go w.handleIncoming()

	err := w.monitorMailbox()
	if err != nil {
		w.logger.Fatalln(err)
	}
}

func (w *Watch) handleIncoming() {
	for {
		messages := <-w.chMsgs

		for _, msg := range messages {
			for _, handler := range w.handlers {
				handler.Deliver(msg)
			}
		}
	}
}

func (w *Watch) monitorMailbox() error {
	var messages []string
	var err error

	w.logger.Printf("Intiating connection to %s", w.client.addr())
	err = w.client.connect()
	if err != nil {
		return err
	}

	defer w.client.disconnect()

	w.logger.Printf("Switching to %s", w.mailbox)
	err = w.client.selectMailbox(w.mailbox)
	if err != nil {
		return err
	}

	w.logger.Printf("Checking for new (unseen) messages")
	messages, err = w.client.unseenMessages()
	if err != nil {
		return err
	}

	if len(messages) != 0 {
		w.logger.Printf("Detected %d new (unseen) messages. Delivering...", len(messages))
		w.chMsgs <- messages
	}

	for {
		w.logger.Printf("Waiting for new messages")
		messages, err = w.client.incomingMessages()
		if err != nil {
			return err
		}

		if len(messages) != 0 {
			w.logger.Printf("Detected %d new (unseen) messages. Delivering...", len(messages))
			w.chMsgs <- messages
		}
	}

	return nil
}

type WatchParams struct {
	Host            string
	Port            uint
	Ssl             bool
	Username        string
	Password        string
	Mailbox         string
	DeliveryUrl     string
	UrlEncodeOnPost bool
}

func NewWatchParams() *WatchParams {
	return &WatchParams{}
}

func NewWatch(wpars *WatchParams, out *log.Logger) *Watch {
	watch := &Watch{
		mailbox: wpars.Mailbox,
		client:  NewMailClient(wpars.Host, wpars.Port, wpars.Ssl, wpars.Username, wpars.Password),
		logger:  out}

	// watch.AddHandler(NewPostBackHandler(wpars.DeliveryUrl, wpars.UrlEncodeOnPost))
	watch.AddHandler(NewLoggerHandler(out))

	return watch
}

func main() {
	var err error
	var stdLogger *log.Logger
	var wparams *WatchParams
	var watch *Watch

	runtime.GOMAXPROCS(runtime.NumCPU())

	wparams, err = parseAndCheckFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		printUsageAndExit()
	}

	stdLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

	imap.DefaultLogger = stdLogger
	imap.DefaultLogMask = imap.LogConn | imap.LogCmd

	watch = NewWatch(wparams, stdLogger)
	watch.Run()

	stdLogger.Println("Have a nice day.")
}

func parseAndCheckFlags() (*WatchParams, error) {
	watchFlags := NewWatchParams()

	flag.Usage = printUsage

	flag.StringVarP(&watchFlags.Host, "server", "s", "", "IMAP server hostname or ip address")
	flag.UintVarP(&watchFlags.Port, "port", "p", 143, "IMAP server port number (defaults to 143 or 993 for ssl")
	flag.BoolVar(&watchFlags.Ssl, "ssl", false, "Enforce a SSL connection (defaults to true if port is 993)")
	flag.StringVarP(&watchFlags.Username, "user", "U", "", "IMAP login username")
	flag.StringVarP(&watchFlags.Password, "password", "P", "", "IMAP login password")
	flag.StringVarP(&watchFlags.Mailbox, "mailbox", "m", "INBOX", "Mailbox to monitor or idle on. Defaults to: INBOX")
	flag.StringVar(&watchFlags.DeliveryUrl, "delivery_url", "", "URL to post incoming raw email message data")
	flag.BoolVar(&watchFlags.UrlEncodeOnPost, "urlencode", false, "Urlencode RAW message data before posting")

	flag.Parse()

	if flag.NFlag() == 0 {
		return watchFlags, errors.New("! Connection options or config file path are mandatory")
	}

	if watchFlags.Host == "" && watchFlags.DeliveryUrl == "" {
		return watchFlags, errors.New("! IMAP server host and delivery url are mandatory")
	}

	if watchFlags.Port == 143 && watchFlags.Ssl == true {
		watchFlags.Port = 993
	} else if watchFlags.Port == 993 && watchFlags.Ssl == false {
		watchFlags.Ssl = true
	}

	return watchFlags, nil
}

func usageMessage() string {
	var usageStr string

	usageStr = "IMAP idling daemon which delivers incoming email to a webhook.\n\n"

	usageStr += "Usage:\n"
	usageStr += fmt.Sprintf("  %s [OPTIONS]\n", path.Base(os.Args[0]))

	usageStr += "\nOptions are:\n"

	flag.VisitAll(func(f *flag.Flag) {
		if len(f.Shorthand) > 0 {
			usageStr += fmt.Sprintf("  -%s, --%s\r\t\t\t%s\n", f.Shorthand, f.Name, f.Usage)
		} else {
			usageStr += fmt.Sprintf("      --%s\r\t\t\t%s\n", f.Name, f.Usage)
		}
	})

	usageStr += fmt.Sprintf("  -h, --help\r\t\t\tThis help screen\n")
	usageStr += "\n"

	return usageStr
}

func printUsage() {
	fmt.Fprintf(os.Stderr, usageMessage())
}

func printUsageAndExit() {
	printUsage()
	os.Exit(1)
}
