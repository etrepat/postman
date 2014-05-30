package main

import (
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
		printErrorAndExit(err)
	}

	watch := watch.New(wFlags)
	watch.Run()

	fmt.Println("Have a nice day.")
}

func appName() string {
	return path.Base(os.Args[0])
}

func parseAndCheckFlags() (*watch.Flags, error) {
	wflags := watch.NewFlags()

	flag.Usage = printUsage

	flag.StringVarP(&wflags.Host, "host", "h", "", "IMAP server hostname or ip address")
	flag.UintVarP(&wflags.Port, "port", "p", 143, "IMAP server port number (defaults to 143 or 993 for ssl")
	flag.StringVarP(&wflags.Username, "user", "U", "", "IMAP login username")
	flag.StringVarP(&wflags.Password, "password", "P", "", "IMAP login password")
	flag.StringVarP(&wflags.Mailbox, "mailbox", "m", "INBOX", "Mailbox to monitor or idle on. Defaults to: INBOX")
	flag.BoolVar(&wflags.Ssl, "ssl", false, "Enforce a SSL connection (defaults to true if port is 993)")
	flag.StringVar(&wflags.DeliveryUrl, "delivery_url", "", "URL to post incoming raw email message data")
	flag.BoolVar(&wflags.UrlEncodeOnPost, "urlencode", false, "Urlencode RAW message data before posting")

	flag.Parse()

	if flag.NFlag() == 0 {
		return wflags, fmt.Errorf("No options provided.")
	}

	if wflags.Host == "" {
		return wflags, fmt.Errorf("IMAP server host is mandatory.")
	}

	if wflags.DeliveryUrl == "" {
		return wflags, fmt.Errorf("Postback delivery url is mandatory.")
	}

	if wflags.Port == 143 && wflags.Ssl == true {
		wflags.Port = 993
	} else if wflags.Port == 993 && wflags.Ssl == false {
		wflags.Ssl = true
	}

	return wflags, nil
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

func printErrorAndExit(err error) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", appName(), err)
	fmt.Fprintf(os.Stderr, "Try \"%s --help\" for more information.\n", appName())
	os.Exit(1)
}
