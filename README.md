# Postman

Postman is a simple IMAP idling daemon which will monitor the specified mailbox for incoming email messages and delivery them to a postback endpoint.

It works incredibly well for applications which need to process incoming email messages as they arrive. ie: helpdesk apps.

## Installation

Postman is written in Go. This means it should run under Windows, Linux, FreeBSD and OSX.

### Binary

Installation is very easy. Simply download the appropiate version for your platform from the [releases](https://github.com/etrepat/postman/releases) page. Once downloaded it can be run from anywhere. You don't need to install it into a global location. This works well for shared hosts and other systems where you may not have a privileged account.

If you want to install it globally, I'd recommend somewherewhich already is in your user's path. For example: `/usr/local/bin` may be a good candidate.

*The Postman executable has no external dependencies*

### Building from source

As with any go package building from source is pretty easy. First:

    go get github.com/etrepat/postman

Then build:

    cd /path/to/postman
    go build -o postman main.go

Now you should have a `postman` binary available in the project folder. It's already ready to run!

## Usage

Postman is a rather simple tool. The first thing you should do is run `postman -h` and see the available parameters. You'll immediately see that the options are pretty self explanatory and basically involve 3 areas: connection options (host, port, user, ...), mailbox selection (which IMAP mailbox to monitor) and operation mode.

    postman -h imap.gmail.com --ssl -U <username> -P <password> --postback-url=<receiving host>

### Connection management parameters

#### -h, --host

Specify the hostname or ip address of IMAP server.

#### -p, --port

IMAP server port number. It will default 143 or 993 if ssl is enforced.

#### --ssl

Enforce a SSL connection. Will default to true if port is set to *993*.

#### -U, --user

The IMAP server login username.

#### -P, --password

The IMAP server login password.

### Mailbox selection and mode of operation parameters

#### -b, --mailbox

The IMAP mailbox name to start monitoring on. Will default to *INBOX* if not given.

#### -m, --mode

Sets the daemon mode of operation. Must be one of: `logger`, `postback`.

The `logger` mode is mainly for debugging/testing purposes and it will "spit out" the raw email message data into stdout whenever a new mail arrives at the specified IMAP mailbox.

In `postback` mode, Postman will grab the raw email message data and perform a **POST** request to an endpoint of your choosing. This mode allows for the following additional parameters:

* **--postback-url**: URL to POST incoming raw email message data. By default all data will be sent in the post body with a *text/plain* content-type.
* **--encode**: Will perform the POST request as if it were form data (x-form-urlencoded) wrapping the raw email message in a post parameter.
* **--parname**: Sets the parameter name to be used when `--encode` is set. Defaults to **message**.

## Receiving email data in Rails

This utility was developed as part of a Rails application. You should take into consideration the following points when using the *postback* functionality in a Rails app:

* First, you should disable forgery protection on the receiving controller action.
* If you need to use an authentication token, you may add it to the `postback-url` as a query parameter. You may use ENV vars for that.
* Doing something like `raw_email = request.body.read` in the controller will get you the raw email message data as a string. You can then use the awesome [mail gem](https://github.com/mikel/mail) like this: `mail = Mail.new(raw_email)` to parse the email message and retrieve all the information you need.

## Using with Upstart

To avoid service interruptions, I'd recommend using Postman in combination with some process monitoring tool. For most of my use cases though I personally find that [Upstart](http://http://upstart.ubuntu.com/) is just enough. Here's an sample init script for Postman which may be used as an starting point:

```sh
start on runlevel [2345]
stop on runlevel [016]

respawn
respawn limit 10 90

exec su - <user> -c 'cd /home/<user>/sites/<my-awesome-app>; ./bin/postman -h imap.gmail.com --ssl -U $SMTP_USERNAME -P $SMTP_PASSWD -m postback --postback-url=$POSTMAN_DELIVERY_URL >> /var/log/<my-awesome-app>/postman.log 2>&1'
```

## Contributing

Thinking of contributing? Maybe you've found some nasty bug? That's great news!

1. Fork & clone the project: `git clone git@github.com:your-username/postman.git`.
2. Create your bugfix/feature branch and code away your changes. (git checkout -b my-new-feature).
4. Push to your fork.
5. Submit new a pull request.

## License

Postman is licensed under the terms of the [MIT License](http://opensource.org/licenses/MIT)
(See LICENSE file for details).

---

Coded by [Estanislau Trepat (etrepat)](http://etrepat.com). I'm also
[@etrepat](http://twitter.com/etrepat) on twitter.
