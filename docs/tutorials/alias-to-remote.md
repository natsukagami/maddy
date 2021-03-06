# Forward messages to a remote MX

Default maddy configuration is done in a way that does not result in any
outbound messages being sent as a result of port 25 traffic.

In particular, this means that if you handle messages for example.org but not
example.com and have the following in your aliases file (e.g. /etc/maddy/aliases):

```
foxcpp@example.org: foxcpp@example.com
```

You will get "User does not exist" error when attempting to send a message to
foxcpp@example.org because foxcpp@example.com does not exist on as a local
user.

Some users may want to make it work, but it is important to understand the
consequences of such configuration:

- Flooding your server will also flood the remote server.
- If your spam filtering is not good enough, you will send spam to the remote
  server.

In both cases, you might harm the reputation of your server (e.g. get your IP
listed in a DNSBL).

**So, this is a bad practice. Do so only if you clearly understand the
consequences (including the Bounce handling section below).**

If you want to do it anyway, here is the part of the configuration that needs
tweaking:

```
smtp tcp://0.0.0.0:25 {
    ... global limits ...
    ... local source check ...

    default_source {
        destination $(local_domains) {
            ... local checks ...
            modify &local_modifiers
            deliver_to &local_mailboxes
        }

        default_destination {
            reject 550 5.1.1 "User not local"
        }
    }
}
```

Here is the quick explanation of what is going on here: When maddy receives a
message on port 25, it runs a set of checks, checks the recipients against
'destination' blocks, runs what is specified in local_modifiers
(that includes replace_rcpt file) and finally delivers the message to targets inside
the matching 'destination' blocks.

The problem here is that recipients are matched before aliases are resolved so
in the end, maddy attempts to look up foxcpp@example.com locally. The solution
is to insert another step into the pipeline configuration to rerun matching
*after* aliases are resolved. This can be done using the 'reroute' directive:

```
smtp tcp://0.0.0.0:25 {
    ... global limits ...
    ... local source check ...

    default_source {
        destination $(local_domains) {
            ... local checks ...
            modify &local_modifiers

            reroute {
                destination $(local_domains) {
                    deliver_to &local_mailboxes
                }
                default_destination {
                    deliver_to &remote_queue
                }
            }
        }

        default_destination {
            reject 550 5.1.1 "User not local"
        }
    }
}
```

## Bounce handling

Once the message is delivered to `remote_queue`, it will follow the usual path
for outbound delivery, including queueing and multiple attempts. This also
means bounce messages will be generated on failures. When accepting messages
from arbitrary senders via the 25 port, the DSN recipient will be whatever
sender specifies in the MAIL FROM command. This is prone to [collateral spam]
when an automatically generated bounce message gets sent to a spoofed address.

However, the default maddy configuration ensures that in this case, the NDN
will be delivered only if the original sender is a local user. Backscatter can
not happen if the sender spoofed a local address since such messages will not
be accepted in the first place.

You can also configure maddy to send bounce messages to remote
addresses, but in this case, you should configure a really strict local policy
to make sure the sender address is not spoofed. There is no detailed
explanation of how to do this since this is a terrible idea in general.

[collateral spam]: https://en.wikipedia.org/wiki/Backscatter_(e-mail)

## Transparent forwarding

As an alternative to silently dropping messages on remote delivery failures,
you might want to use transparent forwarding and reject the message without
accepting it first ("connection-stage rejection").

To do so, simply do not use the queue, replace
```
deliver_to &remote_queue
```
with
```
deliver_to remote
```
