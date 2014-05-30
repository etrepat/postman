package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/etrepat/postman/watch"
	flag "github.com/ogier/pflag"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	wFlags, err := parseAndCheckFlags()
	if err != nil {
		fmt.Println(err)
		printUsageAndExit()
	}

	watch := watch.New(wFlags)
	watch.Run()

	fmt.Println("Have a nice day.")
}

func appName() string {
	return path.Base(os.Args[0])
}

func parseAndCheckFlags() (*watch.Flags, error) {
	watchFlags := watch.NewFlags()

	flag.Usage = printUsage

	flag.StringVarP(&watchFlags.Host, "host", "h", "", "IMAP server hostname or ip address")
	flag.UintVarP(&watchFlags.Port, "port", "p", 143, "IMAP server port number (defaults to 143 or 993 for ssl")
	flag.StringVarP(&watchFlags.Username, "user", "U", "", "IMAP login username")
	flag.StringVarP(&watchFlags.Password, "password", "P", "", "IMAP login password")
	flag.StringVarP(&watchFlags.Mailbox, "mailbox", "m", "INBOX", "Mailbox to monitor or idle on. Defaults to: INBOX")
	flag.BoolVar(&watchFlags.Ssl, "ssl", false, "Enforce a SSL connection (defaults to true if port is 993)")
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
	usageStr += fmt.Sprintf("  %s [OPTIONS]\n", appName())

	usageStr += "\nOptions are:\n"

	flag.VisitAll(func(f *flag.Flag) {
		if len(f.Shorthand) > 0 {
			usageStr += fmt.Sprintf("  -%s, --%s\r\t\t\t%s\n", f.Shorthand, f.Name, f.Usage)
		} else {
			usageStr += fmt.Sprintf("      --%s\r\t\t\t%s\n", f.Name, f.Usage)
		}
	})

	usageStr += "\n"
	usageStr += fmt.Sprintf("       --help\r\t\t\tThis help screen\n")
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
